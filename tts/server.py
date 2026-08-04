import io
import json
import os
import wave
import logging
import logging.handlers
from pathlib import Path
from pythonjsonlogger import jsonlogger
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse, Response
import uvicorn
from piper import PiperVoice


def setup_logging():
    log_path = os.getenv("LOG_PATH", "logs/tts.log")
    log_level = logging.DEBUG if os.getenv("LOG_LEVEL") == "debug" else logging.INFO

    root = logging.getLogger()
    root.setLevel(log_level)

    # stdout: JSON format
    stdout_handler = logging.StreamHandler()
    stdout_handler.setFormatter(jsonlogger.JsonFormatter(
        "%(asctime)s %(levelname)s %(name)s %(message)s",
        rename_fields={"asctime": "time", "levelname": "level", "name": "logger"},
    ))
    root.addHandler(stdout_handler)

    # file: text format
    Path(log_path).parent.mkdir(parents=True, exist_ok=True)
    file_handler = logging.FileHandler(log_path, encoding="utf-8")
    file_handler.setFormatter(logging.Formatter(
        "%(asctime)s %(levelname)-5s %(name)s: %(message)s"
    ))
    root.addHandler(file_handler)

    # suppress uvicorn duplicating its own handlers
    for name in ("uvicorn", "uvicorn.access", "uvicorn.error"):
        uv_logger = logging.getLogger(name)
        uv_logger.handlers = []
        uv_logger.propagate = True


setup_logging()
logger = logging.getLogger(__name__)

app = FastAPI()

DEFAULT_LENGTH_SCALE = float(os.getenv("TTS_LENGTH_SCALE", "1.3"))
DEFAULT_NOISE_SCALE = float(os.getenv("TTS_NOISE_SCALE", "0.667"))
DEFAULT_NOISE_W = float(os.getenv("TTS_NOISE_W", "0.8"))
DEFAULT_VOICE = os.getenv("TTS_VOICE", "tsukuyomi_mb")
DEFAULT_SPEAKER_ID = int(os.getenv("TTS_SPEAKER_ID", "0"))

# 6言語ベース学習データの話者レンジ（ドキュメント準拠）
BASE_6LANG_SPEAKER_RANGES = [
    ("ja", "日本語", 0, 19),
    ("en", "英語", 20, 329),
    ("zh", "中国語", 330, 471),
    ("es", "スペイン語", 472, 534),
    ("fr", "フランス語", 535, 562),
    ("pt", "ポルトガル語", 563, 570),
]

LANG_LABELS = {
    "ja": "日本語",
    "en": "英語",
    "zh": "中国語",
    "es": "スペイン語",
    "fr": "フランス語",
    "pt": "ポルトガル語",
}

VOICE_DEFS = {
    "tsukuyomi": {
        "id": "tsukuyomi",
        "label": "つくよみちゃん（従来版）",
        "dir": "/app/models/tsukuyomi",
    },
    "tsukuyomi_mb": {
        "id": "tsukuyomi_mb",
        "label": "つくよみちゃん MB版（6言語）",
        "dir": "/app/models/tsukuyomi_mb",
    },
    "css10": {
        "id": "css10",
        "label": "CSS10 Japanese（6言語）",
        "dir": "/app/models/css10",
    },
    "base_6lang": {
        "id": "base_6lang",
        "label": "6言語ベース（多話者）",
        "dir": "/app/models/base_6lang",
    },
}


def _onnx_has_speaker_input(model_path: str) -> bool:
    try:
        import onnxruntime as ort

        sess = ort.InferenceSession(model_path, providers=["CPUExecutionProvider"])
        names = {i.name for i in sess.get_inputs()}
        return "sid" in names or "speaker_id" in names
    except Exception as e:
        logger.warning("failed to inspect onnx inputs: %s (%s)", model_path, e)
        return False


def _speaker_label(speaker_id: int, num_speakers: int) -> str:
    if num_speakers >= 571:
        for lang, label, start, end in BASE_6LANG_SPEAKER_RANGES:
            if start <= speaker_id <= end:
                local = speaker_id - start + 1
                total = end - start + 1
                return f"{label} 話者{local}/{total} (id={speaker_id})"
    return f"話者 {speaker_id}"


