<script setup lang="ts">
import type { ProductInfo } from '~/types/protocol'

const props = defineProps<{
  product: ProductInfo
  reason?: string
}>()

const emit = defineEmits<{
  showDetail: [product: ProductInfo, reason?: string]
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
  const name = props.product.name.toLowerCase()
  for (const [key, value] of Object.entries(gradientMap)) {
    if (name.includes(key)) return value
  }
  const hash = props.product.product_id.split('').reduce((a, c) => a + c.charCodeAt(0), 0)
  const colors = Object.values(gradientMap)
  return colors[hash % colors.length]
})

const hasImage = computed(() => !!props.product.image_url)
const imageError = ref(false)
</script>

<template>
  <div class="bg-white rounded-xl shadow-md hover:shadow-lg transition-shadow duration-200 overflow-hidden flex flex-col">
    <!-- Product image or gradient fallback -->
    <div class="h-36 relative overflow-hidden">
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
        <span class="text-white text-lg font-bold drop-shadow">{{ product.name }}</span>
      </div>
    </div>

    <div class="p-4 flex flex-col flex-1">
      <h3 class="font-bold text-gray-900 text-base mb-1">{{ product.name }}</h3>
      <div v-if="product.tags?.length" class="flex flex-wrap gap-1 mb-2">
        <span
          v-for="tag in product.tags"
          :key="tag"
          class="text-xs px-2 py-0.5 bg-indigo-50 text-indigo-600 rounded-full"
        >{{ tag }}</span>
      </div>
      <p class="text-sm text-gray-600 line-clamp-2 flex-1">{{ product.description }}</p>
      <button
        class="mt-3 w-full py-2 px-4 bg-indigo-500 hover:bg-indigo-600 text-white rounded-lg text-sm font-medium transition-colors"
        @click="emit('showDetail', product, reason)"
      >
        詳細を見る
      </button>
    </div>
  </div>
</template>
