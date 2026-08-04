export interface ProductConfig {
  id: string
  name: string
  price: number
  description: string
  image_url: string
}

export interface AppConfig {
  system_prompt: string
  llm_model: string
  tts_engine: string
  tts_voice: string
  tts_speaker_id: number
  tts_language: string
  tts_length_scale: number
  tts_noise_scale: number
  tts_noise_w: number
  voicevox_speaker_id: number
  voicevox_speed_scale: number
  stt_language: string
  stt_prompt: string
  products: ProductConfig[]
}

export interface VoicevoxSpeakerOption {
  id: number
  name: string
  style: string
}

export interface LlmModelOption {
  id: string
  label?: string
  preview?: boolean
}

export interface TtsSpeakerOption {
  id: number
  label: string
}

export interface TtsLanguageOption {
  id: string
  language_id?: number
  label: string
}

export interface TtsVoiceOption {
  id: string
  label: string
  num_speakers?: number
  num_languages?: number
  multi_speaker?: boolean
  speakers?: TtsSpeakerOption[]
  languages?: TtsLanguageOption[]
  note?: string
}

export function useAppSettings() {
  const runtimeConfig = useRuntimeConfig()
  const apiBase = computed(() => (runtimeConfig.public.apiUrl as string).replace(/\/$/, ''))

  const config = ref<AppConfig>({
    system_prompt: '',
    llm_model: '',
    tts_engine: 'piper',
    tts_voice: 'tsukuyomi_mb',
    tts_speaker_id: 0,
    tts_language: 'ja',
    tts_length_scale: 1.3,
    tts_noise_scale: 0.667,
    tts_noise_w: 0.8,
    voicevox_speaker_id: 3,
    voicevox_speed_scale: 1.0,
    stt_language: 'ja',
    stt_prompt: '',
    products: [],
  })

  const models = ref<LlmModelOption[]>([])
  const modelsLoading = ref(false)
  const modelsError = ref('')

  const ttsVoices = ref<TtsVoiceOption[]>([])
  const ttsVoicesLoading = ref(false)
  const ttsVoicesError = ref('')
  const ttsEngine = ref('')
  const voicevoxSpeakers = ref<VoicevoxSpeakerOption[]>([])

  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')
  const success = ref('')

  const selectedTtsVoice = computed(() =>
    ttsVoices.value.find((v) => v.id === config.value.tts_voice),
  )

  const ttsSpeakers = computed(() => selectedTtsVoice.value?.speakers ?? [])
  const ttsLanguages = computed(() => selectedTtsVoice.value?.languages ?? [])
  const isMultiSpeaker = computed(() => !!selectedTtsVoice.value?.multi_speaker)
  const hasTtsLanguages = computed(() => (selectedTtsVoice.value?.num_languages ?? 0) > 1)

  function syncSpeakerAndLanguage() {
    const voice = selectedTtsVoice.value
    if (!voice) return

    if (voice.speakers?.length) {
      const ok = voice.speakers.some((s) => s.id === config.value.tts_speaker_id)
      if (!ok) {
        config.value.tts_speaker_id = voice.speakers[0].id
      }
    } else if (!voice.multi_speaker) {
      config.value.tts_speaker_id = 0
    }

    if (voice.languages?.length) {
      const ok = voice.languages.some((l) => l.id === config.value.tts_language)
      if (!ok) {
        const ja = voice.languages.find((l) => l.id === 'ja')
        config.value.tts_language = ja?.id ?? voice.languages[0].id
      }
    }
  }

  watch(
    () => config.value.tts_voice,
    () => syncSpeakerAndLanguage(),
  )

  async function fetchModels() {
    modelsLoading.value = true
    modelsError.value = ''
    try {
      const res = await fetch(`${apiBase.value}/api/models`)
      if (!res.ok) {
        const body = await res.text()
        throw new Error(body || `モデル一覧の取得に失敗しました (${res.status})`)
      }
      const data = await res.json()
      models.value = Array.isArray(data.models) ? data.models : []

      if (
        config.value.llm_model &&
        models.value.length > 0 &&
        !models.value.some((m) => m.id === config.value.llm_model)
      ) {
        models.value = [
          { id: config.value.llm_model, label: config.value.llm_model },
          ...models.value,
        ]
      }
      if (!config.value.llm_model && models.value.length > 0) {
        config.value.llm_model = models.value[0].id
      }
    } catch (e) {
      modelsError.value = e instanceof Error ? e.message : 'モデル一覧の取得に失敗しました'
      models.value = []
    } finally {
      modelsLoading.value = false
    }
  }

  async function fetchTtsVoices() {
    ttsVoicesLoading.value = true
    ttsVoicesError.value = ''
    try {
      const engineParam = config.value.tts_engine ? `?engine=${config.value.tts_engine}` : ''
      const res = await fetch(`${apiBase.value}/api/tts/voices${engineParam}`)
      if (!res.ok) {
        throw new Error(`音声一覧の取得に失敗しました (${res.status})`)
      }
      const data = await res.json()
      ttsEngine.value = data.engine || 'piper'

      if (data.engine === 'voicevox') {
        voicevoxSpeakers.value = Array.isArray(data.speakers) ? data.speakers : []
        ttsVoices.value = []
      } else {
        ttsVoices.value = Array.isArray(data.voices) ? data.voices : []
        voicevoxSpeakers.value = []
        if (!config.value.tts_voice && data.default_voice) {
          config.value.tts_voice = data.default_voice
        }
        if (
          config.value.tts_voice &&
          ttsVoices.value.length > 0 &&
          !ttsVoices.value.some((v) => v.id === config.value.tts_voice)
        ) {
          ttsVoices.value = [
            {
              id: config.value.tts_voice,
              label: config.value.tts_voice,
              multi_speaker: false,
              speakers: [{ id: 0, label: 'デフォルト' }],
              languages: [],
            },
            ...ttsVoices.value,
          ]
        }
        syncSpeakerAndLanguage()
      }
    } catch (e) {
      ttsVoicesError.value = e instanceof Error ? e.message : '音声一覧の取得に失敗しました'
    } finally {
      ttsVoicesLoading.value = false
    }
  }

  async function fetchConfig() {
    loading.value = true
    error.value = ''
    success.value = ''
    try {
      const res = await fetch(`${apiBase.value}/api/config`)
      if (!res.ok) {
        throw new Error(`設定の取得に失敗しました (${res.status})`)
      }
      const data = await res.json()
      config.value = {
        ...config.value,
        ...data,
        tts_engine: data.tts_engine || 'piper',
        tts_speaker_id: typeof data.tts_speaker_id === 'number' ? data.tts_speaker_id : 0,
        tts_language: data.tts_language || 'ja',
        tts_voice: data.tts_voice || 'tsukuyomi_mb',
        voicevox_speaker_id: typeof data.voicevox_speaker_id === 'number' ? data.voicevox_speaker_id : 3,
        voicevox_speed_scale: typeof data.voicevox_speed_scale === 'number' ? data.voicevox_speed_scale : 1.0,
        products: Array.isArray(data.products) ? data.products : [],
      }
      await Promise.all([fetchModels(), fetchTtsVoices()])
    } catch (e) {
      error.value = e instanceof Error ? e.message : '設定の取得に失敗しました'
    } finally {
      loading.value = false
    }
  }

  async function saveConfig() {
    saving.value = true
    error.value = ''
    success.value = ''
    try {
      const res = await fetch(`${apiBase.value}/api/config`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config.value),
      })
      if (!res.ok) {
        throw new Error(`設定の保存に失敗しました (${res.status})`)
      }
      config.value = await res.json()
      success.value = '設定を保存しました。システムプロンプトの変更は次のセッション接続から反映されます。'
    } catch (e) {
      error.value = e instanceof Error ? e.message : '設定の保存に失敗しました'
    } finally {
      saving.value = false
    }
  }

  return {
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
  }
}
