<script setup lang="ts">
import type { ProductInfo } from '~/types/protocol'

defineProps<{
  products: ProductInfo[]
}>()
</script>

<template>
  <div v-if="products.length > 0" class="py-2">
    <!-- Single product: centered -->
    <div v-if="products.length === 1" class="flex justify-center">
      <div class="w-full max-w-xs">
        <ProductCard :product="products[0]" />
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
      />
    </div>
  </div>
</template>
