<template>
  <div
    class="flex flex-col p-4 border rounded shadow-sm hover:shadow transition-colors cursor-pointer max-w-xs"
    @click="handleClick">
    <div class="flex items-center mb-2">
      <template v-if="typeof icon === 'string'">
        <img v-if="!iconDark" :src="icon" class="w-6 h-6 mr-2" />
        <template v-else>
          <img :src="icon" class="w-6 h-6 mr-2 dark:hidden" />
          <img :src="iconDark" class="hidden w-6 h-6 mr-2 dark:block" />
        </template>
      </template>
      <component v-else :is="icon" size="24" class="mr-2 text-primary" />
      <h3 class="text-lg font-medium">{{ title }}</h3>
      <Badge v-if="badge" variant="secondary" class="ml-2">{{ badge }}</Badge>
    </div>
    <p class="text-sm text-gray-600 dark:text-gray-400">{{ subTitle }}</p>
  </div>
</template>

<script setup>
import { Badge } from '@shared-ui/components/ui/badge'

const props = defineProps({
  title: String,
  subTitle: String,
  icon: [Function, String],
  iconDark: String,
  onClick: Function,
  badge: String
})

const emit = defineEmits(['click'])

const handleClick = () => {
  props.onClick?.()
  emit('click')
}
</script>
