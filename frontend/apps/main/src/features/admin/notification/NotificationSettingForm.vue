<template>
  <form @submit.prevent="onSmtpSubmit" class="space-y-6">
    <FormField name="enabled" v-slot="{ value, handleChange }">
      <FormItem>
        <FormControl>
          <div class="flex items-center space-x-2">
            <Checkbox :checked="value" @update:checked="handleChange" />
            <Label>{{ $t('globals.terms.enabled') }}</Label>
          </div>
        </FormControl>
        <FormMessage />
      </FormItem>
    </FormField>

    <div class="space-y-4">
      <div class="space-y-1">
        <h3 class="text-base font-semibold">Proveedor de envío</h3>
        <p class="text-sm text-muted-foreground">
          Resend usa un flujo guiado. SMTP manual solo si realmente vas a usar otro proveedor.
        </p>
      </div>

      <div class="flex flex-wrap gap-2">
        <MenuCard
          class="shrink-0 w-92 max-w-none"
          title="Resend"
          subTitle="Configuración guiada"
          icon="/images/resend-icon-black.svg"
          iconDark="/images/resend-icon-white.svg"
          :badge="providerMode === 'resend' ? 'Activo' : ''"
          @click="selectResendMode"
        />
        <MenuCard
          class="shrink-0 w-92 max-w-none"
          title="SMTP manual"
          subTitle="Solo para otros proveedores"
          :icon="Mail"
          :badge="providerMode === 'smtp' ? 'Activo' : ''"
          @click="selectSmtpMode"
        />
      </div>
    </div>

    <div v-if="providerMode === 'resend'" class="space-y-4">
      <div class="rounded-lg border border-border bg-background/60 p-4 space-y-4">
        <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div class="space-y-1">
            <h3 class="font-semibold">Resend</h3>
            <p class="text-sm text-muted-foreground">
              No necesitas llenar host, usuario ni autenticación manual. Eso se completa automáticamente.
            </p>
          </div>
          <Button type="button" variant="outline" size="sm" @click="applyResendDefaults()">
            Reaplicar configuración
          </Button>
        </div>

        <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <div class="rounded-md border border-border/60 bg-muted/30 p-3">
            <p class="text-xs uppercase tracking-wide text-muted-foreground">Host</p>
            <p class="text-sm font-medium">smtp.resend.com</p>
          </div>
          <div class="rounded-md border border-border/60 bg-muted/30 p-3">
            <p class="text-xs uppercase tracking-wide text-muted-foreground">Usuario</p>
            <p class="text-sm font-medium">resend</p>
          </div>
          <div class="rounded-md border border-border/60 bg-muted/30 p-3">
            <p class="text-xs uppercase tracking-wide text-muted-foreground">Auth</p>
            <p class="text-sm font-medium">plain</p>
          </div>
          <div class="rounded-md border border-border/60 bg-muted/30 p-3">
            <p class="text-xs uppercase tracking-wide text-muted-foreground">TLS</p>
            <p class="text-sm font-medium">465 = SSL/TLS · 587 = STARTTLS</p>
          </div>
        </div>
      </div>

      <div class="grid gap-6 md:grid-cols-2">
        <FormField v-slot="{ componentField }" name="password">
          <FormItem>
            <FormLabel>API Key de Resend</FormLabel>
            <FormControl>
              <Input type="password" placeholder="re_xxxxxxxxx" v-bind="componentField" />
            </FormControl>
            <FormDescription>
              Usa tu API key de Resend.
            </FormDescription>
            <FormMessage />
          </FormItem>
        </FormField>

        <FormField v-slot="{ componentField }" name="email_address">
          <FormItem>
            <FormLabel>Correo remitente verificado</FormLabel>
            <FormControl>
              <Input
                type="text"
                placeholder="notifications@tu-dominio.com"
                v-bind="componentField"
              />
            </FormControl>
            <FormDescription>
              Debe existir y estar verificado en Resend.
            </FormDescription>
            <FormMessage />
          </FormItem>
        </FormField>
      </div>

      <div class="space-y-3">
        <FormField v-slot="{ componentField }" name="port">
          <FormItem>
            <FormLabel>Puerto SMTP</FormLabel>
            <FormControl>
              <Input type="number" placeholder="587" v-bind="componentField" />
            </FormControl>
            <FormDescription>
              Usa 465 para SSL/TLS o 587 para STARTTLS.
            </FormDescription>
            <FormMessage />
          </FormItem>
        </FormField>

        <div class="grid gap-2 md:grid-cols-2">
          <Button
            type="button"
            :variant="Number(smtpForm.values.port) === 465 ? 'default' : 'outline'"
            @click="setResendTlsProfile(465)"
          >
            Puerto 465 · SSL/TLS
          </Button>
          <Button
            type="button"
            :variant="Number(smtpForm.values.port) === 587 ? 'default' : 'outline'"
            @click="setResendTlsProfile(587)"
          >
            Puerto 587 · STARTTLS
          </Button>
        </div>
      </div>

      <div class="rounded-lg border border-border/60 bg-muted/20 p-4 space-y-3">
        <button
          type="button"
          class="text-sm font-medium text-primary"
          @click="showAdvancedSettings = !showAdvancedSettings"
        >
          {{ showAdvancedSettings ? $t('globals.messages.showLess') : $t('globals.messages.showMore') }} ajustes técnicos opcionales
        </button>

        <div v-if="showAdvancedSettings" class="space-y-6">
          <div class="grid gap-6 md:grid-cols-2">
            <FormField v-slot="{ componentField }" name="max_conns">
              <FormItem>
                <FormLabel>{{ $t('admin.inbox.maxConnections') }}</FormLabel>
                <FormControl>
                  <Input type="number" placeholder="5" v-bind="componentField" />
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>

            <FormField v-slot="{ componentField }" name="max_msg_retries">
              <FormItem>
                <FormLabel>{{ $t('admin.inbox.maxRetries') }}</FormLabel>
                <FormControl>
                  <Input type="number" placeholder="3" v-bind="componentField" />
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>
          </div>

          <div class="grid gap-6 md:grid-cols-2">
            <FormField v-slot="{ componentField }" name="idle_timeout">
              <FormItem>
                <FormLabel>{{ $t('admin.inbox.idleTimeout') }}</FormLabel>
                <FormControl>
                  <Input type="text" placeholder="25s" v-bind="componentField" />
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>

            <FormField v-slot="{ componentField }" name="wait_timeout">
              <FormItem>
                <FormLabel>{{ $t('admin.inbox.waitTimeout') }}</FormLabel>
                <FormControl>
                  <Input type="text" placeholder="60s" v-bind="componentField" />
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>
          </div>

          <FormField v-slot="{ componentField }" name="hello_hostname">
            <FormItem>
              <FormLabel>{{ $t('admin.inbox.heloHostname') }}</FormLabel>
              <FormControl>
                <Input type="text" placeholder="" v-bind="componentField" />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>
        </div>
      </div>
    </div>

    <template v-else>
      <div class="grid gap-6 md:grid-cols-2">
        <FormField v-slot="{ componentField }" name="host">
          <FormItem>
            <FormLabel>{{ $t('globals.terms.smtpHost') }}</FormLabel>
            <FormControl>
              <Input type="text" placeholder="smtp.gmail.com" v-bind="componentField" />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>

        <FormField v-slot="{ componentField }" name="port">
          <FormItem>
            <FormLabel>{{ $t('globals.terms.smtpPort') }}</FormLabel>
            <FormControl>
              <Input type="number" placeholder="587" v-bind="componentField" />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>
      </div>

      <div class="grid gap-6 md:grid-cols-2">
        <FormField v-slot="{ componentField }" name="username">
          <FormItem>
            <FormLabel>{{ $t('globals.terms.username') }}</FormLabel>
            <FormControl>
              <Input type="text" placeholder="admin@yourcompany.com" v-bind="componentField" />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>

        <FormField v-slot="{ componentField }" name="password">
          <FormItem>
            <FormLabel>{{ $t('globals.terms.password') }}</FormLabel>
            <FormControl>
              <Input type="password" placeholder="" v-bind="componentField" />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>
      </div>

      <div class="grid gap-6 md:grid-cols-2">
        <FormField v-slot="{ componentField }" name="auth_protocol">
          <FormItem>
            <FormLabel>{{ $t('admin.inbox.authProtocol') }}</FormLabel>
            <FormControl>
              <Select v-bind="componentField" v-model="componentField.modelValue">
                <SelectTrigger>
                  <SelectValue :placeholder="t('admin.inbox.authProtocol.description')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value="plain">Plain</SelectItem>
                    <SelectItem value="login">Login</SelectItem>
                    <SelectItem value="cram">CRAM-MD5</SelectItem>
                    <SelectItem value="none">None</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>

        <FormField v-slot="{ componentField }" name="tls_type">
          <FormItem>
            <FormLabel>TLS</FormLabel>
            <FormControl>
              <Select v-bind="componentField" v-model="componentField.modelValue">
                <SelectTrigger>
                  <SelectValue :placeholder="t('globals.messages.selectTLS')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value="none">Off</SelectItem>
                    <SelectItem value="tls">SSL/TLS</SelectItem>
                    <SelectItem value="starttls">STARTTLS</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>
      </div>

      <div class="grid gap-6 md:grid-cols-2">
        <FormField v-slot="{ componentField }" name="email_address">
          <FormItem>
            <FormLabel>{{ $t('globals.terms.fromEmailAddress') }}</FormLabel>
            <FormControl>
              <Input
                type="text"
                :placeholder="t('admin.inbox.fromEmailAddress.placeholder')"
                v-bind="componentField"
              />
            </FormControl>
            <FormMessage />
            <FormDescription> {{ $t('admin.inbox.fromEmailAddress.description') }}</FormDescription>
          </FormItem>
        </FormField>

        <FormField v-slot="{ componentField }" name="hello_hostname">
          <FormItem>
            <FormLabel>{{ $t('admin.inbox.heloHostname') }}</FormLabel>
            <FormControl>
              <Input type="text" placeholder="" v-bind="componentField" />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>
      </div>

      <div class="grid gap-6 md:grid-cols-2">
        <FormField v-slot="{ componentField }" name="max_conns">
          <FormItem>
            <FormLabel>{{ $t('admin.inbox.maxConnections') }}</FormLabel>
            <FormControl>
              <Input type="number" placeholder="2" v-bind="componentField" />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>

        <FormField v-slot="{ componentField }" name="max_msg_retries">
          <FormItem>
            <FormLabel>{{ $t('admin.inbox.maxRetries') }}</FormLabel>
            <FormControl>
              <Input type="number" placeholder="3" v-bind="componentField" />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>
      </div>

      <div class="grid gap-6 md:grid-cols-2">
        <FormField v-slot="{ componentField }" name="idle_timeout">
          <FormItem>
            <FormLabel>{{ $t('admin.inbox.idleTimeout') }}</FormLabel>
            <FormControl>
              <Input type="text" placeholder="15s" v-bind="componentField" />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>

        <FormField v-slot="{ componentField }" name="wait_timeout">
          <FormItem>
            <FormLabel>{{ $t('admin.inbox.waitTimeout') }}</FormLabel>
            <FormControl>
              <Input type="text" placeholder="5s" v-bind="componentField" />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>
      </div>

      <FormField v-slot="{ componentField, handleChange }" name="tls_skip_verify">
        <FormItem>
          <SwitchField
            :title="$t('admin.inbox.skipTLSVerification')"
            :description="$t('admin.inbox.skipTLSVerification.description')"
            :checked="componentField.modelValue"
            @update:checked="handleChange"
          />
        </FormItem>
      </FormField>
    </template>

    <Button type="button" :isLoading="isLoading" @click="onSmtpSubmit"> {{ submitLabel }} </Button>
  </form>
