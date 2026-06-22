package models

import (
	"time"

	"github.com/lib/pq"
	"github.com/volatiletech/null/v9"
)

// Supported events (aligned with outbound webhooks).
const (
	EventConversationCreated       = "conversation.created"
	EventConversationStatusChanged = "conversation.status_changed"
	EventConversationTagsChanged   = "conversation.tags_changed"
	EventConversationAssigned      = "conversation.assigned"
	EventConversationUnassigned    = "conversation.unassigned"
	EventMessageCreated            = "message.created"
)

var SupportedEvents = []string{
	EventConversationCreated,
	EventConversationStatusChanged,
	EventConversationTagsChanged,
	EventConversationAssigned,
	EventConversationUnassigned,
	EventMessageCreated,
}

// Integration represents a connected Slack workspace.
type Integration struct {
	ID           int         `db:"id" json:"id"`
	CreatedAt    time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time   `db:"updated_at" json:"updated_at"`
	TeamID       string      `db:"team_id" json:"team_id"`
	TeamName     string      `db:"team_name" json:"team_name"`
	BotToken     string      `db:"bot_token" json:"-"`
	BotUserID    string      `db:"bot_user_id" json:"bot_user_id"`
	InstalledBy  null.Int    `db:"installed_by" json:"installed_by"`
	IsActive     bool        `db:"is_active" json:"is_active"`
	Connected    bool        `db:"-" json:"connected"`
	RulesCount   int         `db:"-" json:"rules_count,omitempty"`
}

// RoutingRule routes Libredesk events to a Slack channel.
type RoutingRule struct {
	ID                int            `db:"id" json:"id"`
	CreatedAt         time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at" json:"updated_at"`
	IntegrationID     int            `db:"integration_id" json:"integration_id"`
	Name              string         `db:"name" json:"name"`
	InboxID           null.Int       `db:"inbox_id" json:"inbox_id"`
	InboxName         null.String    `db:"inbox_name" json:"inbox_name,omitempty"`
	Events            pq.StringArray `db:"events" json:"events"`
	SlackChannelID    string         `db:"slack_channel_id" json:"slack_channel_id"`
	SlackChannelName  string         `db:"slack_channel_name" json:"slack_channel_name"`
	IsActive          bool           `db:"is_active" json:"is_active"`
}

// Channel represents a Slack channel for admin UI selection.
type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OAuthAccessResponse is the Slack oauth.v2.access response subset.
type OAuthAccessResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	BotUserID   string `json:"bot_user_id"`
	Team        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
	AuthedUser struct {
		ID string `json:"id"`
	} `json:"authed_user"`
}

// ConversationsListResponse is the Slack conversations.list response subset.
type ConversationsListResponse struct {
	OK               bool `json:"ok"`
	Error            string `json:"error"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
	Channels []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"channels"`
}

// PostMessageResponse is the Slack chat.postMessage response subset.
type PostMessageResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	TS    string `json:"ts"`
}
