export function useAudioPlayback() {
  const isPlaying = ref(false)
  const volume = ref(1)
  const isMuted = ref(false)

  let audioContext: AudioContext | null = null
  let gainNode: GainNode | null = null
  let currentSource: AudioBufferSourceNode | null = null
  const MAX_QUEUE_SIZE = 50
  const queue: AudioBuffer[] = []
  let playing = false
  let pendingDecodes = 0

  function ensureContext() {
    if (!audioContext) {
      audioContext = new AudioContext()
      gainNode = audioContext.createGain()
      gainNode.connect(audioContext.destination)
      gainNode.gain.value = isMuted.value ? 0 : volume.value
    }
    return audioContext
  }

  async function resumeContext() {
    const ctx = ensureContext()
    if (ctx.state === 'suspended') {
      await ctx.resume()
    }
  }

  function setVolume(val: number) {
    volume.value = Math.max(0, Math.min(1, val))
    if (gainNode && !isMuted.value) {
      gainNode.gain.value = volume.value
    }
  }

  function toggleMute() {
    isMuted.value = !isMuted.value
    if (gainNode) {
      gainNode.gain.value = isMuted.value ? 0 : volume.value
    }
  }

  function base64ToArrayBuffer(base64: string): ArrayBuffer {
    const binaryString = atob(base64)
    const bytes = new Uint8Array(binaryString.length)
    for (let i = 0; i < binaryString.length; i++) {
      bytes[i] = binaryString.charCodeAt(i)
    }
    return bytes.buffer
  }

  async function playAudio(base64Audio: string) {
    if (!base64Audio) return

    pendingDecodes++
    isPlaying.value = true
    try {
      await resumeContext()
      const ctx = ensureContext()
      const arrayBuffer = base64ToArrayBuffer(base64Audio)
      const audioBuffer = await ctx.decodeAudioData(arrayBuffer)
      if (queue.length >= MAX_QUEUE_SIZE) {
        queue.shift()
      }
      queue.push(audioBuffer)
      if (!playing) {
        playNext()
      }
    } catch (e) {
      console.error('Failed to decode audio:', e)
    } finally {
      pendingDecodes--
      if (pendingDecodes === 0 && !playing && queue.length === 0) {
        isPlaying.value = false
      }
    }
  }

  function playNext() {
    if (queue.length === 0) {
      playing = false
      isPlaying.value = false
      currentSource = null
      return
    }

    playing = true
    isPlaying.value = true
    const ctx = ensureContext()
    const buffer = queue.shift()!
    const source = ctx.createBufferSource()
    source.buffer = buffer
    source.connect(gainNode!)
    currentSource = source

    source.onended = () => {
      currentSource = null
      playNext()
    }

    source.start()
  }

  function waitUntilDone(): Promise<void> {
    if (!playing && queue.length === 0 && pendingDecodes === 0) return Promise.resolve()
    return new Promise((resolve) => {
      const check = () => {
        if (!playing && queue.length === 0 && pendingDecodes === 0) {
          resolve()
        } else {
          setTimeout(check, 100)
        }
      }
      check()
    })
  }

  function stopAndClear() {
    queue.length = 0
    pendingDecodes = 0
    if (currentSource) {
      try {
        currentSource.stop()
      } catch {
        // already stopped
      }
      currentSource = null
    }
    playing = false
    isPlaying.value = false
  }

  onUnmounted(() => {
    stopAndClear()
    if (audioContext) {
      audioContext.close()
      audioContext = null
    }
  })

  return {
    isPlaying: readonly(isPlaying),
    volume: readonly(volume),
    isMuted: readonly(isMuted),
    playAudio,
    waitUntilDone,
    stopAndClear,
    resumeContext,
    setVolume,
    toggleMute,
  }
}
