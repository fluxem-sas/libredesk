package eventtrigger

import wmodels "github.com/abhinavxd/libredesk/internal/webhook/models"

// WebhookTrigger dispatches Libredesk events to external integrations.
type WebhookTrigger interface {
	TriggerEvent(event wmodels.WebhookEvent, data any)
}

// SlackTrigger dispatches events to Slack routing rules.
type SlackTrigger interface {
	TriggerEvent(event string, data any)
}

// Hub fans out events to registered triggers.
type Hub struct {
	Webhook WebhookTrigger
	Slack   SlackTrigger
}

// TriggerEvent sends an event to all configured triggers.
func (h *Hub) TriggerEvent(event wmodels.WebhookEvent, data any) {
	if h.Webhook != nil {
		h.Webhook.TriggerEvent(event, data)
	}
	if h.Slack != nil {
		h.Slack.TriggerEvent(string(event), data)
	}
}
