<template>
  <LoadingOverlay :loading="isLoading" reserve-height>
    <div class="space-y-6 max-w-4xl">
      <div>
        <h2 class="text-xl font-semibold">{{ t('slack.title') }}</h2>
        <p class="text-sm text-muted-foreground mt-1">{{ t('slack.description') }}</p>
      </div>

      <Card class="box">
        <CardContent class="p-6 space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div>
              <p class="font-medium">
                {{ integration.connected ? t('slack.connected') : t('slack.notConnected') }}
              </p>
              <p v-if="integration.team_name" class="text-sm text-muted-foreground">
                {{ integration.team_name }}
              </p>
            </div>
            <div class="flex gap-2">
              <Button v-if="!integration.connected" @click="connectSlack">
                {{ t('slack.connect') }}
              </Button>
              <template v-else>
                <Button variant="outline" @click="toggleIntegration">
                  {{ integration.is_active ? t('globals.terms.disable') : t('globals.terms.enable') }}
                </Button>
                <Button variant="destructive" @click="disconnect">
                  {{ t('slack.disconnect') }}
                </Button>
              </template>
            </div>
          </div>
        </CardContent>
      </Card>

      <div v-if="integration.connected" class="space-y-4">
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-lg font-semibold">{{ t('slack.rules') }}</h3>
            <p class="text-sm text-muted-foreground">{{ t('slack.rulesHelp') }}</p>
          </div>
          <Button @click="openRuleDialog()">{{ t('slack.newRule') }}</Button>
        </div>

        <div class="rounded border border-border overflow-hidden">
          <table class="w-full text-sm">
            <thead class="bg-muted/40">
              <tr>
                <th class="text-left p-3 font-medium">{{ t('globals.terms.name') }}</th>
                <th class="text-left p-3 font-medium">Inbox</th>
                <th class="text-left p-3 font-medium">{{ t('slack.channel') }}</th>
                <th class="text-left p-3 font-medium">{{ t('globals.terms.event', 2) }}</th>
                <th class="text-right p-3 font-medium">{{ t('globals.terms.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="rule in rules" :key="rule.id" class="border-t border-border">
                <td class="p-3">{{ rule.name }}</td>
                <td class="p-3">{{ rule.inbox_name || t('slack.allInboxes') }}</td>
                <td class="p-3">#{{ rule.slack_channel_name || rule.slack_channel_id }}</td>
                <td class="p-3">{{ rule.events.join(', ') }}</td>
                <td class="p-3 text-right space-x-2">
                  <Button size="sm" variant="outline" @click="openRuleDialog(rule)">
                    {{ t('globals.terms.edit') }}
                  </Button>
                  <Button size="sm" variant="outline" @click="toggleRule(rule)">
                    {{ rule.is_active ? t('globals.terms.disable') : t('globals.terms.enable') }}
                  </Button>
                  <Button size="sm" variant="destructive" @click="removeRule(rule)">
                    {{ t('globals.terms.delete') }}
                  </Button>
                </td>
              </tr>
              <tr v-if="!rules.length">
                <td colspan="5" class="p-6 text-center text-muted-foreground">
                  {{ t('globals.messages.noResults') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <Dialog :open="ruleDialogOpen" @update:open="ruleDialogOpen = $event">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {{ editingRule ? t('slack.editRule') : t('slack.newRule') }}
          </DialogTitle>
        </DialogHeader>
        <form class="space-y-4" @submit.prevent="saveRule">
          <div class="space-y-2">
            <Label>{{ t('globals.terms.name') }}</Label>
            <Input v-model="ruleForm.name" required />
          </div>
          <div class="space-y-2">
            <Label>Inbox</Label>
            <Select v-model="ruleForm.inbox_id">
              <SelectTrigger>
                <SelectValue :placeholder="t('slack.allInboxes')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{{ t('slack.allInboxes') }}</SelectItem>
                <SelectItem v-for="inbox in inboxes" :key="inbox.id" :value="String(inbox.id)">
                  {{ inbox.name }} ({{ inbox.channel }})
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-2">
            <Label>{{ t('slack.channel') }}</Label>
            <Select v-model="ruleForm.slack_channel_id" required>
              <SelectTrigger>
                <SelectValue :placeholder="t('slack.selectChannel')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="channel in channels"
                  :key="channel.id"
                  :value="channel.id"
                >
                  #{{ channel.name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <Button
              type="button"
              variant="outline"
              size="sm"
              :disabled="!ruleForm.slack_channel_id || testing"
              @click="sendTest"
            >
              {{ t('slack.testMessage') }}
            </Button>
          </div>
          <div class="space-y-2">
            <Label>{{ t('globals.terms.event', 2) }}</Label>
            <div class="grid gap-2 sm:grid-cols-2">
              <label
                v-for="event in supportedEvents"
                :key="event"
                class="flex items-center gap-2 text-sm"
              >
                <Checkbox
                  :checked="ruleForm.events.includes(event)"
                  @update:checked="(checked) => toggleEvent(event, checked)"
                />
                {{ event }}
              </label>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" @click="ruleDialogOpen = false">
              {{ t('globals.terms.cancel') }}
            </Button>
            <Button type="submit" :disabled="saving">
              {{ t('globals.terms.save') }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </LoadingOverlay>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import api from '@/api'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import LoadingOverlay from '@/components/layout/LoadingOverlay.vue'
import { Button } from '@shared-ui/components/ui/button'
import { Card, CardContent } from '@shared-ui/components/ui/card'
import { Input } from '@shared-ui/components/ui/input'
import { Label } from '@shared-ui/components/ui/label'
import { Checkbox } from '@shared-ui/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@shared-ui/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@shared-ui/components/ui/select'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const emitter = useEmitter()

const isLoading = ref(false)
const saving = ref(false)
const testing = ref(false)
const integration = ref({ connected: false, is_active: false })
const rules = ref([])
const inboxes = ref([])
const channels = ref([])
const supportedEvents = ref([])
const ruleDialogOpen = ref(false)
const editingRule = ref(null)
const ruleForm = ref({
  name: '',
  inbox_id: 'all',
  slack_channel_id: '',
  events: ['conversation.created'],
  is_active: true
})

onMounted(async () => {
  if (route.query.slack === 'connected') {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: t('slack.connected') })
    router.replace({ query: {} })
  } else if (route.query.slack === 'error') {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: t('slack.oauthFailed')
    })
    router.replace({ query: {} })
  }
  await loadAll()
})

const loadAll = async () => {
  isLoading.value = true
  try {
    const [integrationResp, rulesResp, inboxesResp, eventsResp] = await Promise.all([
      api.getSlackIntegration(),
      api.getSlackRules(),
      api.getInboxes(),
      api.getSlackEvents()
    ])
    integration.value = integrationResp.data.data || { connected: false }
    rules.value = rulesResp.data.data || []
    inboxes.value = inboxesResp.data.data || []
    supportedEvents.value = eventsResp.data.data || []

    if (integration.value.connected) {
      const channelsResp = await api.getSlackChannels()
      channels.value = channelsResp.data.data || []
    }
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    isLoading.value = false
  }
}

const connectSlack = async () => {
  try {
    const resp = await api.startSlackOAuth()
    window.location.href = resp.data.data.url
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  }
}

const disconnect = async () => {
  if (!integration.value.id) return
  await api.disconnectSlack(integration.value.id)
  await loadAll()
}

const toggleIntegration = async () => {
  if (!integration.value.id) return
  await api.toggleSlackIntegration(integration.value.id)
  await loadAll()
}

const openRuleDialog = (rule = null) => {
  editingRule.value = rule
  if (rule) {
    ruleForm.value = {
      name: rule.name,
      inbox_id: rule.inbox_id ? String(rule.inbox_id) : 'all',
      slack_channel_id: rule.slack_channel_id,
      events: [...rule.events],
      is_active: rule.is_active
    }
  } else {
    ruleForm.value = {
      name: '',
      inbox_id: 'all',
      slack_channel_id: '',
      events: ['conversation.created'],
      is_active: true
    }
  }
  ruleDialogOpen.value = true
}

const toggleEvent = (event, checked) => {
  if (checked) {
    if (!ruleForm.value.events.includes(event)) {
      ruleForm.value.events.push(event)
    }
  } else {
    ruleForm.value.events = ruleForm.value.events.filter((e) => e !== event)
  }
}

const buildRulePayload = () => {
  const channel = channels.value.find((c) => c.id === ruleForm.value.slack_channel_id)
  return {
    name: ruleForm.value.name,
    inbox_id: ruleForm.value.inbox_id === 'all' ? null : Number(ruleForm.value.inbox_id),
    slack_channel_id: ruleForm.value.slack_channel_id,
    slack_channel_name: channel?.name || '',
    events: ruleForm.value.events,
    is_active: ruleForm.value.is_active
  }
}

const saveRule = async () => {
  saving.value = true
  try {
    const payload = buildRulePayload()
    if (editingRule.value) {
      await api.updateSlackRule(editingRule.value.id, payload)
    } else {
      await api.createSlackRule(payload)
    }
    ruleDialogOpen.value = false
    await loadAll()
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    saving.value = false
  }
}

const toggleRule = async (rule) => {
  await api.toggleSlackRule(rule.id)
  await loadAll()
}

const removeRule = async (rule) => {
  await api.deleteSlackRule(rule.id)
  await loadAll()
}

const sendTest = async () => {
  testing.value = true
  try {
    await api.testSlackChannel(ruleForm.value.slack_channel_id)
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: t('slack.testSuccess') })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    testing.value = false
  }
}
</script>
