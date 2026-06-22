package slack

import (
	"encoding/json"
	"fmt"
	"strings"

	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/volatiletech/null/v9"
)

// buildConversationMessage formats a Slack Block Kit message for conversation events.
func buildConversationMessage(event string, appBaseURL string, data any) (blocks []map[string]any, fallback string) {
	conversation, convMap := extractConversation(data)
	if conversation == nil {
		fallback = fmt.Sprintf("Libredesk event: %s", event)
		return nil, fallback
	}

	ref := conversation.ReferenceNumber
	subject := strings.TrimSpace(conversation.Subject.String)
	if subject == "" {
		subject = "—"
	}

	inboxLabel := conversation.InboxName
	if inboxLabel == "" {
		inboxLabel = fmt.Sprintf("Inbox #%d", conversation.InboxID)
	}

	contactName := strings.TrimSpace(strings.TrimSpace(conversation.Contact.FirstName + " " + conversation.Contact.LastName))
	if contactName == "" && conversation.Contact.Email.Valid {
		contactName = conversation.Contact.Email.String
	}
	if contactName == "" {
		contactName = "—"
	}

	title := eventTitle(event, convMap)
	fallback = fmt.Sprintf("%s #%s — %s", title, ref, subject)

	conversationURL := strings.TrimRight(appBaseURL, "/") + "/inboxes/my/conversation/" + conversation.UUID

	blocks = []map[string]any{
		{
			"type": "header",
			"text": map[string]any{
				"type": "plain_text",
				"text": fmt.Sprintf("%s #%s", title, ref),
			},
		},
		{
			"type": "section",
			"fields": []map[string]any{
				{"type": "mrkdwn", "text": fmt.Sprintf("*Inbox:*\n%s", inboxLabel)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Canal:*\n%s", conversation.InboxChannel)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Contacto:*\n%s", contactName)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Estado:*\n%s", nullString(conversation.Status))},
			},
		},
		{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Asunto:*\n%s", subject),
			},
		},
	}

	if preview := strings.TrimSpace(conversation.LastMessage.String); preview != "" {
		if len(preview) > 280 {
			preview = preview[:277] + "..."
		}
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Último mensaje:*\n%s", preview),
			},
		})
	}

	blocks = append(blocks, map[string]any{
		"type": "actions",
		"elements": []map[string]any{
			{
				"type": "button",
				"text": map[string]any{
					"type": "plain_text",
					"text": "Abrir en FluxemDesk",
				},
				"url": conversationURL,
			},
		},
	})

	return blocks, fallback
}

func extractConversation(data any) (*cmodels.Conversation, map[string]any) {
	switch v := data.(type) {
	case cmodels.Conversation:
		return &v, nil
	case *cmodels.Conversation:
		return v, nil
	case map[string]any:
		if raw, ok := v["conversation"]; ok {
			if conv, _ := extractConversation(raw); conv != nil {
				return conv, v
			}
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, v
		}
		var conv cmodels.Conversation
		if err := json.Unmarshal(b, &conv); err != nil {
			return nil, v
		}
		return &conv, v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, nil
		}
		var conv cmodels.Conversation
		if err := json.Unmarshal(b, &conv); err != nil {
			return nil, nil
		}
		return &conv, nil
	}
}

func eventTitle(event string, convMap map[string]any) string {
	switch event {
	case "conversation.created":
		return "Nuevo ticket"
	case "conversation.assigned":
		return "Ticket asignado"
	case "conversation.unassigned":
		return "Ticket sin asignar"
	case "conversation.status_changed":
		return "Estado actualizado"
	case "conversation.tags_changed":
		return "Etiquetas actualizadas"
	case "message.created":
		return "Nuevo mensaje"
	default:
		return "Evento Libredesk"
	}
}

func nullString(v null.String) string {
	if v.Valid && strings.TrimSpace(v.String) != "" {
		return v.String
	}
	return "—"
}
