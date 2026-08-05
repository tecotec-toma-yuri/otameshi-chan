import os
import tempfile
import logging
from faster_whisper import WhisperModel
from flask import Flask, request, jsonify

app = Flask(__name__)

model_size = os.environ.get("WHISPER_MODEL", "small")
device = os.environ.get("WHISPER_DEVICE", "cpu")
compute_type = os.environ.get("WHISPER_COMPUTE_TYPE", "int8")

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

logger.info(f"Loading model: {model_size} (device={device}, compute_type={compute_type})")
model = WhisperModel(model_size, device=device, compute_type=compute_type)
logger.info("Model loaded")


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok"})


@app.route("/inference", methods=["POST"])
def inference():
    if "file" not in request.files:
        return jsonify({"error": "no file provided"}), 400

    audio_file = request.files["file"]
    language = request.form.get("language", None)
    prompt = request.form.get("prompt", None)

    with tempfile.NamedTemporaryFile(suffix=".opus", delete=True) as tmp:
        audio_file.save(tmp.name)

        kwargs = {}
        if language and language != "auto":
            kwargs["language"] = language
        if prompt:
            kwargs["initial_prompt"] = prompt

        segments, info = model.transcribe(
            tmp.name,
            beam_size=5,
            temperature=0.0,
            **kwargs,
        )

        text = " ".join(seg.text.strip() for seg in segments)

    detected_language = info.language if info.language else (language or "")

    logger.info(f"Transcribed: lang={detected_language} text={text[:100]}")

    return jsonify({
        "text": text,
        "language": detected_language,
    })


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8888)
