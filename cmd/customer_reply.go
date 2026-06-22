package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	mmodels "github.com/abhinavxd/libredesk/internal/media/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

const (
	customerReplyRequestedKey      = "customer_reply_requested"
	customerReplyStatusKey         = "customer_reply_status"
	customerReplyAllowAttachKey    = "customer_reply_allow_attachments"
	customerReplyMaxMessagesKey    = "customer_reply_max_messages"
	customerReplyResponseCountKey  = "customer_reply_response_count"
	customerReplyRequestedAtKey    = "customer_reply_requested_at"
	customerReplyExpiresAtKey      = "customer_reply_expires_at"
	customerReplyLastMessageKey    = "customer_reply_last_requested_message_uuid"
	customerReplyLastResponseAtKey = "customer_reply_last_response_at"

	customerReplyStatusOpen     = "open"
	customerReplyStatusClosed   = "closed"
	customerReplyStatusAnswered = "answered"
	customerReplyStatusExpired  = "expired"
)

type requestCustomerReplyReq struct {
	Message          string `json:"message"`
	AllowAttachments bool   `json:"allow_attachments"`
	MaxMessages      int    `json:"max_messages"`
	ExpiresAt        string `json:"expires_at"`
}

type portalReplyTicketRequest struct {
	Content       string `json:"content"`
	AttachmentIDs []int  `json:"attachment_ids"`
}

type customerReplyWindow struct {
	Open             bool
	Status           string
	AllowAttachments bool
	MaxMessages      int
	ResponseCount    int
	ExpiresAt        string
}

func handleRequestCustomerReply(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		req   = requestCustomerReplyReq{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}
	if strings.TrimSpace(req.Message) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.required", "name", "message"), nil, envelope.InputError)
	}
	if req.MaxMessages <= 0 {
		req.MaxMessages = 1
	}

	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conversation, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	attrs := parseConversationAttributes(conversation.CustomAttributes)
	window, err := buildRequestedReplyWindow(req)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, envelope.InputError)
	}

	messageMeta := map[string]any{
		"customer_reply_request": true,
	}
	message, err := app.conversation.CreateAgentPublicMessage(nil, user.ID, uuid, req.Message, messageMeta)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	applyCustomerReplyWindow(attrs, window, message.UUID)
	if err := app.conversation.UpdateConversationCustomAttributes(uuid, attrs); err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(map[string]any{
		"message":            message,
		"customer_can_reply": window.Open,
		"custom_attributes":  attrs,
	})
}