</template>

<script setup>
import { watch, ref, computed } from 'vue'
import { Button } from '@shared-ui/components/ui/button/index.js'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { createFormSchema } from './formSchema.js'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  FormDescription
} from '@shared-ui/components/ui/form/index.js'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@shared-ui/components/ui/select/index.js'
import { Checkbox } from '@shared-ui/components/ui/checkbox/index.js'
import SwitchField from '@shared-ui/components/SwitchField.vue'
import { Label } from '@shared-ui/components/ui/label/index.js'
import { Input } from '@shared-ui/components/ui/input/index.js'
import { useI18n } from 'vue-i18n'
import { Mail } from 'lucide-vue-next'
import MenuCard from '@main/components/layout/MenuCard.vue'

const isLoading = ref(false)
const providerMode = ref('resend')
const showAdvancedSettings = ref(false)
const { t } = useI18n()
const props = defineProps({
  initialValues: {
    type: Object,
    required: false
  },
  submitForm: {
    type: Function,
    required: true
  },
  submitLabel: {
    type: String,
    required: false,
    default: () => ''
  }
})

const defaultValues = {
  enabled: false,
  username: 'resend',
  host: 'smtp.resend.com',
  port: 465,
  password: '',
  max_conns: 5,
  idle_timeout: '25s',
  wait_timeout: '60s',
  auth_protocol: 'plain',
  email_address: '',
  max_msg_retries: 3,
  hello_hostname: '',
  tls_type: 'tls',
  tls_skip_verify: false
}

