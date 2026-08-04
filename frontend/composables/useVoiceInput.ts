export function useVoiceInput(
  onSpeechEnd: (audioBase64: string) => void,
  onSpeechStart?: () => void,
) {
  const isListening = ref(false)
  const isSupported = ref(true)
  const isSpeaking = ref(false)
  const volume = ref(0)

  let audioContext: AudioContext | null = null
  let mediaStream: MediaStream | null = null
  let analyserNode: AnalyserNode | null = null
  let scriptProcessor: ScriptProcessorNode | null = null
  let volumeAnimFrame: number | null = null

  let paused = false

  const noiseSuppression = useNoiseSuppression()

  // Capture at 48kHz for RNNoise, downsample to 16kHz for STT
  const CAPTURE_RATE = 48000
  const STT_RATE = 16000

  let pcmChunks: Float32Array[] = []
  let speechStarted = false
  let silenceFrames = 0
  const SILENCE_THRESHOLD = 0.015
  // At 48kHz with bufferSize=2048: ~42fps, so 30 frames ≈ 700ms
  const SILENCE_FRAMES_REQUIRED = 30
  const MIN_SPEECH_FRAMES = 5

  let speechFrameCount = 0

  async function startListening() {
    try {
      await noiseSuppression.init()

      mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          channelCount: 1,
        },
      })

      audioContext = new AudioContext({ sampleRate: CAPTURE_RATE })
      const source = audioContext.createMediaStreamSource(mediaStream)

      analyserNode = audioContext.createAnalyser()
      analyserNode.fftSize = 256
      source.connect(analyserNode)

      scriptProcessor = audioContext.createScriptProcessor(2048, 1, 1)
      source.connect(scriptProcessor)
      scriptProcessor.connect(audioContext.destination)

      scriptProcessor.onaudioprocess = (e) => {
        if (!isListening.value || paused) return

        const inputData = e.inputBuffer.getChannelData(0)

        // Apply RNNoise denoising
        const denoised = noiseSuppression.processChunk(inputData)
        if (denoised.length === 0) return

        const rms = computeRMS(denoised)

        if (rms > SILENCE_THRESHOLD) {
          if (!speechStarted) {
            speechStarted = true
            isSpeaking.value = true
            pcmChunks = []
            speechFrameCount = 0
            onSpeechStart?.()
          }
          silenceFrames = 0
          speechFrameCount++
        } else if (speechStarted) {
          silenceFrames++
        }

        if (speechStarted) {
          const copy = new Float32Array(denoised.length)
          copy.set(denoised)
          pcmChunks.push(copy)
        }

        if (speechStarted && silenceFrames >= SILENCE_FRAMES_REQUIRED) {
          if (speechFrameCount >= MIN_SPEECH_FRAMES) {
            resampleAndSend(pcmChunks)
          }
          pcmChunks = []
          speechStarted = false
          speechFrameCount = 0
          silenceFrames = 0
          isSpeaking.value = false
        }
      }

      isListening.value = true
      updateVolume()
    } catch (e) {
      console.error('Failed to start voice input:', e)
      isSupported.value = false
    }
  }

  function stopListening() {
    if (speechStarted && speechFrameCount >= MIN_SPEECH_FRAMES && pcmChunks.length > 0) {
      resampleAndSend(pcmChunks)
    }

    pcmChunks = []
    speechStarted = false
    speechFrameCount = 0
    silenceFrames = 0
    isSpeaking.value = false

    if (volumeAnimFrame) {
      cancelAnimationFrame(volumeAnimFrame)
      volumeAnimFrame = null
    }
    if (scriptProcessor) {
      scriptProcessor.disconnect()
      scriptProcessor = null
    }
    if (analyserNode) {
      analyserNode.disconnect()
      analyserNode = null
    }
    if (mediaStream) {
      mediaStream.getTracks().forEach((t) => t.stop())
      mediaStream = null
    }
    if (audioContext) {
      audioContext.close()
      audioContext = null
    }

    noiseSuppression.reset()
    volume.value = 0
    isListening.value = false
  }

  async function resampleAndSend(chunks: Float32Array[]) {
    let totalLen = 0
    for (const c of chunks) totalLen += c.length
    if (totalLen === 0) return

    // Merge chunks into single buffer at 48kHz
    const merged = new Float32Array(totalLen)
    let pos = 0
    for (const c of chunks) {
      merged.set(c, pos)
      pos += c.length
    }

    // Use OfflineAudioContext for high-quality resampling 48kHz → 16kHz
    const outLen = Math.ceil(totalLen * STT_RATE / CAPTURE_RATE)
    const offlineCtx = new OfflineAudioContext(1, outLen, STT_RATE)
    const buf = offlineCtx.createBuffer(1, totalLen, CAPTURE_RATE)
    buf.getChannelData(0).set(merged)
    const src = offlineCtx.createBufferSource()
    src.buffer = buf
    src.connect(offlineCtx.destination)
    src.start()

    const rendered = await offlineCtx.startRendering()
    const resampled = rendered.getChannelData(0)

    const audioBase64 = encodeChunksToBase64([resampled])
    onSpeechEnd(audioBase64)
  }

  function computeRMS(data: Float32Array): number {
    let sum = 0
    for (let i = 0; i < data.length; i++) {
      sum += data[i] * data[i]
    }
    return Math.sqrt(sum / data.length)
  }

  function encodeChunksToBase64(chunks: Float32Array[]): string {
    let totalLength = 0
    for (const c of chunks) totalLength += c.length

    const pcm16 = new Int16Array(totalLength)
    let offset = 0
    for (const c of chunks) {
      for (let i = 0; i < c.length; i++) {
        const s = Math.max(-1, Math.min(1, c[i]))
        pcm16[offset++] = s < 0 ? s * 0x8000 : s * 0x7FFF
      }
    }

    const bytes = new Uint8Array(pcm16.buffer)
    let binary = ''
    for (let i = 0; i < bytes.length; i++) {
      binary += String.fromCharCode(bytes[i])
    }
    return btoa(binary)
  }

  function updateVolume() {
    if (!analyserNode || !isListening.value) return

    const dataArray = new Uint8Array(analyserNode.frequencyBinCount)
    analyserNode.getByteTimeDomainData(dataArray)

    let sum = 0
    for (let i = 0; i < dataArray.length; i++) {
      const normalized = (dataArray[i] - 128) / 128
      sum += normalized * normalized
    }
    const rms = Math.sqrt(sum / dataArray.length)
    volume.value = Math.min(1, rms * 3)

    volumeAnimFrame = requestAnimationFrame(updateVolume)
  }

  onUnmounted(() => {
    stopListening()
    noiseSuppression.destroy()
  })

  function pauseListening() {
    paused = true
    isSpeaking.value = false
    pcmChunks = []
    speechStarted = false
    speechFrameCount = 0
    silenceFrames = 0
  }

  function resumeListening() {
    paused = false
  }

  return {
    isListening: readonly(isListening),
    isSupported: readonly(isSupported),
    isSpeaking: readonly(isSpeaking),
    volume: readonly(volume),
    startListening,
    stopListening,
    pauseListening,
    resumeListening,
  }
}