func handleGatewayReplyTicket(r *fastglue.Request) error {
	var (
		app  = r.Context.(*App)
		ctx  = getGatewayContext(r)
		uuid = r.RequestCtx.UserValue("uuid").(string)
		req  = portalReplyTicketRequest{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(http.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}
	if strings.TrimSpace(req.Content) == "" && len(req.AttachmentIDs) == 0 {
		return r.SendErrorEnvelope(http.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	conversation, err := app.conversation.GetConversation(0, uuid, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if !conversation.ApplicationID.Valid || conversation.ApplicationID.Int != ctx.application.ID {
		return r.SendErrorEnvelope(http.StatusNotFound, app.i18n.T("globals.messages.notFound"), nil, envelope.NotFoundError)
	}

	scopedExternalUserID := scopeExternalUserID(ctx.application.Slug, ctx.externalUserID)
	if conversation.Contact.ExternalUserID.String != scopedExternalUserID {
		return r.SendErrorEnvelope(http.StatusNotFound, app.i18n.T("globals.messages.notFound"), nil, envelope.NotFoundError)
	}

	attrs := parseConversationAttributes(conversation.CustomAttributes)
	window := getCustomerReplyWindow(attrs)
	if !window.Open {
		return r.SendErrorEnvelope(http.StatusForbidden, "Customer reply is not currently enabled for this ticket", nil, envelope.PermissionError)
	}
	if len(req.AttachmentIDs) > 0 && !window.AllowAttachments {
		return r.SendErrorEnvelope(http.StatusBadRequest, "Attachments are not allowed for this reply request", nil, envelope.InputError)
	}

	var media []mmodels.Media
	if len(req.AttachmentIDs) > 0 {
		media, err = getUnassociatedMedia(app, req.AttachmentIDs)
		if err != nil {
			return r.SendErrorEnvelope(http.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
		}
	}

	message, err := app.conversation.CreateContactMessage(media, conversation.ContactID, uuid, req.Content, cmodels.ContentTypeText, false)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	window.ResponseCount++
	if window.ResponseCount >= window.MaxMessages {
		window.Open = false
		window.Status = customerReplyStatusAnswered
		attrs[customerReplyRequestedKey] = false
	} else {
		attrs[customerReplyRequestedKey] = true
	}
	attrs[customerReplyStatusKey] = window.Status
	attrs[customerReplyResponseCountKey] = window.ResponseCount
	attrs[customerReplyLastResponseAtKey] = time.Now().UTC().Format(time.RFC3339)
	if err := app.conversation.UpdateConversationCustomAttributes(uuid, attrs); err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(map[string]any{
		"message":            message,
		"customer_can_reply": window.Open,
		"custom_attributes":  attrs,
	})
}

func buildRequestedReplyWindow(req requestCustomerReplyReq) (customerReplyWindow, error) {
	window := customerReplyWindow{
		Open:             true,
		Status:           customerReplyStatusOpen,
		AllowAttachments: req.AllowAttachments,
		MaxMessages:      req.MaxMessages,
		ResponseCount:    0,
	}
	if strings.TrimSpace(req.ExpiresAt) == "" {
		return window, nil
	}
	parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		return customerReplyWindow{}, fmt.Errorf("expires_at must be a valid RFC3339 datetime")
	}
	window.ExpiresAt = parsed.UTC().Format(time.RFC3339)
	return window, nil
}

func applyCustomerReplyWindow(attrs map[string]any, window customerReplyWindow, requestMessageUUID string) {
	attrs[customerReplyRequestedKey] = window.Open
	attrs[customerReplyStatusKey] = window.Status
	attrs[customerReplyAllowAttachKey] = window.AllowAttachments
	attrs[customerReplyMaxMessagesKey] = window.MaxMessages
	attrs[customerReplyResponseCountKey] = window.ResponseCount
	attrs[customerReplyRequestedAtKey] = time.Now().UTC().Format(time.RFC3339)
	attrs[customerReplyLastMessageKey] = requestMessageUUID
	delete(attrs, customerReplyLastResponseAtKey)
	if window.ExpiresAt != "" {
		attrs[customerReplyExpiresAtKey] = window.ExpiresAt
	} else {
		delete(attrs, customerReplyExpiresAtKey)
	}
}

func parseConversationAttributes(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	attrs := map[string]any{}
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return map[string]any{}
	}
	return attrs
}

func getCustomerReplyWindow(attrs map[string]any) customerReplyWindow {
	window := customerReplyWindow{
		Open:             false,
		Status:           stringFromMap(attrs, customerReplyStatusKey),
		AllowAttachments: boolFromMap(attrs, customerReplyAllowAttachKey),
		MaxMessages:      intFromMap(attrs, customerReplyMaxMessagesKey, 1),
		ResponseCount:    intFromMap(attrs, customerReplyResponseCountKey, 0),
		ExpiresAt:        stringFromMap(attrs, customerReplyExpiresAtKey),
	}
	requested := boolFromMap(attrs, customerReplyRequestedKey)
	if window.Status == "" {
		window.Status = customerReplyStatusClosed
	}
	if requested && window.Status == customerReplyStatusOpen && window.ResponseCount < window.MaxMessages {
		window.Open = true
	}
	if window.ExpiresAt != "" {
		if expiry, err := time.Parse(time.RFC3339, window.ExpiresAt); err == nil && time.Now().After(expiry) {
			window.Open = false
			window.Status = customerReplyStatusExpired
		}
	}
	return window
}

func boolFromMap(attrs map[string]any, key string) bool {
	v, ok := attrs[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func intFromMap(attrs map[string]any, key string, def int) int {
	v, ok := attrs[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return def
	}
}

func stringFromMap(attrs map[string]any, key string) string {
	v, ok := attrs[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
