<script setup lang="ts">
import type { ProductConfig } from '~/composables/useAppSettings'

const {
  config,
  models,
  modelsLoading,
  modelsError,
  ttsEngine,
  ttsVoices,
  ttsVoicesLoading,
  ttsVoicesError,
  ttsSpeakers,
  ttsLanguages,
  isMultiSpeaker,
  hasTtsLanguages,
  selectedTtsVoice,
  voicevoxSpeakers,
  loading,
  saving,
  error,
  success,
  fetchConfig,
  fetchModels,
  fetchTtsVoices,
  saveConfig,
} = useAppSettings()

function onEngineChange() {
  fetchTtsVoices()
}

function addProduct() {
  const nextId = `p${String(config.value.products.length + 1).padStart(3, '0')}`
  config.value.products.push({
    id: nextId,
    name: '',
    price: 0,
    description: '',
    image_url: '',
  })
}

function removeProduct(index: number) {
  config.value.products.splice(index, 1)
}

onMounted(() => {
  fetchConfig()
})
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white border-b border-gray-200 px-4 py-3">
      <div class="max-w-3xl mx-auto flex items-center justify-between">
        <div class="flex items-center gap-3">
          <NuxtLink
            to="/assistant"
            class="text-gray-500 hover:text-gray-700 transition-colors"
            title="アシスタントに戻る"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M19 12H5" />
              <path d="M12 19l-7-7 7-7" />
            </svg>
          </NuxtLink>
          <h1 class="text-lg font-bold text-gray-900">設定</h1>
        </div>
        <button
          class="px-4 py-2 text-white text-sm font-medium rounded-lg transition-colors bg-indigo-500 hover:bg-indigo-600 disabled:bg-gray-400 disabled:cursor-not-allowed"
          :disabled="loading || saving"
          @click="saveConfig"
        >
          {{ saving ? '保存中...' : '保存' }}
        </button>
      </div>
    </header>

    <main class="max-w-3xl mx-auto px-4 py-6 space-y-6">
      <div
        v-if="error"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
      >
        {{ error }}
      </div>
      <div
        v-if="success"
        class="rounded-lg border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-700"
      >
        {{ success }}
      </div>

      <div v-if="loading" class="text-sm text-gray-500 py-12 text-center">
        読み込み中...
      </div>

      <template v-else>
        <section class="bg-white border border-gray-200 rounded-lg p-5 space-y-4">
          <div>
            <h2 class="text-base font-semibold text-gray-900">システムプロンプト</h2>
            <p class="text-sm text-gray-500 mt-1">
              AIの役割・話し方・会話方針を定義します。変更は次のセッション接続から反映されます。
            </p>
          </div>
          <textarea
            v-model="config.system_prompt"
            rows="18"
            class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm font-mono leading-relaxed resize-y focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
            placeholder="システムプロンプトを入力..."
          />
        </section>

        <section class="bg-white border border-gray-200 rounded-lg p-5 space-y-4">
          <div class="flex items-start justify-between gap-3">
            <div>
              <h2 class="text-base font-semibold text-gray-900">LLM</h2>
              <p class="text-sm text-gray-500 mt-1">
                API（/models）から取得したモデルを選択します。
              </p>
            </div>
            <button
              type="button"
              class="shrink-0 px-3 py-1.5 text-xs font-medium text-indigo-600 border border-indigo-200 rounded-md hover:bg-indigo-50 disabled:opacity-50"
              :disabled="modelsLoading"
              @click="fetchModels"
            >
              {{ modelsLoading ? '取得中...' : '再取得' }}
            </button>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">モデル</label>
            <select
              v-model="config.llm_model"
              class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 disabled:bg-gray-100"
              :disabled="modelsLoading || models.length === 0"
            >
              <option v-if="models.length === 0" value="">
                {{ modelsLoading ? 'モデル一覧を取得中...' : '選択可能なモデルがありません' }}
              </option>
              <option
                v-for="m in models"
                :key="m.id"
                :value="m.id"
              >
                {{ m.label || (m.preview ? `${m.id} (Preview)` : m.id) }}
              </option>
            </select>
            <p v-if="modelsError" class="text-xs text-red-600 mt-2">
              {{ modelsError }}
            </p>
            <p v-else-if="models.length > 0" class="text-xs text-gray-400 mt-2">
              {{ models.length }} 件のモデルを取得しました
            </p>
          </div>
        </section>

        <section class="bg-white border border-gray-200 rounded-lg p-5 space-y-4">
          <div>
            <h2 class="text-base font-semibold text-gray-900">TTS（音声合成）</h2>
            <div class="mt-2">
              <label class="block text-sm font-medium text-gray-700 mb-1">エンジン</label>
              <select
                v-model="config.tts_engine"
                class="w-full sm:w-64 border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                @change="onEngineChange"
              >
                <option value="piper">Piper</option>
                <option value="voicevox">VOICEVOX</option>
              </select>
              <p class="text-xs text-gray-400 mt-1">変更は保存後、次のセッションから反映されます</p>
            </div>
          </div>

          <!-- VOICEVOX settings -->
          <template v-if="config.tts_engine === 'voicevox'">
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">話者</label>
              <select
                v-model.number="config.voicevox_speaker_id"
                class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 disabled:bg-gray-100"
                :disabled="ttsVoicesLoading || voicevoxSpeakers.length === 0"
              >
                <option v-if="voicevoxSpeakers.length === 0" :value="config.voicevox_speaker_id">
                  {{ ttsVoicesLoading ? '話者一覧を取得中...' : `ID: ${config.voicevox_speaker_id}` }}
                </option>
                <option
                  v-for="s in voicevoxSpeakers"
                  :key="s.id"
                  :value="s.id"
                >
                  {{ s.name }} - {{ s.style }} (ID: {{ s.id }})
                </option>
              </select>
              <p v-if="ttsVoicesError" class="text-xs text-red-600 mt-1">{{ ttsVoicesError }}</p>
              <p v-else-if="voicevoxSpeakers.length > 0" class="text-xs text-gray-400 mt-1">
                {{ voicevoxSpeakers.length }} 件の話者を取得しました
              </p>
            </div>
            <div class="w-48">
              <label class="block text-sm font-medium text-gray-700 mb-1">話速</label>
              <input
                v-model.number="config.voicevox_speed_scale"
                type="number"
                min="0.5"
                max="2.0"
                step="0.1"
                class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
              />
              <p class="text-xs text-gray-400 mt-1">1.0 が標準速度</p>
            </div>
          </template>

          <!-- Piper TTS settings -->
          <template v-else>
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">音声</label>
              <select
                v-model="config.tts_voice"
                class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 disabled:bg-gray-100"
                :disabled="ttsVoicesLoading || ttsVoices.length === 0"
              >
                <option
                  v-for="v in ttsVoices"
                  :key="v.id"
                  :value="v.id"
                >
                  {{ v.label }}
                </option>
              </select>
              <p v-if="ttsVoicesError" class="text-xs text-amber-600 mt-1">
                {{ ttsVoicesError }}（フォールバック一覧を表示中）
              </p>
            </div>
            <div v-if="isMultiSpeaker" class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">話者</label>
                <select
                  v-model.number="config.tts_speaker_id"
                  class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                >
                  <option
                    v-for="s in ttsSpeakers"
                    :key="s.id"
                    :value="s.id"
                  >
                    {{ s.label }}
                  </option>
                </select>
              </div>
            </div>
            <div v-if="hasTtsLanguages">
              <label class="block text-sm font-medium text-gray-700 mb-1">読み上げ言語</label>
              <select
                v-model="config.tts_language"
                class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
              >
                <option value="auto">auto（STT検出言語に追従）</option>
                <option
                  v-for="l in ttsLanguages"
                  :key="l.id"
                  :value="l.id"
                >
                  {{ l.label }}
                </option>
              </select>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">話速 (length_scale)</label>
                <input
                  v-model.number="config.tts_length_scale"
                  type="number" min="0.5" max="3" step="0.1"
                  class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                />
                <p class="text-xs text-gray-400 mt-1">大きいほどゆっくり</p>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">声の変動 (noise_scale)</label>
                <input
                  v-model.number="config.tts_noise_scale"
                  type="number" min="0" max="2" step="0.001"
                  class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">音素長の変動 (noise_w)</label>
                <input
                  v-model.number="config.tts_noise_w"
                  type="number" min="0" max="2" step="0.1"
                  class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                />
              </div>
            </div>
          </template>
        </section>

        <section class="bg-white border border-gray-200 rounded-lg p-5 space-y-4">
          <div>
            <h2 class="text-base font-semibold text-gray-900">STT（音声認識）</h2>
            <p class="text-sm text-gray-500 mt-1">音声認識エンジンと言語を設定します。変更は次のセッション接続から反映されます。</p>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">エンジン</label>
            <select
              v-model="config.stt_engine"
              class="w-full sm:w-64 border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
            >
              <option value="whisper">faster-whisper（ローカル）</option>
              <option value="groq">Groq（クラウド API）</option>
            </select>
            <p v-if="config.stt_engine === 'groq'" class="text-xs text-gray-400 mt-1">
              Groq API を使用します。GROQ_API_KEY（または OPENAI_API_KEY）の設定が必要です。
            </p>
            <p v-else class="text-xs text-gray-400 mt-1">
              ローカルの faster-whisper サーバーを使用します（CTranslate2 ベース、Opus/WebM 対応）
            </p>
          </div>
          <div v-if="config.stt_engine === 'groq'">
            <label class="block text-sm font-medium text-gray-700 mb-1">STT モデル</label>
            <select
              v-model="config.stt_model"
              class="w-full sm:w-64 border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
            >
              <option value="whisper-large-v3-turbo">whisper-large-v3-turbo（推奨・高速）</option>
              <option value="whisper-large-v3">whisper-large-v3（高精度）</option>
              <option value="distil-whisper-large-v3-en">distil-whisper-large-v3-en（英語特化・最速）</option>
            </select>
            <p class="text-xs text-gray-400 mt-1">Groq で使用する Whisper モデルを選択します</p>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">言語</label>
            <select
              v-model="config.stt_language"
              class="w-full sm:w-64 border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
            >
              <option value="auto">auto（自動検出）</option>
              <option value="ja">日本語</option>
              <option value="en">英語</option>
              <option value="zh">中国語</option>
              <option value="ko">韓国語</option>
              <option value="fr">フランス語</option>
              <option value="de">ドイツ語</option>
              <option value="es">スペイン語</option>
              <option value="pt">ポルトガル語</option>
              <option value="it">イタリア語</option>
              <option value="ru">ロシア語</option>
              <option value="ar">アラビア語</option>
              <option value="hi">ヒンディー語</option>
              <option value="th">タイ語</option>
              <option value="vi">ベトナム語</option>
              <option value="id">インドネシア語</option>
            </select>
            <p class="text-xs text-gray-400 mt-1">「auto」で自動検出になり、検出された言語がTTSにも連携されます</p>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Initial Prompt</label>
            <input
              v-model="config.stt_prompt"
              type="text"
              class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
              placeholder="ハイチュウ、グレープ、ストロベリー、グリーンアップル"
            />
            <p class="text-xs text-gray-400 mt-1">固有名詞や専門用語をカンマ区切りで入力すると認識精度が向上します</p>
          </div>
        </section>

        <section class="bg-white border border-gray-200 rounded-lg p-5 space-y-4">
          <div class="flex items-start justify-between gap-3">
            <div>
              <h2 class="text-base font-semibold text-gray-900">取扱商品</h2>
              <p class="text-sm text-gray-500 mt-1">
                おすすめ対象の商品を設定します。セッション開始時にシステムプロンプトへ自動注入され、Function Calling で商品紹介に使用されます。
              </p>
            </div>
            <button
              type="button"
              class="shrink-0 px-3 py-1.5 text-xs font-medium text-indigo-600 border border-indigo-200 rounded-md hover:bg-indigo-50"
              @click="addProduct"
            >
              + 追加
            </button>
          </div>

          <div v-if="config.products.length === 0" class="text-sm text-gray-400 py-4 text-center border border-dashed border-gray-300 rounded-md">
            商品が登録されていません。「+ 追加」ボタンで追加してください。
          </div>

          <div
            v-for="(product, idx) in config.products"
            :key="idx"
            class="border border-gray-200 rounded-md p-4 space-y-3"
          >
            <div class="flex items-center justify-between">
              <span class="text-xs font-mono text-gray-400">{{ idx + 1 }} / {{ config.products.length }}</span>
              <button
                type="button"
                class="text-xs text-red-500 hover:text-red-700"
                @click="removeProduct(idx)"
              >
                削除
              </button>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label class="block text-xs font-medium text-gray-600 mb-1">商品ID</label>
                <input
                  v-model="product.id"
                  type="text"
                  class="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                  placeholder="h001"
                />
              </div>
              <div>
                <label class="block text-xs font-medium text-gray-600 mb-1">商品名</label>
                <input
                  v-model="product.name"
                  type="text"
                  class="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                  placeholder="ハイチュウ ＜グレープ＞"
                />
              </div>
              <div>
                <label class="block text-xs font-medium text-gray-600 mb-1">価格（円）</label>
                <input
                  v-model.number="product.price"
                  type="number"
                  min="0"
                  class="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                  placeholder="140"
                />
              </div>
              <div>
                <label class="block text-xs font-medium text-gray-600 mb-1">画像URL</label>
                <input
                  v-model="product.image_url"
                  type="text"
                  class="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                  placeholder="/images/h001.png"
                />
              </div>
            </div>
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">説明</label>
              <input
                v-model="product.description"
                type="text"
                class="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                placeholder="ジューシーな果汁感あふれる定番フレーバー"
              />
            </div>
          </div>
        </section>
      </template>
    </main>
  </div>
</template>
