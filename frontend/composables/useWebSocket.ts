import type { ServerMessage, SessionConfig, AudioChunkMessage, AudioSpeechMessage, SessionControlMessage } from '~/types/protocol'

type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'error'

export function useWebSocket() {
  const config = useRuntimeConfig()
  const connectionState = ref<ConnectionState>('disconnected')
  const sessionId = ref('')

  let ws: WebSocket | null = null
  let messageHandler: ((msg: ServerMessage) => void) | null = null
  let retryCount = 0
  const maxRetries = 3
  const baseDelay = 1000
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let sessionConfig: SessionConfig | undefined

  function onMessage(handler: (msg: ServerMessage) => void) {
    messageHandler = handler
  }

  let restoreSessionId: string | undefined

  function connect(cfg?: SessionConfig, restoreId?: string) {
    sessionConfig = cfg
    restoreSessionId = restoreId
    retryCount = 0
    _connect()
  }

  function _connect() {
    if (ws) {
      ws.onclose = null
      ws.close()
    }

    connectionState.value = 'connecting'
    let url = config.public.wsUrl as string
    if (restoreSessionId) {
      url += (url.includes('?') ? '&' : '?') + `restore_session_id=${encodeURIComponent(restoreSessionId)}`
    }
    ws = new WebSocket(url)

    ws.onopen = () => {
      connectionState.value = 'connected'
      retryCount = 0

      if (sessionConfig) {
        const msg: SessionControlMessage = {
          type: 'session_control',
          action: 'start',
          config: sessionConfig,
        }
        ws!.send(JSON.stringify(msg))
      }
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as ServerMessage
        if (msg.type === 'connection_state') {
          sessionId.value = msg.session_id
        }
        messageHandler?.(msg)
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e)
      }
    }

    ws.onerror = () => {
      connectionState.value = 'error'
    }

    ws.onclose = () => {
      connectionState.value = 'disconnected'
      ws = null

      if (retryCount < maxRetries) {
        const delay = baseDelay * Math.pow(2, retryCount)
        retryCount++
        reconnectTimer = setTimeout(() => _connect(), delay)
      }
    }
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    retryCount = maxRetries // prevent auto-reconnect
    if (ws) {
      ws.onclose = null
      ws.close()
      ws = null
    }
    connectionState.value = 'disconnected'
    sessionId.value = ''
  }

  function sendText(text: string) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    const msg: AudioChunkMessage = {
      type: 'audio_chunk',
      text,
    }
    ws.send(JSON.stringify(msg))
  }

  function sendAudioChunk(base64Audio: string, durationMs: number) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    const msg: AudioChunkMessage = {
      type: 'audio_chunk',
      audio_data: base64Audio,
      duration_ms: durationMs,
    }
    ws.send(JSON.stringify(msg))
  }

  function sendAudioSpeech(base64Audio: string) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    const msg: AudioSpeechMessage = {
      type: 'audio_speech',
      audio: base64Audio,
    }
    ws.send(JSON.stringify(msg))
  }

  function sendClose(reason: string) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    const msg: SessionControlMessage = {
      type: 'session_control',
      action: 'close',
      reason,
    }
    ws.send(JSON.stringify(msg))
  }

  onUnmounted(() => {
    disconnect()
  })

  return {
    connectionState: readonly(connectionState),
    sessionId: readonly(sessionId),
    connect,
    disconnect,
    sendText,
    sendAudioChunk,
    sendAudioSpeech,
    sendClose,
    onMessage,
  }
}
