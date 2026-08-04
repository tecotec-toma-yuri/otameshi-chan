<script setup lang="ts">
import type { ConversationMessage } from '~/composables/useSession'

const props = defineProps<{
  messages: ConversationMessage[]
}>()

const container = ref<HTMLElement | null>(null)

// Auto-scroll to bottom on new messages
watch(
  () => props.messages.length,
  () => {
    nextTick(() => {
      if (container.value) {
        container.value.scrollTop = container.value.scrollHeight
      }
    })
  }
)

// Also scroll when streaming text updates
watch(
  () => props.messages[props.messages.length - 1]?.text,
  () => {
    nextTick(() => {
      if (container.value) {
        container.value.scrollTop = container.value.scrollHeight
      }
    })
  }
)

function formatTime(date: Date) {
  return new Date(date).toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit' })
}

function formatLatency(latency: NonNullable<ConversationMessage['latency']>) {
  const parts: string[] = []
  if (latency.stt_ms > 0) {
    parts.push(`STT ${latency.stt_ms}ms`)
  }
  parts.push(`LLM ${latency.llm_total_ms}ms (TTFB ${latency.llm_ttfb_ms}ms)`)
  parts.push(`TTS ${latency.tts_ms}ms`)
  parts.push(`合計 ${latency.total_ms}ms`)
  return parts.join(' / ')
}
</script>

<template>
  <div ref="container" class="h-full min-h-0 overflow-y-auto px-4 py-4 space-y-4">
    <div v-if="messages.length === 0" class="flex items-center justify-center h-full text-gray-400">
      <p>メッセージを送信して会話を開始してください</p>
    </div>

    <div
      v-for="(msg, i) in messages"
      :key="i"
      class="flex"
      :class="{
        'justify-end': msg.role === 'user',
        'justify-start': msg.role === 'ai',
        'justify-center': msg.role === 'system',
      }"
    >
      <!-- System message -->
      <div
        v-if="msg.role === 'system'"
        class="bg-gray-100 text-gray-600 text-sm px-4 py-2 rounded-full max-w-md text-center"
      >
        {{ msg.text }}
        <span class="text-xs text-gray-400 ml-2">{{ formatTime(msg.timestamp) }}</span>
      </div>

      <!-- User message -->
      <div
        v-else-if="msg.role === 'user'"
        class="max-w-[75%]"
      >
        <div class="bg-blue-500 text-white px-4 py-2 rounded-2xl rounded-br-sm min-w-[4.5rem]">
          <div v-if="msg.loading" class="flex items-center gap-1.5 py-0.5" aria-label="認識中">
            <span class="w-1.5 h-1.5 rounded-full bg-white/90 animate-pulse-dot" />
            <span class="w-1.5 h-1.5 rounded-full bg-white/90 animate-pulse-dot [animation-delay:150ms]" />
            <span class="w-1.5 h-1.5 rounded-full bg-white/90 animate-pulse-dot [animation-delay:300ms]" />
          </div>
          <p v-else class="whitespace-pre-wrap">{{ msg.text }}</p>
        </div>
        <p class="text-xs text-gray-400 text-right mt-1">
          {{ msg.loading ? '認識中...' : formatTime(msg.timestamp) }}
        </p>
      </div>

      <!-- AI message -->
      <div
        v-else
        class="max-w-[75%]"
      >
        <div class="bg-gray-100 text-gray-900 px-4 py-2 rounded-2xl rounded-bl-sm">
          <p class="whitespace-pre-wrap">
            {{ msg.text }}
            <span
              v-if="msg.streaming"
              class="inline-block w-1.5 h-4 bg-gray-500 ml-0.5 animate-blink align-text-bottom"
            />
          </p>
        </div>
        <p class="text-xs text-gray-400 mt-1">{{ formatTime(msg.timestamp) }}</p>
        <p v-if="msg.latency" class="text-xs text-indigo-500/80 mt-0.5 font-mono">
          {{ formatLatency(msg.latency) }}
        </p>

        <!-- Inline product cards -->
        <ProductCardList v-if="msg.products && msg.products.length > 0" :products="msg.products" />
      </div>
    </div>
  </div>
</template>

<style scoped>
@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}
.animate-blink {
  animation: blink 1s step-end infinite;
}

@keyframes pulse-dot {
  0%, 80%, 100% { opacity: 0.35; transform: translateY(0); }
  40% { opacity: 1; transform: translateY(-2px); }
}
.animate-pulse-dot {
  animation: pulse-dot 1s ease-in-out infinite;
}
</style>
