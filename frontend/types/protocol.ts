// Client → Server messages
export interface AudioChunkMessage {
  type: 'audio_chunk'
  audio_data?: string
  text?: string
  duration_ms?: number
}

export interface AudioSpeechMessage {
  type: 'audio_speech'
  audio: string
}

export interface SessionControlMessage {
  type: 'session_control'
  action: string
  reason?: string
  config?: SessionConfig
}

// Server → Client messages
export interface TextDeltaMessage {
  type: 'text_delta'
  text: string
}

export interface LatencyInfo {
  stt_ms: number
  llm_ttfb_ms: number
  llm_total_ms: number
  tts_ms: number
  total_ms: number
}

export interface TextDoneMessage {
  type: 'text_done'
  text: string
  audio_chunk: string
  is_final: boolean
  latency?: LatencyInfo
}

export interface ProductRecommendationMessage {
  type: 'product_recommendation'
  transcript: string
  audio_chunk: string
  reason?: string
  products: ProductInfo[]
}

export interface ClearAudioBufferMessage {
  type: 'clear_audio_buffer'
  reason: string
}

export interface SessionCloseMessage {
  type: 'session_close'
  reason: string
  message: string
}

export interface ErrorMessage {
  type: 'error'
  code: string
  message: string
  recoverable: boolean
}

export interface ConnectionStateMessage {
  type: 'connection_state'
  state: string
  session_id: string
}

export interface ContentViolationMessage {
  type: 'content_violation'
  reason: string
  message: string
}

export interface UserTranscriptMessage {
  type: 'user_transcript'
  text: string
  skipped?: boolean
}

export interface AudioStreamMessage {
  type: 'audio_stream'
  audio: string
  sentence_id: number
  is_final?: boolean
}

export interface HistoryRestoreMessage {
  type: 'history_restore'
  messages: { role: string; content: string }[]
}

// Shared types
export interface SessionConfig {
  recommendation_mode: 'ai_driven' | 'sequential'
  post_recommendation_behavior: 'return_to_conversation' | 'ask_interest'
}

export interface ProductInfo {
  product_id: string
  name: string
  description: string
  image_url: string
  tags?: string[]
}

// Union type for all server messages
export type ServerMessage =
  | TextDeltaMessage
  | TextDoneMessage
  | ProductRecommendationMessage
  | ClearAudioBufferMessage
  | SessionCloseMessage
  | ErrorMessage
  | ConnectionStateMessage
  | ContentViolationMessage
  | UserTranscriptMessage
  | AudioStreamMessage
  | HistoryRestoreMessage
