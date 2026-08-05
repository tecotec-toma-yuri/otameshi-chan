<script setup lang="ts">
const props = defineProps<{
  status: string
  inputMode?: 'text' | 'mic'
  connectionState?: string
}>()

const labelMap: Record<string, string> = {
  disconnected: '未接続',
  connecting: '接続中...',
  idle: 'サーバー待受中',
  listening: 'リッスン中',
  processing: '処理中',
  ai_speaking: 'AI発話中',
  barge_in: '割り込み検知中',
  error: 'エラー',
  completed: '対話終了',
}

const displayStatus = computed(() => {
  if (props.connectionState === 'disconnected' || props.connectionState === 'connecting' || props.connectionState === 'error') {
    return props.connectionState
  }
  if (props.status === 'listening' && props.inputMode === 'text') {
    return 'text_ready'
  }
  return props.status
})

const label = computed(() => {
  if (displayStatus.value === 'text_ready') return 'テキスト入力待機'
  return labelMap[displayStatus.value] || displayStatus.value
})

const isActive = computed(() =>
  ['listening', 'text_ready', 'processing', 'ai_speaking', 'barge_in', 'connecting'].includes(displayStatus.value)
)

const colorClass = computed(() => {
  switch (displayStatus.value) {
    case 'disconnected': return 'text-gray-500 bg-gray-50'
    case 'connecting': return 'text-yellow-600 bg-yellow-50'
    case 'listening': return 'text-blue-600 bg-blue-50'
    case 'text_ready': return 'text-indigo-600 bg-indigo-50'
    case 'processing': return 'text-yellow-600 bg-yellow-50'
    case 'ai_speaking': return 'text-green-600 bg-green-50'
    case 'barge_in': return 'text-orange-600 bg-orange-50'
    case 'error': return 'text-red-600 bg-red-50'
    case 'completed': return 'text-gray-600 bg-gray-50'
    default: return 'text-gray-500 bg-gray-50'
  }
})

const dotClass = computed(() => {
  switch (displayStatus.value) {
    case 'connecting': return 'bg-yellow-500'
    case 'listening': return 'bg-blue-500'
    case 'text_ready': return 'bg-indigo-500'
    case 'processing': return 'bg-yellow-500'
    case 'ai_speaking': return 'bg-green-500'
    case 'barge_in': return 'bg-orange-500'
    default: return 'bg-gray-400'
  }
})
</script>

<template>
  <div
    class="inline-flex items-center gap-2 px-3 py-1 rounded-full text-sm font-medium"
    :class="colorClass"
  >
    <span
      v-if="isActive"
      class="inline-block w-2 h-2 rounded-full animate-pulse"
      :class="dotClass"
    />
    {{ label }}
  </div>
</template>