def _build_speakers(num_speakers: int, speaker_id_map: dict | None, voice_id: str = "") -> list[dict]:
    speakers: list[dict] = []
    if speaker_id_map:
        for name, sid in sorted(speaker_id_map.items(), key=lambda x: int(x[1])):
            speakers.append({"id": int(sid), "label": str(name)})
        if speakers:
            return speakers

    if num_speakers <= 1:
        return [{"id": 0, "label": "デフォルト"}]

    # speakers_base_6lang.json があれば利用
    catalog_path = "/app/speakers_base_6lang.json"
    if voice_id == "base_6lang" and os.path.exists(catalog_path):
        try:
            with open(catalog_path, encoding="utf-8") as f:
                catalog = json.load(f)
            for rng in catalog.get("ranges", []):
                lang = rng.get("language", "")
                label = rng.get("label", lang)
                start = int(rng["start"])
                end = int(rng["end"])
                for sid in range(start, end + 1):
                    if lang == "ja" or sid == start or sid < start + 5:
                        local = sid - start + 1
                        total = end - start + 1
                        speakers.append({
                            "id": sid,
                            "label": f"{label} 話者{local}/{total} (id={sid})",
                        })
            if speakers:
                return speakers
        except Exception as e:
            logger.warning("failed to load speaker catalog: %s", e)

    if num_speakers >= 571:
        for _lang, label, start, end in BASE_6LANG_SPEAKER_RANGES:
            for sid in range(start, end + 1):
                if _lang == "ja" or sid == start:
                    speakers.append({"id": sid, "label": _speaker_label(sid, num_speakers)})
            if _lang != "ja":
                for sid in range(start + 1, min(start + 5, end + 1)):
                    speakers.append({"id": sid, "label": _speaker_label(sid, num_speakers)})
        return speakers

    for sid in range(num_speakers):
        speakers.append({"id": sid, "label": _speaker_label(sid, num_speakers)})
    return speakers


def _build_languages(language_id_map: dict | None) -> list[dict]:
    if not language_id_map:
        return []
    out = []
    for code, lid in sorted(language_id_map.items(), key=lambda x: int(x[1])):
        out.append({
            "id": code,
            "language_id": int(lid),
            "label": LANG_LABELS.get(code, code),
        })
    return out


def _voice_meta(voice_id: str, voice: PiperVoice) -> dict:
    cfg = voice.config
    num_speakers = int(getattr(cfg, "num_speakers", 1) or 1)
    speaker_id_map = getattr(cfg, "speaker_id_map", None) or {}
    language_id_map = getattr(cfg, "language_id_map", None) or {}
    num_languages = int(getattr(cfg, "num_languages", 0) or len(language_id_map) or 0)

    # config.json を直接読んで speaker_id_map を補完（属性に無い場合）
    config_path = os.path.join(VOICE_DEFS[voice_id]["dir"], "config.json")
    if os.path.exists(config_path):
        try:
            with open(config_path, encoding="utf-8") as f:
                raw = json.load(f)
            if not speaker_id_map:
                speaker_id_map = raw.get("speaker_id_map") or {}
            if not language_id_map:
                language_id_map = raw.get("language_id_map") or {}
            num_speakers = int(raw.get("num_speakers", num_speakers) or num_speakers)
            num_languages = int(raw.get("num_languages", num_languages) or num_languages)
        except Exception as e:
            logger.warning("failed to read config for %s: %s", voice_id, e)

    # ONNX に sid 入力があるかで多話者を判定（公開 base は emb_g 除去済みの場合あり）
    has_sid = _onnx_has_speaker_input(os.path.join(VOICE_DEFS[voice_id]["dir"], "model.onnx"))
    if voice_id == "base_6lang" and num_speakers <= 1 and has_sid:
        num_speakers = 571
    elif not has_sid:
        num_speakers = 1

    speakers = _build_speakers(num_speakers, speaker_id_map, voice_id)
    languages = _build_languages(language_id_map)

    return {
        "id": voice_id,
        "label": VOICE_DEFS[voice_id]["label"],
        "num_speakers": num_speakers,
        "num_languages": num_languages,
        "speakers": speakers,
        "languages": languages,
        "multi_speaker": num_speakers > 1,
    }


voices: dict[str, PiperVoice] = {}
voice_metas: dict[str, dict] = {}
# base_6lang 未配置時の暫定: 既存 6 言語単一話者モデルを話者として切替
voice_banks: dict[str, dict[int, str]] = {}

for voice_id, meta in VOICE_DEFS.items():
    model_path = os.path.join(meta["dir"], "model.onnx")
    config_path = os.path.join(meta["dir"], "config.json")
    if not os.path.exists(model_path) or not os.path.exists(config_path):
        logger.warning("voice model missing, skip: %s", voice_id)
        continue
    logger.info("Loading voice: %s (%s)", voice_id, model_path)
    voice = PiperVoice.load(model_path, config_path=config_path)
    voices[voice_id] = voice
    voice_metas[voice_id] = _voice_meta(voice_id, voice)
    logger.info(
        "Loaded voice: %s speakers=%s languages=%s",
        voice_id,
        voice_metas[voice_id]["num_speakers"],
        voice_metas[voice_id]["num_languages"],
    )

# 公式多話者 ONNX が無い場合、6言語ファインチューン済みモデルでボイスバンクを構成
if "base_6lang" not in voices:
    bank: dict[int, str] = {}
    bank_speakers: list[dict] = []
    bank_languages: list[dict] = []
    sid = 0
    for src_id, label in (
        ("tsukuyomi_mb", "つくよみちゃん MB版"),
        ("css10", "CSS10 Japanese"),
    ):
        if src_id not in voices:
            continue
        bank[sid] = src_id
        bank_speakers.append({"id": sid, "label": f"{label} (bank #{sid})"})
        if not bank_languages:
            bank_languages = list(voice_metas[src_id].get("languages") or [])
        sid += 1
    if bank:
        voice_banks["base_6lang"] = bank
        voice_metas["base_6lang"] = {
            "id": "base_6lang",
            "label": "6言語ベース（多話者・ボイスバンク暫定）",
            "num_speakers": len(bank),
            "num_languages": len(bank_languages),
            "speakers": bank_speakers,
            "languages": bank_languages,
            "multi_speaker": True,
            "note": "公式多話者 ONNX 未公開のため、既存 6 言語モデルを話者として切替えています。tts/models/base_6lang に ONNX を置くと本家に切り替わります。",
        }
        logger.info("Registered virtual base_6lang bank speakers=%s", list(bank.keys()))

