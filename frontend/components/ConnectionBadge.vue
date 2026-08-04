<script setup lang="ts">
const props = defineProps<{
  state: 'connected' | 'disconnected' | 'connecting' | 'error' | 'completed'
}>()

const label = computed(() => {
  switch (props.state) {
    case 'connected': return '接続中'
    case 'connecting': return '接続中...'
    case 'disconnected': return '切断中'
    case 'error': return 'エラー'
    case 'completed': return '対話終了'
    default: return '不明'
  }
})

const dotClass = computed(() => {
  switch (props.state) {
    case 'connected': return 'bg-green-500'
    case 'connecting': return 'bg-yellow-500 animate-pulse'
    case 'disconnected': return 'bg-red-500'
    case 'error': return 'bg-red-500'
    case 'completed': return 'bg-gray-400'
    default: return 'bg-gray-400'
  }
})
</script>

<template>
  <div class="flex items-center gap-2 text-sm">
    <span class="inline-block w-2.5 h-2.5 rounded-full" :class="dotClass" />
    <span class="text-gray-700">{{ label }}</span>
  </div>
</template>
