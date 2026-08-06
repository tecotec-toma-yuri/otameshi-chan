package protocol

import "encoding/json"

// OutboundMessage serializes to a flat JSON object: {"type":"...", ...payload_fields}.
type OutboundMessage struct {
	Type    string
	Payload interface{}
}

func (m OutboundMessage) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(m.Payload)
	if err != nil {
		return nil, err
	}
	if len(raw) < 2 || raw[0] != '{' {
		raw = []byte("{}")
	}
	typeField := `"type":"` + m.Type + `"`
	if len(raw) == 2 {
		return []byte("{" + typeField + "}"), nil
	}
	return append([]byte("{"+typeField+","), raw[1:]...), nil
}

// NewOutbound creates a flat outbound message.
func NewOutbound(msgType string, payload interface{}) OutboundMessage {
	return OutboundMessage{Type: msgType, Payload: payload}
}

// InboundMessage is used to parse incoming WS messages that have {type, ...fields}.
type InboundMessage struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (m *InboundMessage) UnmarshalJSON(data []byte) error {
	var t struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	m.Type = t.Type
	m.Raw = json.RawMessage(data)
	return nil
}

// --- Client → Server ---

type AudioChunk struct {
	AudioData  string `json:"audio_data,omitempty"`
	Text       string `json:"text,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
}

type SessionControl struct {
	Action string        `json:"action"`
	Reason string        `json:"reason,omitempty"`
	Config *SessionConfig `json:"config,omitempty"`
}

type SessionConfig struct {
	RecommendationMode         string `json:"recommendation_mode"`
	PostRecommendationBehavior string `json:"post_recommendation_behavior"`
}

// --- Server → Client ---

type TextDelta struct {
	Text string `json:"text"`
}

type LatencyInfo struct {
	STTMs      int64 `json:"stt_ms"`
	LLMTTFBMs  int64 `json:"llm_ttfb_ms"`
	LLMTotalMs int64 `json:"llm_total_ms"`
	TTSMs      int64 `json:"tts_ms"`
	TotalMs    int64 `json:"total_ms"`
}

type TextDone struct {
	Text       string       `json:"text"`
	AudioChunk string       `json:"audio_chunk,omitempty"`
	IsFinal    bool         `json:"is_final"`
	Latency    *LatencyInfo `json:"latency,omitempty"`
}

type ProductRecommendation struct {
	Transcript string        `json:"transcript"`
	AudioChunk string        `json:"audio_chunk,omitempty"`
	Reason     string        `json:"reason,omitempty"`
	Products   []ProductInfo `json:"products"`
}

type ProductInfo struct {
	ProductID   string   `json:"product_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ImageURL    string   `json:"image_url"`
	Tags        []string `json:"tags,omitempty"`
}

type ClearAudioBuffer struct {
	Reason string `json:"reason"`
}

type SessionClose struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type Error struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

type ConnectionState struct {
	State     string `json:"state"`
	SessionID string `json:"session_id"`
}

type ContentViolation struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type UserTranscript struct {
	Text    string `json:"text"`
	Skipped bool   `json:"skipped,omitempty"`
}

const (
	TypeAudioChunk     = "audio_chunk"
	TypeAudioSpeech    = "audio_speech"
	TypeSessionControl = "session_control"

	TypeTextDelta             = "text_delta"
	TypeTextDone              = "text_done"
	TypeProductRecommendation = "product_recommendation"
	TypeClearAudioBuffer      = "clear_audio_buffer"
	TypeSessionClose          = "session_close"
	TypeError                 = "error"
	TypeConnectionState       = "connection_state"
	TypeContentViolation      = "content_violation"
	TypeUserTranscript        = "user_transcript"
	TypeAudioStream           = "audio_stream"
	TypeHistoryRestore        = "history_restore"
)

type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type HistoryRestore struct {
	Messages []HistoryMessage `json:"messages"`
}

type AudioStream struct {
	Audio      string `json:"audio"`
	SentenceID int    `json:"sentence_id"`
	IsFinal    bool   `json:"is_final"`
}
