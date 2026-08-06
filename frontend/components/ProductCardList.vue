<script setup lang="ts">
import type { ProductInfo } from '~/types/protocol'

defineProps<{
  products: ProductInfo[]
  reason?: string
}>()

const detailProduct = ref<ProductInfo | null>(null)
const detailReason = ref<string | undefined>()

function openDetail(product: ProductInfo, reason?: string) {
  detailProduct.value = product
  detailReason.value = reason
}
</script>

<template>
  <div v-if="products.length > 0" class="py-2">
    <!-- Recommendation reason -->
    <p v-if="reason" class="text-sm text-gray-500 mb-2 italic">{{ reason }}</p>

    <!-- Single product: centered -->
    <div v-if="products.length === 1" class="flex justify-center">
      <div class="w-full max-w-xs">
        <ProductCard :product="products[0]" :reason="reason" @show-detail="openDetail" />
      </div>
    </div>

    <!-- Multiple products: responsive grid -->
    <div
      v-else
      class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4"
    >
      <ProductCard
        v-for="product in products"
        :key="product.product_id"
        :product="product"
        :reason="reason"
        @show-detail="openDetail"
      />
    </div>

    <ProductDetailModal
      :product="detailProduct"
      :reason="detailReason"
      @close="detailProduct = null"
    />
  </div>
</template>
