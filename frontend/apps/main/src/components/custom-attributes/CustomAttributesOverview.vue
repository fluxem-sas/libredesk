<template>
  <div class="space-y-3">
    <div v-if="normalizedAttributes.length === 0" class="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
      {{ emptyText }}
    </div>

    <div v-else class="space-y-2">
      <div
        v-for="attribute in normalizedAttributes"
        :key="attribute.key"
        class="rounded-md border bg-muted/20 px-3 py-2"
      >
        <div class="min-w-0">
          <p class="text-xs font-medium uppercase tracking-wide text-muted-foreground">
            {{ attribute.label }}
          </p>
          <p class="mt-1 break-words text-sm text-foreground whitespace-pre-wrap">
            {{ attribute.displayValue }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  customAttributes: {
    type: Object,
    default: () => ({})
  },
  attributes: {
    type: Array,
    default: () => ([])
  },
  emptyText: {
    type: String,
    default: ''
  }
})

const { t } = useI18n()

const attributeMap = computed(() => {
  return Object.fromEntries((props.attributes || []).map((attribute) => [attribute.key, attribute]))
})

const normalizedAttributes = computed(() => {
  return Object.entries(props.customAttributes || {})
    .filter(([, value]) => value !== null && value !== undefined && value !== '')
    .map(([key, value]) => {
      const definition = attributeMap.value[key]
      return {
        key,
        label: definition?.name || formatKey(key),
        displayValue: formatValue(value)
      }
    })
    .sort((a, b) => a.label.localeCompare(b.label))
})

function formatKey(key) {
  return key
    .replace(/[_-]+/g, ' ')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/(^\w|\s\w)/g, (match) => match.toUpperCase())
}

function formatValue(value) {
  if (typeof value === 'boolean') {
    return value ? t('globals.messages.yes', { name: '' }).trim() : t('globals.messages.no', { name: '' }).trim()
  }

  if (Array.isArray(value)) {
    return value.length ? value.map((item) => formatValue(item)).join(', ') : t('globals.terms.none')
  }

  if (typeof value === 'object') {
    try {
      return JSON.stringify(value, null, 2)
    } catch {
      return String(value)
    }
  }

  return String(value)
}
</script>
