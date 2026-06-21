<script setup>
import { useOnline } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { useWidgetStore } from '@widget/store/widget.js'
import BaseBanner from './BaseBanner.vue'

const isOnline = useOnline()
const { connectionFailed, connecting, connected } = storeToRefs(useWidgetStore())
</script>

<template>
  <BaseBanner
    v-if="!isOnline"
    :text="$t('globals.messages.noInternetConnection')"
    color-class="bg-warning/15 text-warning dark:bg-warning/20 dark:text-warning"
  />
  <BaseBanner
    v-else-if="connectionFailed"
    :text="$t('globals.messages.connectionFailedRefresh')"
    color-class="bg-destructive/15 text-destructive dark:bg-destructive/20 dark:text-destructive"
  />
  <BaseBanner
    v-else-if="connected"
    :text="$t('globals.messages.connected')"
    color-class="bg-success/15 text-success dark:bg-success/20 dark:text-success"
  />
  <BaseBanner
    v-else-if="connecting"
    :text="$t('globals.messages.connecting')"
    color-class="bg-warning/15 text-warning dark:bg-warning/20 dark:text-warning"
  />
</template>