<script setup lang="ts">
import type { SessionConfig } from '~/types/protocol'

const route = useRoute()
const session = useSession()

const textInput = ref('')
const inputMode = ref<'text' | 'mic'>('text')
const sessionConfig: SessionConfig = {
  recommendation_mode: 'ai_driven',
  post_recommendation_behavior: 'return_to_conversation',
}

const router = useRouter()
const isConnected = computed(() => session.connectionState.value === 'connected')
const canSend = computed(() => isConnected.value && textInput.value.trim().length > 0)
const isRestoredSession = computed(() => !!route.query.restore)

function handleConnect() {
  if (isConnected.value) {
    session.disconnect()
    if (inputMode.value === 'mic') {
      session.stopCapture()
    }
  } else {
    session.resumeAudioContext()
    const restoreId = route.query.restore as string | undefined
    session.connect(sessionConfig, restoreId)
  }
}

function startNewSession() {
  if (isConnected.value) {
    session.disconnect()
    if (inputMode.value === 'mic') {
      session.stopCapture()
    }
  }
  router.replace({ path: '/assistant', query: {} })
  nextTick(() => {
    session.resumeAudioContext()
    session.connect(sessionConfig)
  })
}

onMounted(() => {
  if (route.query.restore) {
    session.resumeAudioContext()
    session.connect(sessionConfig, route.query.restore as string)
  }
})

function handleSend() {
  if (!canSend.value) return
  session.sendText(textInput.value.trim())
  textInput.value = ''
}

function handleKeyDown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    handleSend()
  }
}

function switchInputMode(mode: 'text' | 'mic') {
  if (inputMode.value === mode) return
  if (inputMode.value === 'mic') {
    session.stopCapture()
  }
  inputMode.value = mode
  if (mode === 'mic' && isConnected.value) {
    session.startCapture()
  }
}

function handleVolumeChange(e: Event) {
  const val = parseFloat((e.target as HTMLInputElement).value)
  session.setPlaybackVolume(val)
}

const connectButtonLabel = computed(() => {
  switch (session.connectionState.value) {
    case 'connecting': return '接続中...'
    case 'connected': return '切断'
    default: return '接続'
  }
})

const connectButtonClass = computed(() => {
  if (session.connectionState.value === 'connected') {
    return 'bg-red-500 hover:bg-red-600'
  }
  if (session.connectionState.value === 'connecting') {
    return 'bg-gray-400 cursor-not-allowed'
  }
  return 'bg-indigo-500 hover:bg-indigo-600'
})
</script>

