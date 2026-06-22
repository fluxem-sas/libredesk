import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { initI18n } from './i18n'
import { useAppSettingsStore } from './stores/appSettings'
import router from './router'
import mitt from 'mitt'
import api from './api'
import '@shared-ui/assets/styles/main.scss'
import '@shared-ui/utils/string.js'
import Root from './Root.vue'

const setFavicon = (url) => {
  let link = document.createElement("link")
  link.rel = "icon"
  document.head.appendChild(link)
  link.href = url
}

const AUTH_LOCALE_KEY = 'preferred-locale'
const AUTH_SUPPORTED_LOCALES = ['es-ES', 'en-US']

async function initApp () {
  const config = (await api.getConfig()).data.data
  const emitter = mitt()
  const serverLang = config['app.lang'] || 'en-US'
  const savedLocale = localStorage.getItem(AUTH_LOCALE_KEY)
  const lang =
    savedLocale && AUTH_SUPPORTED_LOCALES.includes(savedLocale) ? savedLocale : serverLang

  const otherLang = lang === 'es-ES' ? 'en-US' : 'es-ES'
  const [langMessages, otherLangMessages] = await Promise.all([
    api.getLanguage(lang),
    api.getLanguage(otherLang)
  ])

  // Set favicon.
  if (config['app.favicon_url'])
    setFavicon(config['app.favicon_url'])

  // Initialize i18n.
  const i18nConfig = {
    legacy: false,
    locale: lang,
    fallbackLocale: 'en-US',
    messages: {
      [lang]: langMessages.data,
      [otherLang]: otherLangMessages.data
    }
  }

  const i18n = initI18n(i18nConfig)
  const app = createApp(Root)
  const pinia = createPinia()
  app.use(pinia)

  // Fetch and store app settings in store (after pinia is initialized)
  const settingsStore = useAppSettingsStore()

  // Store the public config in the store
  settingsStore.setPublicConfig(config)

  try {
    await settingsStore.fetchSettings('general')
  } catch (error) {
    // Pass
  }

  // Add emitter to global properties.
  app.config.globalProperties.emitter = emitter

  app.use(router)
  app.use(i18n)
  app.mount('#app')
}

initApp().catch((error) => {
  console.error('Error initializing app: ', error)
})
