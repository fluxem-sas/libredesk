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

    <div class="grid grid-cols-2 gap-4">
      <FormField v-slot="{ componentField }" name="logo_url">
        <FormItem>
          <FormLabel>{{ $t('application.logoURL') }}</FormLabel>
          <FormControl>
            <Input type="url" placeholder="https://example.com/logo.svg" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="identity_url">
        <FormItem>
          <FormLabel>{{ $t('application.identityURL') }}</FormLabel>
          <FormControl>
            <Input type="url" placeholder="https://login.example.com/internal/support/users/{{user_id}}" v-bind="componentField" />
          </FormControl>
          <FormDescription>{{ $t('application.identityURLHelp') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>
    </div>

    <div class="grid grid-cols-2 gap-4">
      <FormField v-slot="{ componentField }" name="gateway_app_id">
        <FormItem>
          <FormLabel>{{ $t('application.gatewayAppId') }}</FormLabel>
          <FormControl>
            <Input type="text" :readonly="!isNewForm" placeholder="kiaro-gateway" v-bind="componentField" />
          </FormControl>
          <FormDescription>{{ $t('application.gatewayAppIdHelp') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="gateway_api_key">
        <FormItem>
          <FormLabel>{{ $t('application.gatewayAPIKey') }}</FormLabel>
          <FormControl>
            <Input type="password" v-bind="componentField" />
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
import { Checkbox } from '@shared-ui/components/ui/checkbox'
import { Label } from '@shared-ui/components/ui/label'
import { Textarea } from '@shared-ui/components/ui/textarea'
import { useI18n } from 'vue-i18n'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  FormDescription
} from '@shared-ui/components/ui/form'
import { Input } from '@shared-ui/components/ui/input'

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
</script>
