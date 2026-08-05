<script setup lang="ts">
import type { SessionConfig } from '~/types/protocol'

const emit = defineEmits<{
  'update:config': [config: SessionConfig]
}>()

const collapsed = ref(true)

const recommendationMode = ref<'ai_driven' | 'sequential'>('ai_driven')
const postBehavior = ref<'return_to_conversation' | 'ask_interest'>('return_to_conversation')

function emitConfig() {
  const config: SessionConfig = {
    recommendation_mode: recommendationMode.value,
    post_recommendation_behavior: postBehavior.value,
  }
  emit('update:config', config)
}

watch([recommendationMode, postBehavior], () => {
  emitConfig()
})

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
            AI駆動 (興味度判定 → 商品推薦)
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input
              v-model="recommendationMode"
              type="radio"
              value="sequential"
              class="text-indigo-600"
            />
            シーケンシャル (推薦後に関連商品を連続紹介)
          </label>
        </div>
        <p v-if="recommendationMode === 'sequential'" class="text-xs text-gray-400 mt-2">
          各商品の「関連商品ID」に設定された商品を順に紹介します。設定画面で商品ごとに関連商品を指定してください。
        </p>
      </fieldset>

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
