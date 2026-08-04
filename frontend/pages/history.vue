<script setup lang="ts">
interface SessionSummary {
  id: string
  started_at: string
  ended_at: string
  turns: number
  preview: string
}

interface Message {
  role: string
  content: string
}

interface SessionDetail {
  id: string
  started_at: string
  ended_at: string
  messages: Message[]
}

const config = useRuntimeConfig()
const apiUrl = config.public.apiUrl as string

const sessions = ref<SessionSummary[]>([])
const selected = ref<SessionDetail | null>(null)
const loading = ref(true)
const error = ref('')

async function fetchSessions() {
  loading.value = true
  error.value = ''
  try {
    const data = await $fetch<SessionSummary[]>(`${apiUrl}/api/history`)
    sessions.value = data || []
  } catch (e: any) {
    error.value = e.message || '履歴の取得に失敗しました'
  } finally {
    loading.value = false
  }
}

async function selectSession(id: string) {
  try {
    selected.value = await $fetch<SessionDetail>(`${apiUrl}/api/history/${id}`)
  } catch (e: any) {
    error.value = e.message || '会話の取得に失敗しました'
  }
}

function resumeSession(id: string) {
  navigateTo({ path: '/assistant', query: { restore: id } })
}

function formatDate(dateStr: string) {
  const d = new Date(dateStr)
  return d.toLocaleString('ja-JP', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

function durationMinutes(start: string, end: string) {
  const ms = new Date(end).getTime() - new Date(start).getTime()
  const mins = Math.round(ms / 60000)
  return mins < 1 ? '1分未満' : `${mins}分`
}

onMounted(() => {
  fetchSessions()
})
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white border-b border-gray-200 px-4 py-3">
      <div class="max-w-4xl mx-auto flex items-center gap-3">
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
        <h1 class="text-lg font-bold text-gray-900">会話履歴</h1>
      </div>
    </header>

    <main class="max-w-4xl mx-auto p-4">
      <div v-if="error" class="bg-red-50 text-red-700 p-3 rounded-lg mb-4 text-sm">{{ error }}</div>

      <div v-if="loading" class="text-center text-gray-500 py-8">読み込み中...</div>

      <div v-else-if="sessions.length === 0" class="text-center text-gray-500 py-8">
        会話履歴がありません。アシスタントとの会話を終了すると、ここに保存されます。
      </div>

      <div v-else class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <!-- セッション一覧 -->
        <div class="md:col-span-1 space-y-2">
          <button
            v-for="s in sessions"
            :key="s.id"
            class="w-full text-left p-3 rounded-lg border transition-colors"
            :class="selected?.id === s.id ? 'bg-indigo-50 border-indigo-300' : 'bg-white border-gray-200 hover:bg-gray-50'"
            @click="selectSession(s.id)"
          >
            <div class="text-sm font-medium text-gray-900">{{ formatDate(s.started_at) }}</div>
            <div class="text-xs text-gray-500 mt-1">
              {{ durationMinutes(s.started_at, s.ended_at) }} ・ {{ s.turns }}ターン
            </div>
            <div v-if="s.preview" class="text-xs text-gray-600 mt-1 truncate">{{ s.preview }}</div>
          </button>
        </div>

        <!-- 会話詳細 -->
        <div class="md:col-span-2">
          <div v-if="!selected" class="text-center text-gray-400 py-12">
            左のセッションを選択してください
          </div>
          <div v-else class="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
            <div class="flex items-center justify-between border-b pb-2">
              <div class="text-sm text-gray-500">
                {{ formatDate(selected.started_at) }} 〜 {{ formatDate(selected.ended_at) }}
                （{{ durationMinutes(selected.started_at, selected.ended_at) }}）
              </div>
              <button
                class="px-3 py-1 text-sm font-medium text-white bg-indigo-500 hover:bg-indigo-600 rounded-lg transition-colors"
                @click="resumeSession(selected.id)"
              >
                この会話から再開
              </button>
            </div>
            <div
              v-for="(msg, i) in selected.messages"
              :key="i"
              class="flex"
              :class="msg.role === 'user' ? 'justify-end' : 'justify-start'"
            >
              <div
                class="max-w-[80%] px-3 py-2 rounded-lg text-sm"
                :class="msg.role === 'user'
                  ? 'bg-indigo-500 text-white'
                  : 'bg-gray-100 text-gray-900'"
              >
                {{ msg.content }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>
