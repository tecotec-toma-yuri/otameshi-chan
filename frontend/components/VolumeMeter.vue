<script setup lang="ts">
const props = defineProps<{
  volume: number
}>()

const widthPercent = computed(() => Math.round(props.volume * 100))

const barColor = computed(() => {
  if (props.volume < 0.4) return 'bg-green-500'
  if (props.volume < 0.7) return 'bg-yellow-500'
  return 'bg-red-500'
})
</script>

<template>
  <div class="w-full">
    <div class="flex items-center gap-2">
      <span class="text-xs text-gray-500 w-10 shrink-0">音量</span>
      <div class="flex-1 h-3 bg-gray-200 rounded-full overflow-hidden">
        <div
          class="h-full rounded-full transition-all duration-75"
          :class="barColor"
          :style="{ width: `${widthPercent}%` }"
        />
      </div>
    </div>
  </div>
</template>
