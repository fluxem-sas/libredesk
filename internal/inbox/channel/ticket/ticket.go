// Package ticket implements the ticket channel inbox.
// Ticket inboxes do not send or receive messages directly; they act as
// classification buckets for conversations created through the gateway API.
package ticket

import (
	"context"
	"errors"

	"github.com/abhinavxd/libredesk/internal/conversation/models"
)

// ChannelTicket is the channel identifier.
const ChannelTicket = "ticket"

// Config holds optional ticket inbox configuration.
type Config struct {
	AllowAttachments bool `json:"allow_attachments"`
}

// Opts contains the options for creating a ticket inbox.
type Opts struct {
	ID     int
	Name   string
	Config Config
}

// Ticket implements the inbox.Inbox interface for ticket channels.
type Ticket struct {
	id   int
	name string
	cfg  Config
}

// New creates a new ticket inbox.
func New(opts Opts) *Ticket {
	return &Ticket{
		id:   opts.ID,
		name: opts.Name,
		cfg:  opts.Config,
	}
}

// Identifier returns the inbox ID.
func (t *Ticket) Identifier() int { return t.id }

// Name returns the inbox name.
func (t *Ticket) Name() string { return t.name }

// Channel returns the channel identifier.
func (t *Ticket) Channel() string { return ChannelTicket }

// FromAddress returns an empty string as ticket inboxes have no from address.
func (t *Ticket) FromAddress() string { return "" }

// FromNameTemplate returns an empty string as ticket inboxes have no template.
func (t *Ticket) FromNameTemplate() string { return "" }

// ReplyToAddress returns an empty string as ticket inboxes have no reply-to.
func (t *Ticket) ReplyToAddress() string { return "" }

// Receive is a no-op for ticket inboxes.
func (t *Ticket) Receive(ctx context.Context) error { return nil }

// Send returns an error because ticket inboxes do not send outbound messages.
func (t *Ticket) Send(msg models.OutboundMessage) error {
	return errors.New("ticket inboxes do not send messages")
}

// Close is a no-op for ticket inboxes.
func (t *Ticket) Close() error { return nil }
