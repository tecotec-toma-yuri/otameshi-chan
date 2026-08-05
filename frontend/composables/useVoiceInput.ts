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
  let workletNode: AudioWorkletNode | null = null
  let volumeAnimFrame: number | null = null

  let paused = false

  const noiseSuppression = useNoiseSuppression()

  const CAPTURE_RATE = 48000

  let pcmChunks: Float32Array[] = []
  let speechStarted = false
  let silenceFrames = 0
  const SILENCE_THRESHOLD = 0.015
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

      const workletBlob = new Blob([`
        class VoiceProcessor extends AudioWorkletProcessor {
          constructor() { super(); }
          process(inputs) {
            const input = inputs[0];
            if (input.length > 0 && input[0].length > 0) {
              this.port.postMessage(input[0]);
            }
            return true;
          }
        }
        registerProcessor('voice-processor', VoiceProcessor);
      `], { type: 'application/javascript' })
      const workletUrl = URL.createObjectURL(workletBlob)
      await audioContext.audioWorklet.addModule(workletUrl)
      URL.revokeObjectURL(workletUrl)

      workletNode = new AudioWorkletNode(audioContext, 'voice-processor')
      source.connect(workletNode)
      workletNode.connect(audioContext.destination)

      workletNode.port.onmessage = (e) => {
        if (!isListening.value || paused) return

        const inputData = e.data as Float32Array

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
            encodeOpusAndSend(pcmChunks)
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
      encodeOpusAndSend(pcmChunks)
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
    if (workletNode) {
      workletNode.disconnect()
      workletNode = null
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

  async function encodeOpusAndSend(chunks: Float32Array[]) {
    let totalLen = 0
    for (const c of chunks) totalLen += c.length
    if (totalLen === 0) return

    const merged = new Float32Array(totalLen)
    let pos = 0
    for (const c of chunks) {
      merged.set(c, pos)
      pos += c.length
    }

    const ctx = new OfflineAudioContext(1, totalLen, CAPTURE_RATE)
    const buffer = ctx.createBuffer(1, totalLen, CAPTURE_RATE)
    buffer.getChannelData(0).set(merged)
    const src = ctx.createBufferSource()
    src.buffer = buffer
    const dest = ctx.destination
    src.connect(dest)
    src.start()
    const rendered = await ctx.startRendering()

    const playbackCtx = new AudioContext({ sampleRate: CAPTURE_RATE })
    const streamDest = playbackCtx.createMediaStreamDestination()
    const bufferSrc = playbackCtx.createBufferSource()
    bufferSrc.buffer = rendered
    bufferSrc.connect(streamDest)

    const recorder = new MediaRecorder(streamDest.stream, {
      mimeType: 'audio/webm;codecs=opus',
      audioBitsPerSecond: 32000,
    })

    const recordedChunks: Blob[] = []
    const done = new Promise<Blob>((resolve) => {
      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) recordedChunks.push(e.data)
      }
      recorder.onstop = () => {
        resolve(new Blob(recordedChunks, { type: 'audio/webm;codecs=opus' }))
      }
    })

    recorder.start()
    bufferSrc.start()

    bufferSrc.onended = () => {
      recorder.stop()
      playbackCtx.close()
    }

    const blob = await done
    const arrayBuf = await blob.arrayBuffer()
    const bytes = new Uint8Array(arrayBuf)
    let binary = ''
    for (let i = 0; i < bytes.length; i++) {
      binary += String.fromCharCode(bytes[i])
    }
    const audioBase64 = btoa(binary)
    onSpeechEnd(audioBase64)
  }

  function computeRMS(data: Float32Array): number {
    let sum = 0
    for (let i = 0; i < data.length; i++) {
      sum += data[i] * data[i]
    }
    return Math.sqrt(sum / data.length)
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
