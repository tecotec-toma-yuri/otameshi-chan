<script setup lang="ts">
import type { ProductInfo } from '~/types/protocol'

const props = defineProps<{
  product: ProductInfo
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
  // Deterministic color from product_id
  const hash = props.product.product_id.split('').reduce((a, c) => a + c.charCodeAt(0), 0)
  const colors = Object.values(gradientMap)
  return colors[hash % colors.length]
})

const formattedPrice = computed(() =>
  new Intl.NumberFormat('ja-JP', { style: 'currency', currency: 'JPY' }).format(props.product.price)
)

function showDetail() {
  alert(
    `${props.product.name}\n\n` +
    `価格: ${formattedPrice.value}\n\n` +
    `${props.product.description}\n\n` +
    `商品ID: ${props.product.product_id}`
  )
}
</script>

<template>
  <div class="bg-white rounded-xl shadow-md hover:shadow-lg transition-shadow duration-200 overflow-hidden flex flex-col">
    <!-- Placeholder image -->
    <div
      class="h-36 bg-gradient-to-br flex items-center justify-center"
      :class="gradient"
    >
      <span class="text-white text-lg font-bold drop-shadow">{{ product.name }}</span>
    </div>

    <div class="p-4 flex flex-col flex-1">
      <h3 class="font-bold text-gray-900 text-base mb-1">{{ product.name }}</h3>
      <p class="text-lg font-semibold text-indigo-600 mb-2">{{ formattedPrice }}</p>
      <p class="text-sm text-gray-600 line-clamp-2 flex-1">{{ product.description }}</p>
      <button
        class="mt-3 w-full py-2 px-4 bg-indigo-500 hover:bg-indigo-600 text-white rounded-lg text-sm font-medium transition-colors"
        @click="showDetail"
      >
        詳細
      </button>
    </div>
  </div>
</template>
