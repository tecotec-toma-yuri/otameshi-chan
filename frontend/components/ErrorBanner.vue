<script setup lang="ts">
const props = defineProps<{
  message: string
  type: 'error' | 'warning'
}>()

const emit = defineEmits<{
  close: []
}>()

const visible = ref(true)
let timer: ReturnType<typeof setTimeout> | null = null

watch(
  () => props.message,
  (val) => {
    if (val) {
      visible.value = true
      if (timer) clearTimeout(timer)
      timer = setTimeout(() => {
        visible.value = false
        emit('close')
      }, 5000)
    }
  },
  { immediate: true }
)

function close() {
  visible.value = false
  if (timer) clearTimeout(timer)
  emit('close')
}

onUnmounted(() => {
  if (timer) clearTimeout(timer)
})
</script>

<template>
  <Transition name="slide">
    <div
      v-if="visible && message"
      class="fixed top-0 left-0 right-0 z-50 px-4 py-3 flex items-center justify-between"
      :class="{
        'bg-red-500 text-white': type === 'error',
        'bg-yellow-400 text-yellow-900': type === 'warning',
      }"
    >
      <p class="text-sm font-medium">{{ message }}</p>
      <button
        class="ml-4 shrink-0 p-1 rounded hover:bg-black/10 transition-colors"
        @click="close"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  </Transition>
</template>

<style scoped>
.slide-enter-active,
.slide-leave-active {
  transition: transform 0.3s ease, opacity 0.3s ease;
}
.slide-enter-from,
.slide-leave-to {
  transform: translateY(-100%);
  opacity: 0;
}
</style>