const submitLabel = computed(() => {
  if (props.submitLabel) {
    return props.submitLabel
  }
  return t('globals.messages.save')
})

const smtpForm = useForm({
  validationSchema: toTypedSchema(createFormSchema(t)),
  initialValues: defaultValues
})

const isResendConfig = (values = {}) => {
  return values.host === 'smtp.resend.com' || values.username === 'resend'
}

const mergeWithDefaults = (values = {}) => ({
  ...defaultValues,
  ...values
})

const applyResendDefaults = () => {
  const currentPort = Number(smtpForm.values.port) === 587 ? 587 : 465

  smtpForm.setFieldValue('host', 'smtp.resend.com')
  smtpForm.setFieldValue('username', 'resend')
  smtpForm.setFieldValue('auth_protocol', 'plain')
  smtpForm.setFieldValue('port', currentPort)
  smtpForm.setFieldValue('tls_type', currentPort === 587 ? 'starttls' : 'tls')
  smtpForm.setFieldValue('max_conns', Number(smtpForm.values.max_conns) || 5)
  smtpForm.setFieldValue('max_msg_retries', Number(smtpForm.values.max_msg_retries) || 3)
  smtpForm.setFieldValue('idle_timeout', smtpForm.values.idle_timeout || '25s')
  smtpForm.setFieldValue('wait_timeout', smtpForm.values.wait_timeout || '60s')
  smtpForm.setFieldValue('hello_hostname', smtpForm.values.hello_hostname || '')
  smtpForm.setFieldValue('tls_skip_verify', false)
}