<template>
  <div class="h-screen flex flex-col bg-gray-50">
    <!-- Error banner -->
    <ErrorBanner
      v-if="session.errorMessage.value"
      :message="session.errorMessage.value"
      :type="session.errorType.value"
      @close="session.clearError()"
    />

    <!-- Header -->
    <header class="bg-white border-b border-gray-200 px-4 py-3 shrink-0">
      <div class="max-w-4xl mx-auto flex items-center justify-between">
        <div class="flex items-center gap-3">
          <h1 class="text-lg font-bold text-gray-900">おためしちゃん</h1>
          <StatusIndicator :status="session.state.value" :input-mode="inputMode" :connection-state="session.connectionState.value" />
          <span v-if="isRestoredSession" class="text-xs text-amber-600 bg-amber-50 px-2 py-0.5 rounded">復元</span>
          <button
            v-if="isRestoredSession"
            class="px-2.5 py-1 text-xs font-medium text-white bg-indigo-500 hover:bg-indigo-600 rounded-lg transition-colors"
            @click="startNewSession"
          >
            新規会話
          </button>
        </div>
        <div class="flex items-center gap-3">
          <!-- Volume control -->
          <div class="flex items-center gap-2">
            <button
              class="p-1.5 rounded-lg transition-colors"
              :class="session.isMuted.value ? 'text-red-500 hover:bg-red-50' : 'text-gray-500 hover:bg-gray-100'"
              @click="session.toggleMute()"
              :title="session.isMuted.value ? '音声ON' : '音声OFF'"
            >
              <svg v-if="!session.isMuted.value" xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>
                <path d="M19.07 4.93a10 10 0 0 1 0 14.14"/>
                <path d="M15.54 8.46a5 5 0 0 1 0 7.07"/>
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>
                <line x1="23" y1="9" x2="17" y2="15"/>
                <line x1="17" y1="9" x2="23" y2="15"/>
              </svg>
            </button>
            <input
              type="range"
              min="0"
              max="1"
              step="0.05"
              :value="session.playbackVolume.value"
              class="w-20 h-1.5 accent-indigo-500 cursor-pointer"
              :disabled="session.isMuted.value"
              @input="handleVolumeChange"
            />
          </div>
          <NuxtLink
            to="/history"
            class="px-3 py-2 text-sm font-medium text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors"
          >
            履歴
          </NuxtLink>
          <NuxtLink
            to="/settings"
            class="px-3 py-2 text-sm font-medium text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors"
          >
            設定
          </NuxtLink>
          <button
            class="px-4 py-2 text-white text-sm font-medium rounded-lg transition-colors"
            :class="connectButtonClass"
            :disabled="session.connectionState.value === 'connecting'"
            @click="handleConnect"
          >
            {{ connectButtonLabel }}
          </button>
        </div>
      </div>
    </header>


    <!-- Conversation timeline (main scrollable area) -->
    <div class="flex-1 min-h-0 overflow-hidden max-w-4xl mx-auto w-full flex flex-col">
      <ConversationTimeline :messages="(session.messages.value as any)" />
    </div>

    <!-- Bottom controls -->
    <div class="bg-white border-t border-gray-200 px-4 py-3 shrink-0">
      <div class="max-w-4xl mx-auto space-y-3">
        <!-- Mic mode -->
        <template v-if="inputMode === 'mic'">
          <VolumeMeter
            v-if="session.isCapturing.value"
            :volume="session.micVolume.value"
          />
          <div
            v-if="session.isSpeaking.value"
            class="text-center text-sm text-indigo-500 font-medium px-4"
          >
            音声を検出中...
          </div>
          <div class="flex items-center gap-3">
            <!-- Input mode toggle -->
            <div class="flex items-center bg-gray-100 rounded-lg p-0.5">
              <button
                class="px-3 py-1.5 text-xs font-medium rounded-md transition-colors"
                :class="'text-gray-500 hover:text-gray-700'"
                @click="switchInputMode('text')"
              >
                テキスト
              </button>
              <button
                class="px-3 py-1.5 text-xs font-medium rounded-md transition-colors bg-white text-gray-900 shadow-sm"
              >
                マイク
              </button>
            </div>
            <!-- Mic button -->
            <div class="flex-1 flex flex-col items-center gap-1">
              <button
                :disabled="!isConnected"
                class="w-14 h-14 rounded-full flex items-center justify-center transition-colors"
                :class="session.isCapturing.value
                  ? 'bg-red-500 text-white hover:bg-red-600 animate-pulse'
                  : isConnected
                    ? 'bg-indigo-500 text-white hover:bg-indigo-600'
                    : 'bg-gray-300 text-gray-400 cursor-not-allowed'"
                @click="session.isCapturing.value ? session.stopCapture() : session.startCapture()"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z"/>
                  <path d="M19 10v2a7 7 0 0 1-14 0v-2"/>
                  <line x1="12" y1="19" x2="12" y2="23"/>
                  <line x1="8" y1="23" x2="16" y2="23"/>
                </svg>
              </button>
              <span class="text-xs text-gray-500">
                {{ session.isCapturing.value ? '音声認識中' : isConnected ? '音声入力開始' : '接続してください' }}
              </span>
            </div>
            <div class="w-[88px]"></div>
          </div>
        </template>

        <!-- Text mode -->
        <div v-else class="flex items-center gap-2">
          <!-- Input mode toggle -->
          <div class="flex items-center bg-gray-100 rounded-lg p-0.5 shrink-0">
            <button
              class="px-3 py-1.5 text-xs font-medium rounded-md transition-colors bg-white text-gray-900 shadow-sm"
            >
              テキスト
            </button>
            <button
              class="px-3 py-1.5 text-xs font-medium rounded-md transition-colors text-gray-500 hover:text-gray-700"
              @click="switchInputMode('mic')"
            >
              マイク
            </button>
          </div>
          <input
            v-model="textInput"
            type="text"
            :placeholder="isConnected ? 'メッセージを入力...' : '接続してください'"
            :disabled="!isConnected"
            class="flex-1 border border-gray-300 rounded-lg px-4 py-2 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 disabled:bg-gray-100 disabled:text-gray-400"
            @keydown="handleKeyDown"
          />
          <button
            :disabled="!canSend"
            class="px-5 py-2 bg-indigo-500 text-white text-sm font-medium rounded-lg hover:bg-indigo-600 transition-colors disabled:bg-gray-300 disabled:cursor-not-allowed"
            @click="handleSend"
          >
            送信
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
