<template>
  <form @submit.prevent="submitForm(values)" class="space-y-6 w-full">
    <FormField v-slot="{ componentField }" name="name">
      <FormItem>
        <FormLabel>{{ $t('globals.terms.name') }}</FormLabel>
        <FormControl>
          <Input type="text" :placeholder="$t('globals.terms.name')" v-bind="componentField" />
        </FormControl>
        <FormMessage />
      </FormItem>
    </FormField>

    <FormField v-slot="{ componentField }" name="application_id">
      <FormItem>
        <FormLabel>{{ $t('globals.terms.application') }}</FormLabel>
        <FormControl>
          <Select v-bind="componentField">
            <SelectTrigger>
              <SelectValue :placeholder="$t('globals.terms.application')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="application in availableApplications" :key="application.id" :value="application.id">
                {{ application.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </FormControl>
        <FormDescription>{{ $t('admin.inbox.application.description') }}</FormDescription>
        <FormMessage />
      </FormItem>
    </FormField>

    <FormField v-slot="{ value, handleChange }" name="enabled">
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

    <div class="flex space-x-3">
      <Button type="submit" :isLoading="isLoading">
        {{ isNewForm ? $t('globals.messages.create') : $t('globals.messages.save') }}
      </Button>
    </div>
  </form>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '@/api'
import { Checkbox } from '@shared-ui/components/ui/checkbox'
import { Label } from '@shared-ui/components/ui/label'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  FormDescription
} from '@shared-ui/components/ui/form'
import { Input } from '@shared-ui/components/ui/input'
import { Button } from '@shared-ui/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@shared-ui/components/ui/select'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import * as z from 'zod'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const applications = ref([])

const props = defineProps({
  initialValues: {
    type: Object,
    default: () => ({})
  },
  submitForm: {
    type: Function,
    required: true
  },
  isLoading: {
    type: Boolean,
    default: false
  },
  isNewForm: {
    type: Boolean,
    default: false
  }
})

const schema = toTypedSchema(
  z.object({
    name: z.string().min(1, t('globals.messages.required')),
    application_id: z.preprocess(
      (value) => {
        if (value === null || value === undefined || value === '') {
          return undefined
        }
        return Number(value)
      },
      z.number({ required_error: t('globals.messages.required') }).int().positive(t('globals.messages.required'))
    ),
    enabled: z.boolean().optional().default(true)
  })
)

const { values } = useForm({
  validationSchema: schema,
  initialValues: {
    name: props.initialValues.name || '',
    application_id: props.initialValues.application_id ?? undefined,
    enabled: props.initialValues.enabled ?? true
  }
})

const availableApplications = computed(() =>
  applications.value.filter((app) => app.enabled || app.id === values.application_id)
)

onMounted(async () => {
  try {
    const resp = await api.getApplications()
    applications.value = resp.data.data || []
  } catch (error) {
    console.error('Error fetching applications:', error)
  }
})
</script>
