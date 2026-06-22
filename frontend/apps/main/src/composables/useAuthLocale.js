import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '@/api'

const AUTH_LOCALE_KEY = 'preferred-locale'
const SUPPORTED_LOCALES = ['es-ES', 'en-US']

export function useAuthLocale () {
  const { locale, setLocaleMessage } = useI18n()

  const isSupported = (code) => SUPPORTED_LOCALES.includes(code)

  const applyLocale = async (code) => {
    if (!isSupported(code) || locale.value === code) return

    const { data } = await api.getLanguage(code)
    setLocaleMessage(code, data)
    locale.value = code
    localStorage.setItem(AUTH_LOCALE_KEY, code)
    document.documentElement.lang = code.split('-')[0]
  }

  onMounted(async () => {
    const saved = localStorage.getItem(AUTH_LOCALE_KEY)
    if (saved && isSupported(saved) && saved !== locale.value) {
      await applyLocale(saved)
    }
  })

  return {
    locale,
    supportedLocales: SUPPORTED_LOCALES,
    applyLocale,
    localeLabel: (code) => (code === 'es-ES' ? 'ES' : 'EN')
  }
}