if not voices and not voice_banks:
    raise RuntimeError("no TTS voices loaded")

default_voice = DEFAULT_VOICE if DEFAULT_VOICE in voices or DEFAULT_VOICE in voice_banks else next(iter(voices))
logger.info(
    "Default voice: %s (available=%s banks=%s)",
    default_voice,
    list(voices.keys()),
    list(voice_banks.keys()),
)


def _resolve_language_id(voice_id: str, language: str | None, language_id: int | None) -> int | None:
    meta = voice_metas.get(voice_id) or {}
    languages = meta.get("languages") or []
    if not languages:
        return None

    if language_id is not None:
        return int(language_id)

    if language:
        for lang in languages:
            if lang["id"] == language:
                return int(lang["language_id"])

    for lang in languages:
        if lang["id"] == "ja":
            return int(lang["language_id"])
    return int(languages[0]["language_id"])


def _resolve_speaker_id(voice_id: str, speaker_id: int | None) -> int:
    meta = voice_metas.get(voice_id) or {}
    num_speakers = int(meta.get("num_speakers") or 1)
    if num_speakers <= 1:
        return 0
    if speaker_id is None:
        speaker_id = DEFAULT_SPEAKER_ID
    sid = int(speaker_id)
    if sid < 0 or sid >= num_speakers:
        logger.warning("speaker_id %s out of range for %s, fallback to 0", sid, voice_id)
        return 0
    return sid


def _get_synth_voice(voice_id: str, speaker_id: int) -> tuple[PiperVoice, str]:
    """Returns (piper_voice, resolved_voice_id_for_logging)."""
    if voice_id in voice_banks:
        bank = voice_banks[voice_id]
        src = bank.get(speaker_id) or bank[0]
        return voices[src], src
    return voices[voice_id], voice_id


@app.get("/health")
def health():
    return {
        "status": "ok",
        "voices": list(voices.keys()) + list(voice_banks.keys()),
        "default_voice": default_voice,
    }


@app.get("/voices")
def list_voices():
    ordered = []
    for voice_id in VOICE_DEFS.keys():
        if voice_id in voice_metas:
            ordered.append(voice_metas[voice_id])
    return {
        "default_voice": default_voice,
        "default_speaker_id": DEFAULT_SPEAKER_ID,
        "voices": ordered,
    }


@app.post("/synthesize")
async def synthesize(request: Request):
    data = await request.json()
    if not data or "text" not in data:
        return JSONResponse({"error": "missing 'text' field"}, status_code=400)

    text = data["text"]
    if not text.strip():
        return JSONResponse({"error": "empty text"}, status_code=400)

    voice_id = data.get("voice") or default_voice
    if voice_id not in voices and voice_id not in voice_banks:
        return JSONResponse(
            {
                "error": f"unknown voice '{voice_id}'",
                "available": list(voices.keys()) + list(voice_banks.keys()),
            },
            status_code=400,
        )

    length_scale = data.get("length_scale", DEFAULT_LENGTH_SCALE)
    noise_scale = data.get("noise_scale", DEFAULT_NOISE_SCALE)
    noise_w = data.get("noise_w", DEFAULT_NOISE_W)
    speaker_id = _resolve_speaker_id(voice_id, data.get("speaker_id"))
    language_id = _resolve_language_id(
        voice_id,
        data.get("language"),
        data.get("language_id"),
    )

    piper_voice, resolved_id = _get_synth_voice(voice_id, speaker_id)

    logger.info(
        "Synthesizing voice=%s resolved=%s speaker=%s language_id=%s speed=%.2f: %s",
        voice_id,
        resolved_id,
        speaker_id,
        language_id,
        length_scale,
        text[:50],
    )

    synth_kwargs = {
        "length_scale": length_scale,
        "noise_scale": noise_scale,
        "noise_w": noise_w,
    }
    # ボイスバンク経由の場合は単一話者モデルなので speaker_id は渡さない
    if voice_id not in voice_banks:
        synth_kwargs["speaker_id"] = speaker_id
    if language_id is not None:
        synth_kwargs["language_id"] = language_id

    buf = io.BytesIO()
    with wave.open(buf, "wb") as wav_file:
        piper_voice.synthesize(text, wav_file, **synth_kwargs)

    buf.seek(0)
    return Response(content=buf.read(), media_type="audio/wav")


if __name__ == "__main__":
    uvicorn.run(
        app,
        host="0.0.0.0",
        port=5000,
        log_config=None,
    )
