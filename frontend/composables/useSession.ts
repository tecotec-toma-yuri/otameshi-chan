import type { ServerMessage, ProductInfo, SessionConfig, LatencyInfo } from '~/types/protocol'

export type SessionState = 'idle' | 'listening' | 'processing' | 'ai_speaking' | 'barge_in' | 'error' | 'completed'

export interface ConversationMessage {
  role: 'user' | 'ai' | 'system'
  text: string
  timestamp: Date
  products?: ProductInfo[]
  streaming?: boolean
  loading?: boolean
  latency?: LatencyInfo
  restored?: boolean
}

export function useSession() {
  const state = ref<SessionState>('idle')
  const messages = ref<ConversationMessage[]>([])
  const currentProducts = ref<ProductInfo[]>([])
  const errorMessage = ref('')
  const errorType = ref<'error' | 'warning'>('error')
  const pendingListeningTransition = ref(false)

  // ストリーミング中のAIメッセージ。マイク割り込みではユーザー発話が先に
  // 配列へ積まれるため「配列の最後」では特定できず、参照で追跡する。
  let streamingMsg: ConversationMessage | null = null

  function pushAIMessage(init: Omit<ConversationMessage, 'role' | 'timestamp'>): ConversationMessage {
    messages.value.push({ role: 'ai', timestamp: new Date(), ...init })
    return messages.value[messages.value.length - 1]!
  }

  const websocket = useWebSocket()
  const audioPlayback = useAudioPlayback()
  const audioCapture = useAudioCapture(websocket.sendAudioChunk)

  watch(audioPlayback.isPlaying, (playing) => {
    if (!playing && pendingListeningTransition.value) {
      pendingListeningTransition.value = false
      if (state.value === 'ai_speaking') {
        state.value = 'listening'
      }
    }
  })

  watch(state, (newState) => {
    if (newState === 'ai_speaking' || newState === 'listening') {
      voiceInput.resumeListening()
    }
  })

  function clearLoadingUserMessages() {
    messages.value = messages.value.filter((m) => !(m.role === 'user' && m.loading))
  }

  function handleSpeechStart() {
    if (state.value === 'ai_speaking' || audioPlayback.isPlaying.value) {
      audioPlayback.stopAndClear()
      state.value = 'barge_in'
    }
  }

  function handleSpeechEnd(audioBase64: string) {
    voiceInput.pauseListening()
    messages.value.push({
      role: 'user',
      text: '',
      loading: true,
      timestamp: new Date(),
    })
    state.value = 'processing'
    websocket.sendAudioSpeech(audioBase64)
  }

  const voiceInput = useVoiceInput(handleSpeechEnd, handleSpeechStart)

  function finalizeSessionLocally() {
    audioPlayback.stopAndClear()
    audioCapture.stopCapture()
    voiceInput.stopListening()
    websocket.disconnect()
  }

  websocket.onMessage((msg: ServerMessage) => {
    switch (msg.type) {
      case 'text_delta':
        state.value = 'ai_speaking'
        if (streamingMsg) {
          streamingMsg.text += msg.text
        } else {
          streamingMsg = pushAIMessage({ text: msg.text, streaming: true })
        }
        break

      case 'audio_stream':
        state.value = 'ai_speaking'
        if (msg.audio) {
          audioPlayback.playAudio(msg.audio)
        }
        break

      case 'text_done':
        if (streamingMsg) {
          streamingMsg.text = msg.text
          streamingMsg.streaming = false
          if (msg.latency) {
            streamingMsg.latency = msg.latency
          }
          streamingMsg = null
        } else {
          // 無音確認など、text_delta を伴わない単独の text_done
          pushAIMessage({ text: msg.text, latency: msg.latency })
        }

        if (msg.audio_chunk) {
          audioPlayback.playAudio(msg.audio_chunk)
        }

        if (msg.is_final) {
          pendingListeningTransition.value = true
        }
        break

      case 'product_recommendation':
        currentProducts.value = msg.products
        {
          const text = msg.transcript || '商品をおすすめします。'
          if (streamingMsg) {
            streamingMsg.text = text
            streamingMsg.streaming = false
            streamingMsg.products = msg.products
          } else {
            pushAIMessage({ text, products: msg.products })
          }
          streamingMsg = null
        }

        if (msg.audio_chunk) {
          audioPlayback.playAudio(msg.audio_chunk)
        }
        state.value = 'ai_speaking'
        break

      case 'history_restore':
        if (msg.messages && Array.isArray(msg.messages)) {
          for (const m of msg.messages) {
            messages.value.push({
              role: m.role === 'user' ? 'user' : 'ai',
              text: m.content,
              timestamp: new Date(),
              restored: true,
            })
          }
        }
        break

      case 'clear_audio_buffer':
        audioPlayback.stopAndClear()
        if (streamingMsg) {
          // 中断された世代は text_done が届かないため、ここで表示を確定させる
          streamingMsg.streaming = false
          streamingMsg = null
        }
        if (state.value !== 'completed') {
          state.value = 'listening'
        }
        break

      case 'session_close':
        state.value = 'completed'
        if (streamingMsg) {
          streamingMsg.streaming = false
          streamingMsg = null
        }
        clearLoadingUserMessages()
        messages.value.push({
          role: 'system',
          text: msg.message || '対話が終了しました。',
          timestamp: new Date(),
        })
        audioPlayback.waitUntilDone().then(() => {
          finalizeSessionLocally()
        })
        break

      case 'error':
        errorMessage.value = msg.message
        errorType.value = 'error'
        if (msg.code === 'stt_error') {
          clearLoadingUserMessages()
        }
        if (!msg.recoverable) {
          state.value = 'error'
        }
        break

      case 'content_violation':
        errorMessage.value = msg.message
        errorType.value = 'warning'
        break

      case 'user_transcript':
        if (msg.skipped) {
          clearLoadingUserMessages()
          state.value = 'listening'
          break
        }
        {
          const loadingMsg = [...messages.value].reverse().find((m) => m.role === 'user' && m.loading)
          if (loadingMsg) {
            loadingMsg.text = msg.text
            loadingMsg.loading = false
          } else {
            messages.value.push({
              role: 'user',
              text: msg.text,
              timestamp: new Date(),
            })
          }
        }
        state.value = 'processing'
        break

      case 'connection_state':
        if (msg.state === 'connected' || msg.state === 'ready') {
          state.value = 'listening'
        }
        break
    }
  })

  function connect(config?: SessionConfig, restoreId?: string) {
    messages.value = []
    currentProducts.value = []
    streamingMsg = null
    errorMessage.value = ''
    state.value = 'idle'
    websocket.connect(config, restoreId)
  }

  function disconnect() {
    websocket.sendClose('user_disconnect')
    finalizeSessionLocally()
    pendingListeningTransition.value = false
    messages.value.push({
      role: 'system',
      text: 'セッションが終了しました。',
      timestamp: new Date(),
    })
    state.value = 'idle'
  }

  function sendText(text: string) {
    if (!text.trim()) return

    if (state.value === 'ai_speaking' || state.value === 'processing' || audioPlayback.isPlaying.value) {
      audioPlayback.stopAndClear()
      state.value = 'barge_in'
    }

    messages.value.push({
      role: 'user',
      text,
      timestamp: new Date(),
    })

    websocket.sendText(text)
    state.value = 'processing'
  }

  function clearError() {
    errorMessage.value = ''
  }

  return {
    state: readonly(state),
    messages: readonly(messages),
    currentProducts: readonly(currentProducts),
    errorMessage: readonly(errorMessage),
    errorType: readonly(errorType),
    connectionState: websocket.connectionState,
    sessionId: websocket.sessionId,
    isPlaying: audioPlayback.isPlaying,
    playbackVolume: audioPlayback.volume,
    isMuted: audioPlayback.isMuted,
    micVolume: voiceInput.volume,
    isCapturing: voiceInput.isListening,
    isSpeaking: voiceInput.isSpeaking,
    voiceSupported: voiceInput.isSupported,
    connect,
    disconnect,
    sendText,
    clearError,
    startCapture: voiceInput.startListening,
    stopCapture: voiceInput.stopListening,
    resumeAudioContext: audioPlayback.resumeContext,
    setPlaybackVolume: audioPlayback.setVolume,
    toggleMute: audioPlayback.toggleMute,
  }
}
