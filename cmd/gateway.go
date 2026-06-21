package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	applicationmodels "github.com/abhinavxd/libredesk/internal/application/models"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	imodels "github.com/abhinavxd/libredesk/internal/inbox/models"
	"github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/volatiletech/null/v9"
	"github.com/zerodha/fastglue"
)

const (
	gatewayHeaderAppID       = "X-App-Id"
	gatewayHeaderAPIKey      = "X-App-Key"
	gatewayHeaderExternalUID = "X-External-User-Id"
	gatewayHeaderExternalOrg = "X-External-Org-Id"
	gatewayHeaderExternalDep = "X-External-Dependency-Id"
	gatewayHeaderRequestID   = "X-Request-Id"
	gatewayHeaderIdempotency = "Idempotency-Key"
)

type gatewayContext struct {
	application          applicationmodels.Application
	externalUserID       string
	externalOrgID        string
	externalDependencyID string
	requestID            string
	idempotencyKey       string
}

// gatewayAuth authenticates requests from trusted application gateways using
// X-App-Id and X-App-Key headers. It adds the authenticated application and
// external identity to the request context.
func gatewayAuth(handler fastglue.FastRequestHandler) fastglue.FastRequestHandler {
	return func(r *fastglue.Request) error {
		app := r.Context.(*App)

		appID := string(r.RequestCtx.Request.Header.Peek(gatewayHeaderAppID))
		apiKey := string(r.RequestCtx.Request.Header.Peek(gatewayHeaderAPIKey))
		if appID == "" || apiKey == "" {
			return r.SendErrorEnvelope(http.StatusUnauthorized, app.i18n.T("validation.invalidCredential"), nil, envelope.UnauthorizedError)
		}

		application, err := app.application.ValidateGatewayAPIKey(appID, apiKey)
		if err != nil {
			return r.SendErrorEnvelope(http.StatusUnauthorized, app.i18n.T("validation.invalidCredential"), nil, envelope.UnauthorizedError)
		}
		if !application.Enabled {
			return r.SendErrorEnvelope(http.StatusForbidden, app.i18n.T("globals.messages.disabled"), nil, envelope.PermissionError)
		}

		externalUserID := string(r.RequestCtx.Request.Header.Peek(gatewayHeaderExternalUID))
		if externalUserID == "" {
			return r.SendErrorEnvelope(http.StatusBadRequest, app.i18n.Ts("globals.messages.required", "name", "X-External-User-Id"), nil, envelope.InputError)
		}

		r.RequestCtx.SetUserValue("gateway_context", gatewayContext{
			application:          application,
			externalUserID:       externalUserID,
			externalOrgID:        string(r.RequestCtx.Request.Header.Peek(gatewayHeaderExternalOrg)),
			externalDependencyID: string(r.RequestCtx.Request.Header.Peek(gatewayHeaderExternalDep)),
			requestID:            string(r.RequestCtx.Request.Header.Peek(gatewayHeaderRequestID)),
			idempotencyKey:       string(r.RequestCtx.Request.Header.Peek(gatewayHeaderIdempotency)),
		})

		return handler(r)
	}
}

func getGatewayContext(r *fastglue.Request) gatewayContext {
	return r.RequestCtx.UserValue("gateway_context").(gatewayContext)
}

type createTicketRequest struct {
	Subject          string         `json:"subject"`
	Content          string         `json:"content"`
	Email            string         `json:"email"`
	FirstName        string         `json:"first_name"`
	LastName         string         `json:"last_name"`
	CustomAttributes map[string]any `json:"custom_attributes"`
}