const selectResendMode = () => {
  providerMode.value = 'resend'
  applyResendDefaults()
}

const selectSmtpMode = () => {
  providerMode.value = 'smtp'
}

const setResendTlsProfile = (port) => {
  smtpForm.setFieldValue('port', port)
  smtpForm.setFieldValue('tls_type', port === 587 ? 'starttls' : 'tls')
}

const validateResendMode = () => {
  let valid = true
  smtpForm.setFieldError('password', undefined)
  smtpForm.setFieldError('email_address', undefined)
  smtpForm.setFieldError('port', undefined)

  if (!smtpForm.values.password) {
    smtpForm.setFieldError('password', t('globals.messages.required'))
    valid = false
  }

  if (!smtpForm.values.email_address) {
    smtpForm.setFieldError('email_address', t('globals.messages.required'))
    valid = false
  }

  const port = Number(smtpForm.values.port)
  if (!port || port < 1 || port > 65535) {
    smtpForm.setFieldError('port', t('validation.invalidPortValue'))
    valid = false
  }

  return valid
}

const normalizeResendPayload = () => {
  const port = Number(smtpForm.values.port) || 587

  return {
    ...smtpForm.values,
    host: 'smtp.resend.com',
    username: 'resend',
    auth_protocol: 'plain',
    tls_skip_verify: false,
    port,
    tls_type: port === 587 ? 'starttls' : 'tls',
    max_conns: Number(smtpForm.values.max_conns) || 5,
    max_msg_retries: Number(smtpForm.values.max_msg_retries) || 3,
    idle_timeout: smtpForm.values.idle_timeout || '25s',
    wait_timeout: smtpForm.values.wait_timeout || '60s',
    hello_hostname: smtpForm.values.hello_hostname || ''
  }
}

const normalizeSMTPPayload = () => ({
  ...smtpForm.values,
  port: Number(smtpForm.values.port),
  max_conns: Number(smtpForm.values.max_conns),
  max_msg_retries: Number(smtpForm.values.max_msg_retries)
})

const onSmtpSubmit = async () => {
  if (isLoading.value) return

  isLoading.value = true
  try {
    if (providerMode.value === 'resend') {
      if (!validateResendMode()) {
        return
      }
      await props.submitForm(normalizeResendPayload())
      return
    }

    const result = await smtpForm.validate()
    if (!result.valid) {
      return
    }

    await props.submitForm(normalizeSMTPPayload())
  } finally {
    isLoading.value = false
  }
}

watch(
  () => props.initialValues,
  (newValues) => {
    const mergedValues = mergeWithDefaults(newValues)
    smtpForm.setValues(mergedValues)
    providerMode.value = isResendConfig(mergedValues) ? 'resend' : 'smtp'
    showAdvancedSettings.value = false
    if (providerMode.value === 'resend') {
      applyResendDefaults()
    }
  },
  { deep: true, immediate: true }
)
</script>
