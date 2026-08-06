<script setup lang="ts">
import type { ProductInfo } from '~/types/protocol'

const props = defineProps<{
  product: ProductInfo | null
  reason?: string
}>()

const emit = defineEmits<{
  close: []
}>()

const gradientMap: Record<string, string> = {
  grape: 'from-purple-400 to-purple-600',
  strawberry: 'from-pink-400 to-pink-600',
  apple: 'from-green-400 to-green-600',
  orange: 'from-orange-400 to-orange-600',
  banana: 'from-yellow-300 to-yellow-500',
  lemon: 'from-yellow-200 to-yellow-400',
  blueberry: 'from-blue-400 to-blue-600',
  melon: 'from-emerald-300 to-emerald-500',
  peach: 'from-rose-300 to-rose-500',
}

const gradient = computed(() => {
  if (!props.product) return 'from-gray-400 to-gray-600'
  const name = props.product.name.toLowerCase()
  for (const [key, value] of Object.entries(gradientMap)) {
    if (name.includes(key)) return value
  }
  const hash = props.product.product_id.split('').reduce((a, c) => a + c.charCodeAt(0), 0)
  const colors = Object.values(gradientMap)
  return colors[hash % colors.length]
})

const hasImage = computed(() => !!props.product?.image_url)
const imageError = ref(false)

watch(() => props.product, () => {
  imageError.value = false
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="product"
      class="fixed inset-0 z-50 flex items-center justify-center p-4"
      @click.self="emit('close')"
    >
      <div class="absolute inset-0 bg-black/50" />
      <div class="relative bg-white rounded-2xl shadow-2xl max-w-md w-full max-h-[90vh] overflow-y-auto">
        <!-- Image -->
        <div class="h-48 relative overflow-hidden rounded-t-2xl">
          <img
            v-if="hasImage && !imageError"
            :src="product.image_url"
            :alt="product.name"
            class="w-full h-full object-cover"
            @error="imageError = true"
          />
          <div
            v-else
            class="w-full h-full bg-gradient-to-br flex items-center justify-center"
            :class="gradient"
          >
            <span class="text-white text-2xl font-bold drop-shadow">{{ product.name }}</span>
          </div>
          <button
            class="absolute top-3 right-3 w-8 h-8 bg-white/80 hover:bg-white rounded-full flex items-center justify-center text-gray-600 hover:text-gray-900 transition-colors"
            @click="emit('close')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
            </svg>
          </button>
        </div>

        <div class="p-6">
          <h2 class="text-xl font-bold text-gray-900 mb-3">{{ product.name }}</h2>

          <div v-if="product.tags?.length" class="flex flex-wrap gap-1.5 mb-4">
            <span
              v-for="tag in product.tags"
              :key="tag"
              class="text-sm px-3 py-1 bg-indigo-50 text-indigo-600 rounded-full"
            >{{ tag }}</span>
          </div>

          <p class="text-gray-700 leading-relaxed mb-4">{{ product.description }}</p>

          <div v-if="reason" class="bg-amber-50 border border-amber-200 rounded-lg p-3">
            <p class="text-sm text-amber-800">
              <span class="font-medium">推薦理由:</span> {{ reason }}
            </p>
          </div>

          <button
            class="mt-6 w-full py-2.5 px-4 bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-lg text-sm font-medium transition-colors"
            @click="emit('close')"
          >
            閉じる
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
