export default defineNuxtConfig({
  devtools: { enabled: true },

  ssr: false,

  modules: ['@nuxtjs/tailwindcss'],

  runtimeConfig: {
    public: {
      wsUrl: process.env.NUXT_PUBLIC_WS_URL || 'ws://localhost:8080/ws',
      apiUrl: process.env.NUXT_PUBLIC_API_URL || 'http://localhost:8080',
    },
  },

  app: {
    head: {
      title: 'おためしちゃん',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      ],
    },
  },

  experimental: {
    appManifest: false,
  },

  compatibilityDate: '2024-12-01',
})
