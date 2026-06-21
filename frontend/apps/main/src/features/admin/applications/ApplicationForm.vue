<template>
  <form class="space-y-6 w-full">
    <FormField name="enabled" v-slot="{ value, handleChange }" v-if="!isNewForm">
      <FormItem>
        <FormControl>
          <div class="flex items-center space-x-2">
            <Checkbox :checked="value" @update:checked="handleChange" />
            <Label>{{ $t('globals.terms.active') }}</Label>
          </div>
        </FormControl>
        <FormMessage />
      </FormItem>
    </FormField>

    <div class="grid grid-cols-2 gap-4">
      <FormField v-slot="{ componentField }" name="name">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.name') }}</FormLabel>
          <FormControl>
            <Input type="text" :placeholder="t('application.namePlaceholder')" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="slug">
        <FormItem>
          <FormLabel>{{ $t('application.slug') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="kiaro" v-bind="componentField" />
          </FormControl>
          <FormDescription>{{ $t('application.slugHelp') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>
    </div>

    <FormField v-slot="{ componentField }" name="description">
      <FormItem>
        <FormLabel>{{ $t('globals.terms.description') }}</FormLabel>
        <FormControl>
          <Textarea rows="4" v-bind="componentField" />
        </FormControl>
        <FormMessage />
      </FormItem>
    </FormField>

    <div class="grid grid-cols-2 gap-4 items-start">
      <FormField v-slot="{ value, handleChange }" name="logo_url">
        <FormItem>
          <FormLabel>{{ $t('application.logo') }}</FormLabel>
          <FormControl>
            <div class="flex items-center gap-4">
              <div class="w-16 h-16 rounded-lg border bg-muted flex items-center justify-center overflow-hidden">
                <img
                  v-if="value"
                  :src="value"
                  alt="Application logo"
                  class="w-full h-full object-contain"
                />
                <ImageIcon v-else class="w-8 h-8 text-muted-foreground" />
              </div>
              <div class="flex flex-col gap-2">
                <input
                  ref="logoInput"
                  type="file"
                  accept="image/png,image/jpeg,image/jpg,image/svg+xml"
                  class="hidden"
                  @change="(e) => onLogoUpload(e, handleChange)"
                />
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  :isLoading="isUploadingLogo"
                  :disabled="isUploadingLogo"
                  @click="logoInput?.click()"
                >
                  {{ value ? $t('globals.messages.change') : $t('globals.messages.upload') }}
                </Button>
                <Button
                  v-if="value"
                  type="button"
                  variant="ghost"
                  size="sm"
                  @click="handleChange('')"
                >
                  {{ $t('globals.messages.remove') }}
                </Button>
              </div>
            </div>
          </FormControl>
          <FormDescription>{{ $t('application.logoHelp') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="identity_url">
        <FormItem>
          <FormLabel>{{ $t('application.identityURL') }}</FormLabel>
          <FormControl>
            <Input type="url" placeholder="https://your-login.example.com/api/v1/auth/me" v-bind="componentField" />
          </FormControl>
          <FormDescription>{{ $t('application.identityURLHelp') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>
    </div>

    <div v-if="!isNewForm" class="grid grid-cols-2 gap-4">
      <FormField v-slot="{ componentField }" name="gateway_app_id">
        <FormItem>
          <FormLabel>{{ $t('application.gatewayAppId') }}</FormLabel>
          <FormControl>
            <Input type="text" readonly v-bind="componentField" />
          </FormControl>
          <FormDescription>{{ $t('application.gatewayAppIdHelp') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="gateway_api_key">
        <FormItem>
          <FormLabel>{{ $t('application.gatewayAPIKey') }}</FormLabel>
          <FormControl>
            <div class="relative">
              <Input
                :type="showApiKey ? 'text' : 'password'"
                readonly
                v-bind="componentField"
                class="pr-10"
              />
              <Button
                type="button"
                variant="ghost"
                size="sm"
                class="absolute right-0 top-0 h-full px-3"
                @click="showApiKey = !showApiKey"
              >
                <EyeIcon v-if="!showApiKey" class="w-4 h-4" />
                <EyeOffIcon v-else class="w-4 h-4" />
              </Button>
            </div>
          </FormControl>
          <FormDescription>{{ $t('application.gatewayAPIKeyHelp') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>
    </div>

    <slot name="footer"></slot>
  </form>
</template>

<script setup>
import { ref } from 'vue'
import { Checkbox } from '@shared-ui/components/ui/checkbox'
import { Label } from '@shared-ui/components/ui/label'
import { Textarea } from '@shared-ui/components/ui/textarea'
import { Button } from '@shared-ui/components/ui/button'
import { useI18n } from 'vue-i18n'
import { ImageIcon, EyeIcon, EyeOffIcon } from 'lucide-vue-next'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  FormDescription
} from '@shared-ui/components/ui/form'
import { Input } from '@shared-ui/components/ui/input'
import api from '@/api'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import { useEmitter } from '@/composables/useEmitter'
import { handleHTTPError } from '@shared-ui/utils/http.js'

defineProps({
  form: {
    type: Object,
    required: true
  },
  isNewForm: {
    type: Boolean
  }
})

const { t } = useI18n()
const emitter = useEmitter()
const logoInput = ref(null)
const isUploadingLogo = ref(false)
const showApiKey = ref(false)

async function onLogoUpload(event, handleChange) {
  const file = event.target.files?.[0]
  if (!file) return

  isUploadingLogo.value = true
  try {
    const formData = new FormData()
    formData.append('files', file)
    const resp = await api.uploadMedia(formData)
    const media = resp.data.data
    if (media?.url) {
      handleChange(media.url)
    } else {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: t('application.logoUploadError')
      })
    }
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    isUploadingLogo.value = false
    event.target.value = ''
  }
}
</script>