// handleGatewayCreateTicket creates a new ticket conversation from a gateway request.
func handleGatewayCreateTicket(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		ctx = getGatewayContext(r)
		req = createTicketRequest{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(http.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	if req.Subject == "" {
		return r.SendErrorEnvelope(http.StatusBadRequest, app.i18n.Ts("globals.messages.required", "name", "subject"), nil, envelope.InputError)
	}
	if req.Content == "" {
		return r.SendErrorEnvelope(http.StatusBadRequest, app.i18n.Ts("globals.messages.required", "name", "content"), nil, envelope.InputError)
	}

	// Scope the external user ID with the application slug to avoid collisions
	// between different apps that may share numeric user IDs.
	scopedExternalUserID := scopeExternalUserID(ctx.application.Slug, ctx.externalUserID)

	// If the gateway sent an idempotency key, return the previously created
	// conversation instead of creating a duplicate ticket.
	if ctx.idempotencyKey != "" {
		existingUUID, err := app.idempotency.Get(ctx.application.ID, ctx.idempotencyKey)
		if err != nil {
			app.lo.Error("error checking idempotency key", "error", err)
			return r.SendErrorEnvelope(http.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
		}
		if existingUUID != "" {
			conversation, err := app.conversation.GetConversation(0, existingUUID, "")
			if err != nil {
				return sendErrorEnvelope(r, err)
			}
			return r.SendEnvelope(conversation)
		}
	}

	// Find the ticket inbox for this application.
	inbox, err := app.inbox.GetByApplicationAndChannel(ctx.application.ID, imodels.ChannelTicket)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if !inbox.Enabled {
		return r.SendErrorEnvelope(http.StatusBadRequest, app.i18n.T("globals.messages.disabled"), nil, envelope.InputError)
	}

	// Find or create contact.
	contact := models.User{
		Email:            null.NewString(strings.ToLower(strings.TrimSpace(req.Email)), req.Email != ""),
		FirstName:        req.FirstName,
		LastName:         req.LastName,
		ExternalUserID:   null.StringFrom(scopedExternalUserID),
		CustomAttributes: json.RawMessage(`{}`),
	}
	if err := app.user.CreateContact(&contact); err != nil {
		app.lo.Error("error creating contact from gateway", "error", err)
		return r.SendErrorEnvelope(http.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}

	// Create conversation.
	conversationID, conversationUUID, err := app.conversation.CreateConversation(
		contact.ID,
		inbox.ID,
		"",
		time.Now(),
		req.Subject,
		true,
		nil,
		req.CustomAttributes,
		0, 0,
	)
	if err != nil {
		app.lo.Error("error creating ticket conversation", "error", err)
		return r.SendErrorEnvelope(http.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}

	// Create initial contact message.
	if _, err := app.conversation.CreateContactMessage(nil, contact.ID, conversationUUID, req.Content, cmodels.ContentTypeText, true); err != nil {
		if err := app.conversation.DeleteConversation(conversationUUID); err != nil {
			app.lo.Error("error deleting conversation after message failure", "error", err)
		}
		return r.SendErrorEnvelope(http.StatusInternalServerError, app.i18n.T("globals.messages.errorSendingMessage"), nil, envelope.GeneralError)
	}

	conversation, err := app.conversation.GetConversation(conversationID, "", "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Store the idempotency key so retries return this same conversation.
	if ctx.idempotencyKey != "" {
		if err := app.idempotency.Set(ctx.application.ID, ctx.idempotencyKey, string(r.RequestCtx.Path()), conversationUUID); err != nil {
			app.lo.Error("error storing idempotency key", "error", err)
			// Do not fail the request; the ticket was already created successfully.
		}
	}

	return r.SendEnvelope(conversation)
}

// handleGatewayListTickets lists tickets for the authenticated external user.
func handleGatewayListTickets(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		ctx = getGatewayContext(r)
	)

	page, pageSize := getPagination(r)
	scopedExternalUserID := scopeExternalUserID(ctx.application.Slug, ctx.externalUserID)

	contact, err := app.user.GetByExternalID(scopedExternalUserID)
	if err != nil {
		envErr, ok := err.(envelope.Error)
		if ok && envErr.ErrorType == envelope.NotFoundError {
			return r.SendEnvelope(envelope.PageResults{
				Results:    []cmodels.TicketListItem{},
				Total:      0,
				PerPage:    pageSize,
				TotalPages: 0,
				Page:       page,
			})
		}
		return sendErrorEnvelope(r, err)
	}

	conversations, err := app.conversation.GetConversationsByApplicationAndContact(ctx.application.ID, contact.ID, page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	total := 0
	if len(conversations) > 0 {
		total = conversations[0].Total
	}

	return r.SendEnvelope(envelope.PageResults{
		Results:    conversations,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

// handleGatewayGetTicket returns a single ticket if it belongs to the authenticated external user.
func handleGatewayGetTicket(r *fastglue.Request) error {
	var (
		app  = r.Context.(*App)
		ctx  = getGatewayContext(r)
		uuid = r.RequestCtx.UserValue("uuid").(string)
	)

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

	return r.SendEnvelope(conversation)
}

func scopeExternalUserID(appSlug, externalUserID string) string {
	return appSlug + ":" + externalUserID
}
