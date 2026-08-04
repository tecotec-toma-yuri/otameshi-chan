import { Rnnoise } from '@shiguredo/rnnoise-wasm'
import type { DenoiseState } from '@shiguredo/rnnoise-wasm'

export function useNoiseSuppression() {
  let rnnoise: Awaited<ReturnType<typeof Rnnoise.load>> | null = null
  let denoiseState: DenoiseState | null = null
  let frameSize = 480
  let inputBuffer = new Float32Array(0)

  const ready = ref(false)

  async function init() {
    if (rnnoise) return
    rnnoise = await Rnnoise.load()
    frameSize = rnnoise.frameSize
    denoiseState = rnnoise.createDenoiseState()
    inputBuffer = new Float32Array(0)
    ready.value = true
  }

  function processChunk(samples: Float32Array): Float32Array {
    if (!denoiseState) return samples

    // Accumulate samples
    const combined = new Float32Array(inputBuffer.length + samples.length)
    combined.set(inputBuffer)
    combined.set(samples, inputBuffer.length)

    const outputChunks: Float32Array[] = []
    let offset = 0

    while (offset + frameSize <= combined.length) {
      const frame = new Float32Array(frameSize)
      for (let i = 0; i < frameSize; i++) {
        // RNNoise expects 16-bit PCM scale (-32768 to 32767)
        frame[i] = combined[offset + i] * 32768
      }

      denoiseState.processFrame(frame)

      // Convert back to -1.0 to 1.0
      const out = new Float32Array(frameSize)
      for (let i = 0; i < frameSize; i++) {
        out[i] = frame[i] / 32768
      }
      outputChunks.push(out)
      offset += frameSize
    }

    // Save remaining samples
    inputBuffer = combined.slice(offset)

    if (outputChunks.length === 0) return new Float32Array(0)

    let totalLen = 0
    for (const c of outputChunks) totalLen += c.length
    const result = new Float32Array(totalLen)
    let pos = 0
    for (const c of outputChunks) {
      result.set(c, pos)
      pos += c.length
    }
    return result
  }

  function reset() {
    inputBuffer = new Float32Array(0)
  }

  function destroy() {
    if (denoiseState) {
      denoiseState.destroy()
      denoiseState = null
    }
    rnnoise = null
    ready.value = false
  }

  return {
    ready: readonly(ready),
    init,
    processChunk,
    reset,
    destroy,
  }
}
