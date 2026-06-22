<template>
  <div class="auth-preferences">
    <div class="auth-preferences__group">
      <button
        v-for="code in supportedLocales"
        :key="code"
        type="button"
        class="auth-preferences__lang"
        :class="{ 'auth-preferences__lang--active': locale === code }"
        :aria-label="code === 'es-ES' ? 'Español' : 'English'"
        @click="applyLocale(code)"
      >
        {{ localeLabel(code) }}
      </button>
    </div>

    <div class="auth-preferences__divider" aria-hidden="true"></div>

    <div class="auth-preferences__group">
      <span class="auth-preferences__label">{{ t('navigation.darkMode') }}</span>
      <Switch
        :checked="mode === 'dark'"
        @update:checked="(val) => (mode = val ? 'dark' : 'light')"
      />
    </div>
  </div>
</template>

<script setup>
import { useColorMode } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { Switch } from '@shared-ui/components/ui/switch'
import { useAuthLocale } from '@/composables/useAuthLocale'

const { t } = useI18n()
const mode = useColorMode()
const { locale, supportedLocales, applyLocale, localeLabel } = useAuthLocale()
</script>
