<template>
  <AuthLayout>
    <div class="auth-card animate-auth-slide-in-up" id="reset-password-container">
      <div class="auth-card__header">
        <div class="auth-card__logo">
          <img :src="logoUrl" alt="FluxemDesk" class="auth-card__logo-image" />
        </div>
        <h1 class="auth-card__title">{{ t('auth.resetPassword') }}</h1>
        <p class="auth-card__subtitle">{{ t('auth.enterEmailForReset') }}</p>
      </div>

      <form @submit.prevent="requestResetAction" class="auth-card__form">
        <div class="auth-card__field">
          <Label for="email" class="auth-card__label">{{ t('globals.terms.email') }}</Label>
          <Input
            id="email"
            type="email"
            v-model.trim="resetForm.email"
            :class="{ 'auth-card__input--error': emailHasError }"
            class="auth-card__input"
          />
        </div>

        <Button class="auth-card__submit" :disabled="isLoading" type="submit">
          <span v-if="isLoading" class="flex items-center justify-center">
            <div
              class="w-5 h-5 border-2 border-primary-foreground/30 border-t-primary-foreground rounded-full animate-spin mr-3"
            ></div>
            {{ t('globals.messages.sending') }}
          </span>
          <span v-else>{{ t('auth.sendResetLink') }}</span>
        </Button>
      </form>

      <Error
        v-if="errorMessage"
        :errorMessage="errorMessage"
        :border="true"
        class="auth-card__error"
      />

      <div class="auth-card__options" style="justify-content: center; margin-top: 1.5rem;">
        <router-link to="/" class="auth-card__link">
          {{ t('auth.backToLogin') }}
        </router-link>
      </div>
    </div>
  </AuthLayout>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import api from '../../api'
import { validateEmail } from '@shared-ui/utils/string'
import { useTemporaryClass } from '../../composables/useTemporaryClass'
import { Button } from '@shared-ui/components/ui/button'
import { Error } from '@shared-ui/components/ui/error'
import { Input } from '@shared-ui/components/ui/input'
import { EMITTER_EVENTS } from '../../constants/emitterEvents.js'
import { useEmitter } from '../../composables/useEmitter'
import { Label } from '@shared-ui/components/ui/label'
import { useI18n } from 'vue-i18n'
import AuthLayout from '@/layouts/auth/AuthLayout.vue'
import logoUrl from '/images/logo-fluxemdesk.svg?url'

const errorMessage = ref('')
const { t } = useI18n()
const isLoading = ref(false)
const emitter = useEmitter()
const router = useRouter()
const resetForm = ref({
  email: ''
})

const validateForm = () => {
  if (!validateEmail(resetForm.value.email)) {
    errorMessage.value = t('validation.invalidEmail')
    useTemporaryClass('reset-password-container', 'animate-shake')
    return false
  }
  return true
}

const requestResetAction = async () => {
  if (!validateForm()) return

  errorMessage.value = ''
  isLoading.value = true

  try {
    await api.resetPassword({
      email: resetForm.value.email
    })
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      description: t('auth.checkEmailForReset')
    })
    router.push({ name: 'login' })
  } catch (err) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(err).message
    })
    errorMessage.value = handleHTTPError(err).message
    useTemporaryClass('reset-password-container', 'animate-shake')
  } finally {
    isLoading.value = false
  }
}

const emailHasError = computed(() => {
  return !validateEmail(resetForm.value.email) && resetForm.value.email !== ''
})
</script>
