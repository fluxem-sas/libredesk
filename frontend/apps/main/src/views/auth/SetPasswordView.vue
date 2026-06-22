<template>
  <AuthLayout>
    <div class="auth-card animate-auth-slide-in-up" id="set-password-container">
      <div class="auth-card__header">
        <div class="auth-card__logo">
          <img :src="logoUrl" alt="FluxemDesk" class="auth-card__logo-image" />
        </div>
        <h1 class="auth-card__title">{{ t('auth.setNewPassword') }}</h1>
        <p class="auth-card__subtitle">{{ t('auth.enterNewPasswordTwice') }}</p>
      </div>

      <form @submit.prevent="setPasswordAction" class="auth-card__form">
        <div class="auth-card__field">
          <Label for="password" class="auth-card__label">{{ t('auth.newPassword') }}</Label>
          <Input
            id="password"
            type="password"
            autocomplete="new-password"
            v-model="passwordForm.password"
            :class="{ 'auth-card__input--error': passwordHasError }"
            class="auth-card__input"
          />
        </div>

        <div class="auth-card__field">
          <Label for="confirmPassword" class="auth-card__label">{{ t('auth.confirmPassword') }}</Label>
          <Input
            id="confirmPassword"
            type="password"
            autocomplete="new-password"
            v-model="passwordForm.confirmPassword"
            :class="{ 'auth-card__input--error': confirmPasswordHasError }"
            class="auth-card__input"
          />
        </div>

        <Button class="auth-card__submit" :disabled="isLoading" type="submit">
          <span v-if="isLoading" class="flex items-center justify-center">
            <div
              class="w-5 h-5 border-2 border-primary-foreground/30 border-t-primary-foreground rounded-full animate-spin mr-3"
            ></div>
            {{ t('auth.settingPassword') }}
          </span>
          <span v-else>{{ t('auth.setNewPassword') }}</span>
        </Button>
      </form>

      <Error
        v-if="errorMessage"
        :errorMessage="errorMessage"
        :border="true"
        class="auth-card__error"
      />
    </div>
  </AuthLayout>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import api from '../../api'
import { useEmitter } from '../../composables/useEmitter'
import { EMITTER_EVENTS } from '../../constants/emitterEvents.js'
import { useTemporaryClass } from '../../composables/useTemporaryClass'
import { Button } from '@shared-ui/components/ui/button'
import { Error } from '@shared-ui/components/ui/error'
import { Input } from '@shared-ui/components/ui/input'
import { Label } from '@shared-ui/components/ui/label'
import { useI18n } from 'vue-i18n'
import AuthLayout from '@/layouts/auth/AuthLayout.vue'
import logoUrl from '/images/logo-fluxemdesk.svg?url'

const { t } = useI18n()
const errorMessage = ref('')
const isLoading = ref(false)
const router = useRouter()
const route = useRoute()
const emitter = useEmitter()
const passwordForm = ref({
  password: '',
  confirmPassword: '',
  token: ''
})

onMounted(() => {
  passwordForm.value.token = route.query.token
  if (!passwordForm.value.token) {
    router.push({ name: 'login' })
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: t('auth.invalidResetLink')
    })
  }
})

const validateForm = () => {
  if (!passwordForm.value.password) {
    errorMessage.value = t('auth.passwordRequired')
    useTemporaryClass('set-password-container', 'animate-shake')
    return false
  }
  if (passwordForm.value.password !== passwordForm.value.confirmPassword) {
    errorMessage.value = t('auth.passwordsDoNotMatch')
    useTemporaryClass('set-password-container', 'animate-shake')
    return false
  }
  return true
}

const setPasswordAction = async () => {
  if (!validateForm()) return

  errorMessage.value = ''
  isLoading.value = true

  try {
    await api.setPassword({
      token: passwordForm.value.token,
      password: passwordForm.value.password
    })
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      description: t('auth.passwordSetSuccess')
    })
    router.push({ name: 'login' })
  } catch (err) {
    errorMessage.value = handleHTTPError(err).message
    useTemporaryClass('set-password-container', 'animate-shake')
  } finally {
    isLoading.value = false
  }
}

const passwordHasError = computed(() => {
  return passwordForm.value.password !== '' && passwordForm.value.password.length < 8
})

const confirmPasswordHasError = computed(() => {
  return (
    passwordForm.value.confirmPassword !== '' &&
    passwordForm.value.password !== passwordForm.value.confirmPassword
  )
})
</script>
