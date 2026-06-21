<template>
  <div class="mb-5">
    <CustomBreadcrumb :links="breadcrumbLinks" />
  </div>
  <LoadingOverlay :loading="isLoading">
    <ApplicationForm @submit.prevent="onSubmit" :form="form" :isNewForm="isNewForm">
      <template #footer>
        <div class="flex space-x-3">
          <Button type="submit" :isLoading="formLoading">
            {{ isNewForm ? t('globals.messages.create') : t('globals.messages.save') }}
          </Button>
        </div>
      </template>
    </ApplicationForm>
  </LoadingOverlay>
</template>

<script setup>
import { onMounted, ref, computed } from 'vue'
import api from '@/api'
import ApplicationForm from '@/features/admin/applications/ApplicationForm.vue'
import LoadingOverlay from '@/components/layout/LoadingOverlay.vue'
import { CustomBreadcrumb } from '@shared-ui/components/ui/breadcrumb'
import { Button } from '@shared-ui/components/ui/button'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import { useEmitter } from '@/composables/useEmitter'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { createFormSchema } from '@/features/admin/applications/formSchema.js'

const router = useRouter()
const { t } = useI18n()
const emitter = useEmitter()
const isLoading = ref(false)
const formLoading = ref(false)

const props = defineProps({
  id: {
    type: String,
    required: false
  }
})

const form = useForm({
  validationSchema: toTypedSchema(createFormSchema(t)),
  initialValues: {
    name: '',
    slug: '',
    description: '',
    logo_url: '',
    identity_url: '',
    gateway_app_id: '',
    gateway_api_key: '',
    enabled: true
  }
})

const onSubmit = form.handleSubmit(async (values) => {
  try {
    formLoading.value = true
    if (props.id) {
      await api.updateApplication(props.id, values)
    } else {
      const resp = await api.createApplication(values)
      router.push({ name: 'edit-application', params: { id: resp.data.data.id } })
    }
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'success',
      description: t('globals.messages.savedSuccessfully')
    })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    formLoading.value = false
  }
})

const isNewForm = computed(() => !props.id)

const breadcrumbLinks = [
  { path: 'application-list', label: t('globals.terms.application') },
  { path: '', label: props.id ? t('globals.messages.edit') : t('globals.messages.new') }
]

onMounted(async () => {
  if (props.id) {
    try {
      isLoading.value = true
      const resp = await api.getApplication(props.id)
      form.setValues(resp.data.data)
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    } finally {
      isLoading.value = false
    }
  }
})
</script>
