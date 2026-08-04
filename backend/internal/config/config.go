package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"gopkg.in/yaml.v3"
)

type Product struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Price       int    `json:"price" yaml:"price"`
	Description string `json:"description" yaml:"description"`
	ImageURL    string `json:"image_url" yaml:"image_url"`
}

type Config struct {
	SystemPrompt      string  `json:"system_prompt" yaml:"system_prompt"`
	LLMModel          string  `json:"llm_model" yaml:"llm_model"`
	TTSEngine         string  `json:"tts_engine" yaml:"tts_engine"`
	TTSVoice          string  `json:"tts_voice" yaml:"tts_voice"`
	TTSSpeakerID      int     `json:"tts_speaker_id" yaml:"tts_speaker_id"`
	TTSLanguage       string  `json:"tts_language" yaml:"tts_language"`
	TTSLengthScale    float64 `json:"tts_length_scale" yaml:"tts_length_scale"`
	TTSNoiseScale     float64 `json:"tts_noise_scale" yaml:"tts_noise_scale"`
	TTSNoiseW         float64 `json:"tts_noise_w" yaml:"tts_noise_w"`
	VoicevoxSpeakerID int     `json:"voicevox_speaker_id" yaml:"voicevox_speaker_id"`
	VoicevoxSpeedScale float64 `json:"voicevox_speed_scale" yaml:"voicevox_speed_scale"`
	STTLanguage       string    `json:"stt_language" yaml:"stt_language"`
	STTPrompt         string    `json:"stt_prompt" yaml:"stt_prompt"`
	Products          []Product `json:"products" yaml:"products"`
}

var (
	mu       sync.RWMutex
	current  Config
	filePath string
)

func defaultConfig() Config {
	return Config{
		SystemPrompt: `あなたは「おためしちゃん」という名前の、お菓子についておしゃべりするのが大好きなアシスタントです。

## 性格・話し方
- 明るくフレンドリーで、お客様との会話を楽しむタイプ
- 短めの返答（1〜2文）でテンポよく会話する
- 日本語で応答する

## 会話の進め方
- まずはお客様の好みや気分を聞いてみる（「どんな味が好きですか？」「今日はどんな気分ですか？」など）
- いきなり商品を紹介せず、2〜3回のやり取りで好みを把握してから提案する
- お客様が「おすすめして」「商品を見せて」など明確に求めた場合はすぐに紹介してOK`,
		LLMModel:       envOrDefault("OPENAI_MODEL", "gpt-4o-mini"),
		TTSEngine:      envOrDefault("TTS_ENGINE", "piper"),
		TTSVoice:       envOrDefault("TTS_VOICE", "tsukuyomi_mb"),
		TTSSpeakerID:   envIntOrDefault("TTS_SPEAKER_ID", 0),
		TTSLanguage:    envOrDefault("TTS_LANGUAGE", "ja"),
		TTSLengthScale: envFloatOrDefault("TTS_LENGTH_SCALE", 1.3),
		TTSNoiseScale:  envFloatOrDefault("TTS_NOISE_SCALE", 0.667),
		TTSNoiseW:         envFloatOrDefault("TTS_NOISE_W", 0.8),
		VoicevoxSpeakerID: envIntOrDefault("VOICEVOX_SPEAKER_ID", 3),
		VoicevoxSpeedScale: envFloatOrDefault("VOICEVOX_SPEED_SCALE", 1.0),
		STTLanguage:       envOrDefault("STT_LANGUAGE", "ja"),
		STTPrompt:         envOrDefault("STT_PROMPT", ""),
		Products: []Product{
			{ID: "h001", Name: "ハイチュウ ＜グレープ＞", Price: 140, Description: "ジューシーな果汁感あふれる定番フレーバー", ImageURL: "/images/h001.png"},
			{ID: "h002", Name: "ハイチュウ ＜ストロベリー＞", Price: 140, Description: "いちごの華やかな香りと甘酸っぱさ", ImageURL: "/images/h002.png"},
			{ID: "h003", Name: "ハイチュウ ＜グリーンアップル＞", Price: 140, Description: "爽やかな香りとスッキリとした酸味", ImageURL: "/images/h003.png"},
		},
	}
}

func Load(path string) {
	filePath = path
	current = defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Error("failed to read config file", "error", err)
		}
		slog.Info("using default config (no config file found)", "path", path)
		return
	}

	var saved Config
	if err := yaml.Unmarshal(data, &saved); err != nil {
		slog.Error("failed to parse config file", "error", err)
		return
	}

	merge(&saved)
	slog.Info("config loaded from file", "path", path)
}

func merge(saved *Config) {
	current = *saved
}

func Get() Config {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

func Update(c Config) error {
	mu.Lock()
	defer mu.Unlock()
	current = c
	return save()
}

func save() error {
	data, err := yaml.Marshal(&current)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		slog.Error("failed to create config directory", "error", err)
		return err
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		slog.Error("failed to write config file", "error", err)
		return err
	}
	slog.Info("config saved", "path", filePath)
	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envFloatOrDefault(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func envIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}
