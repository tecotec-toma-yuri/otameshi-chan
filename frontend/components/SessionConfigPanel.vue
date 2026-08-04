<script setup lang="ts">
import type { SessionConfig } from '~/types/protocol'

const emit = defineEmits<{
  'update:config': [config: SessionConfig]
}>()

const collapsed = ref(true)

const recommendationMode = ref<'ai_driven' | 'sequential'>('ai_driven')
const sequentialItemsJson = ref('[]')
const sequentialIntervalSec = ref(30)
const postBehavior = ref<'return_to_conversation' | 'ask_interest'>('return_to_conversation')

function emitConfig() {
  const config: SessionConfig = {
    recommendation_mode: recommendationMode.value,
    post_recommendation_behavior: postBehavior.value,
  }

  if (recommendationMode.value === 'sequential') {
    try {
      config.sequential_items = JSON.parse(sequentialItemsJson.value)
    } catch {
      config.sequential_items = []
    }
    config.sequential_interval_sec = sequentialIntervalSec.value
  }

  emit('update:config', config)
}

// Emit on any change
watch([recommendationMode, sequentialItemsJson, sequentialIntervalSec, postBehavior], () => {
  emitConfig()
})

// Emit initial config
onMounted(() => {
  emitConfig()
})
</script>

<template>
  <div class="border border-gray-200 rounded-lg overflow-hidden">
    <button
      class="w-full px-4 py-3 bg-gray-50 text-left text-sm font-medium text-gray-700 flex items-center justify-between hover:bg-gray-100 transition-colors"
      @click="collapsed = !collapsed"
    >
      <span>セッション設定</span>
      <svg
        class="w-4 h-4 transition-transform"
        :class="{ 'rotate-180': !collapsed }"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </button>

    <div v-show="!collapsed" class="p-4 space-y-4 bg-white">
      <!-- Recommendation mode -->
      <fieldset>
        <legend class="text-sm font-medium text-gray-700 mb-2">レコメンドモード</legend>
        <div class="space-y-2">
          <label class="flex items-center gap-2 text-sm">
            <input
              v-model="recommendationMode"
              type="radio"
              value="ai_driven"
              class="text-indigo-600"
            />
            AI駆動 (会話に基づく推薦)
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input
              v-model="recommendationMode"
              type="radio"
              value="sequential"
              class="text-indigo-600"
            />
            シーケンシャル (事前定義順)
          </label>
        </div>
      </fieldset>

      <!-- Sequential-only fields -->
      <template v-if="recommendationMode === 'sequential'">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">
            商品リスト (JSON)
          </label>
          <textarea
            v-model="sequentialItemsJson"
            class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm font-mono resize-y focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
            rows="3"
            placeholder='[{"product_ids": ["P001", "P002"]}]'
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">
            推薦間隔 (秒)
          </label>
          <input
            v-model.number="sequentialIntervalSec"
            type="number"
            min="5"
            max="300"
            class="w-24 border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
          />
        </div>
      </template>

      <!-- Post recommendation behavior -->
      <fieldset>
        <legend class="text-sm font-medium text-gray-700 mb-2">推薦後の動作</legend>
        <div class="space-y-2">
          <label class="flex items-center gap-2 text-sm">
            <input
              v-model="postBehavior"
              type="radio"
              value="return_to_conversation"
              class="text-indigo-600"
            />
            会話に戻る
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input
              v-model="postBehavior"
              type="radio"
              value="ask_interest"
              class="text-indigo-600"
            />
            興味を確認する
          </label>
        </div>
      </fieldset>
    </div>
  </div>
</template>
