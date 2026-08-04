export function useAudioCapture(sendChunk: (base64: string, durationMs: number) => void) {
  const volume = ref(0)
  const isCapturing = ref(false)

  let audioContext: AudioContext | null = null
  let mediaStream: MediaStream | null = null
  let analyserNode: AnalyserNode | null = null
  let processorNode: ScriptProcessorNode | null = null
  let volumeAnimFrame: number | null = null
  let accumulatedSamples: Float32Array[] = []
  let accumulatedLength = 0
  const chunkIntervalMs = 250

  async function startCapture() {
    try {
      mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          sampleRate: 16000,
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
        },
      })

      audioContext = new AudioContext({ sampleRate: 16000 })
      const source = audioContext.createMediaStreamSource(mediaStream)

      // Analyser for volume metering
      analyserNode = audioContext.createAnalyser()
      analyserNode.fftSize = 256
      source.connect(analyserNode)

      // ScriptProcessor for capturing PCM data
      const bufferSize = 4096
      processorNode = audioContext.createScriptProcessor(bufferSize, 1, 1)
      source.connect(processorNode)
      processorNode.connect(audioContext.destination)

      let lastSendTime = Date.now()

      processorNode.onaudioprocess = (e) => {
        const inputData = e.inputBuffer.getChannelData(0)
        const copy = new Float32Array(inputData.length)
        copy.set(inputData)
        accumulatedSamples.push(copy)
        accumulatedLength += copy.length

        const now = Date.now()
        if (now - lastSendTime >= chunkIntervalMs) {
          flushChunk(now - lastSendTime)
          lastSendTime = now
        }
      }

      isCapturing.value = true
      updateVolume()
    } catch (e) {
      console.error('Failed to start audio capture:', e)
    }
  }

  function flushChunk(durationMs: number) {
    if (accumulatedSamples.length === 0) return

    // Merge accumulated samples
    const merged = new Float32Array(accumulatedLength)
    let offset = 0
    for (const chunk of accumulatedSamples) {
      merged.set(chunk, offset)
      offset += chunk.length
    }
    accumulatedSamples = []
    accumulatedLength = 0

    // Convert Float32 PCM to Int16 PCM
    const int16 = new Int16Array(merged.length)
    for (let i = 0; i < merged.length; i++) {
      const s = Math.max(-1, Math.min(1, merged[i]))
      int16[i] = s < 0 ? s * 0x8000 : s * 0x7FFF
    }

    // Encode as base64
    const bytes = new Uint8Array(int16.buffer)
    let binary = ''
    for (let i = 0; i < bytes.length; i++) {
      binary += String.fromCharCode(bytes[i])
    }
    const base64 = btoa(binary)
    sendChunk(base64, durationMs)
  }

  function updateVolume() {
    if (!analyserNode || !isCapturing.value) return

    const dataArray = new Uint8Array(analyserNode.frequencyBinCount)
    analyserNode.getByteTimeDomainData(dataArray)

    // Calculate RMS
    let sum = 0
    for (let i = 0; i < dataArray.length; i++) {
      const normalized = (dataArray[i] - 128) / 128
      sum += normalized * normalized
    }
    const rms = Math.sqrt(sum / dataArray.length)
    volume.value = Math.min(1, rms * 3) // Scale up for visibility

    volumeAnimFrame = requestAnimationFrame(updateVolume)
  }

  function stopCapture() {
    isCapturing.value = false

    if (volumeAnimFrame) {
      cancelAnimationFrame(volumeAnimFrame)
      volumeAnimFrame = null
    }

    if (processorNode) {
      processorNode.disconnect()
      processorNode = null
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

    accumulatedSamples = []
    accumulatedLength = 0
    volume.value = 0
  }

  onUnmounted(() => {
    stopCapture()
  })

  return {
    volume: readonly(volume),
    isCapturing: readonly(isCapturing),
    startCapture,
    stopCapture,
  }
}
