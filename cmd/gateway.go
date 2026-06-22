package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	applicationmodels "github.com/abhinavxd/libredesk/internal/application/models"
	"github.com/abhinavxd/libredesk/internal/attachment"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/image"
	imodels "github.com/abhinavxd/libredesk/internal/inbox/models"
	mmodels "github.com/abhinavxd/libredesk/internal/media/models"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	"github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"github.com/volatiletech/null/v9"
	"github.com/zerodha/fastglue"
)

const (
	gatewayHeaderAppID       = "X-App-Id"
	gatewayHeaderAPIKey      = "X-App-Key"
	gatewayHeaderIdentityUID = "X-User-Id"
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
	AttachmentIDs    []int          `json:"attachment_ids"`
}

type gatewayIdentityEnvelope struct {
	Data gatewayIdentity `json:"data"`
}

type gatewayIdentity struct {
	ID           string                      `json:"id"`
	FullName     string                      `json:"fullName"`
	FirstName    string                      `json:"firstName"`
	LastName     string                      `json:"lastName"`
	Email        string                      `json:"email"`
	Cedula       string                      `json:"cedula"`
	Roles        []string                    `json:"roles"`
	Permissions  []string                    `json:"permissions"`
	Organization *gatewayIdentityOrg         `json:"organization"`
	Dependencies []gatewayIdentityDependency `json:"dependencies"`
	Status       string                      `json:"status"`
	AreaID       *int                        `json:"areaId"`
	ProjectID    *int                        `json:"projectId"`
	CreatedBy    string                      `json:"createdBy"`
}

type gatewayIdentityOrg struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Nit           string `json:"nit"`
	Status        string `json:"status"`
	CodigoEmpresa string `json:"codigoEmpresa"`
}

type gatewayIdentityDependency struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	CodigoEmpresa string `json:"codigoEmpresa"`
	Nit           string `json:"nit"`
	IsDefault     bool   `json:"isDefault"`
	Active        bool   `json:"active"`
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

	contact, gatewayAttrs, err := resolveGatewayContact(app, ctx, req, scopedExternalUserID)
	if err != nil {
		app.lo.Error("error resolving gateway contact", "application_id", ctx.application.ID, "external_user_id", ctx.externalUserID, "error", err)
		return r.SendErrorEnvelope(http.StatusBadGateway, err.Error(), nil, envelope.GeneralError)
	}

	// Load attachments if the gateway sent media IDs (previously uploaded via
	// POST /api/v1/portal/media/upload).
	var media []mmodels.Media
	if len(req.AttachmentIDs) > 0 {
		media, err = getUnassociatedMedia(app, req.AttachmentIDs)
		if err != nil {
			app.lo.Error("error loading gateway attachments", "error", err)
			return r.SendErrorEnvelope(http.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
		}
	}

	conversationAttrs := mergeGatewayAttributes(req.CustomAttributes, gatewayAttrs)

	// Create conversation.
	conversationID, conversationUUID, err := app.conversation.CreateConversation(
		contact.ID,
		inbox.ID,
		"",
		time.Now(),
		req.Subject,
		true,
		nil,
		conversationAttrs,
		0, 0,
	)
	if err != nil {
		app.lo.Error("error creating ticket conversation", "error", err)
		return r.SendErrorEnvelope(http.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}

	// Create initial contact message.
	if _, err := app.conversation.CreateContactMessage(media, contact.ID, conversationUUID, req.Content, cmodels.ContentTypeText, true); err != nil {
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

	private := false
	messages, err := app.conversation.GetAllConversationMessages(uuid, &private, []string{cmodels.MessageIncoming, cmodels.MessageOutgoing})
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	rootURL, _ := app.setting.GetAppRootURL()
	for i := range messages {
		for j := range messages[i].Attachments {
			att := messages[i].Attachments[j]
			messages[i].Attachments[j].URL = app.media.GetURL(att.UUID, att.ContentType, att.Name)
		}
		resolveQuotedCIDs(app, &messages[i])
		resolveAttachmentCIDs(&messages[i], rootURL)
	}
	app.conversation.ProcessCSATStatus(messages)

	response := struct {
		cmodels.Conversation
		Messages []cmodels.Message `json:"messages"`
	}{
		Conversation: conversation,
		Messages:     messages,
	}

	return r.SendEnvelope(response)
}

func scopeExternalUserID(appSlug, externalUserID string) string {
	return appSlug + ":" + externalUserID
}

func resolveGatewayContact(app *App, ctx gatewayContext, req createTicketRequest, scopedExternalUserID string) (models.User, map[string]any, error) {
	existingContact, err := app.user.GetByExternalID(scopedExternalUserID)
	if err != nil {
		if envErr, ok := err.(envelope.Error); !ok || envErr.ErrorType != envelope.NotFoundError {
			return models.User{}, nil, err
		}
		existingContact = models.User{}
	}

	gatewayAttrs := map[string]any{
		"gateway_application_slug": ctx.application.Slug,
		"gateway_application_name": ctx.application.Name,
		"gateway_application_id":   ctx.application.GatewayAppID,
		"gateway_external_user_id": ctx.externalUserID,
	}
	if ctx.externalOrgID != "" {
		gatewayAttrs["gateway_external_org_id"] = ctx.externalOrgID
	}
	if ctx.externalDependencyID != "" {
		gatewayAttrs["gateway_external_dependency_id"] = ctx.externalDependencyID
	}

	if existingContact.ID > 0 {
		storedAttrs := parseGatewayContactAttributes(existingContact.CustomAttributes)
		conversationAttrs := mergeGatewayAttributes(storedAttrs, gatewayAttrs)

		firstName := firstNonEmpty(existingContact.FirstName, req.FirstName)
		lastName := firstNonEmpty(existingContact.LastName, req.LastName)
		email := firstNonEmpty(existingContact.Email.String, req.Email)
		if firstName != existingContact.FirstName || lastName != existingContact.LastName || email != existingContact.Email.String {
			if err := app.user.UpdateContactBasicInfo(existingContact.ID, firstName, lastName, email); err != nil {
				app.lo.Error("error refreshing existing gateway contact basic info", "contact_id", existingContact.ID, "error", err)
			} else {
				existingContact.FirstName = firstName
				existingContact.LastName = lastName
				existingContact.Email = null.NewString(strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(email) != "")
			}
		}

		if len(gatewayAttrs) > 0 {
			if err := app.user.SaveCustomAttributes(existingContact.ID, gatewayAttrs, false); err != nil {
				app.lo.Error("error saving gateway contact attributes", "contact_id", existingContact.ID, "error", err)
			}
		}

		return existingContact, conversationAttrs, nil
	}

	var identity *gatewayIdentity
	if strings.TrimSpace(ctx.application.IdentityURL) != "" {
		identity, err = fetchGatewayIdentity(ctx)
		if err != nil {
			return models.User{}, nil, err
		}
		mergeInto(gatewayAttrs, buildGatewayIdentityAttributes(ctx, *identity))
	}

	firstName := firstNonEmpty(
		valueOrEmpty(identity, func(i *gatewayIdentity) string { return i.FirstName }),
		req.FirstName,
	)
	lastName := firstNonEmpty(
		valueOrEmpty(identity, func(i *gatewayIdentity) string { return i.LastName }),
		req.LastName,
	)
	email := firstNonEmpty(
		valueOrEmpty(identity, func(i *gatewayIdentity) string { return i.Email }),
		req.Email,
	)

	if firstName == "" && lastName == "" {
		fullName := valueOrEmpty(identity, func(i *gatewayIdentity) string { return i.FullName })
		if fullName != "" {
			firstName = fullName
		}
	}

	if strings.TrimSpace(ctx.application.IdentityURL) == "" && email == "" && firstName == "" && lastName == "" {
		return models.User{}, nil, fmt.Errorf("the application has no identity_url configured and the gateway request did not provide fallback contact fields")
	}

	contact := models.User{
		Email:            null.NewString(strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(email) != ""),
		FirstName:        firstName,
		LastName:         lastName,
		ExternalUserID:   null.StringFrom(scopedExternalUserID),
		CustomAttributes: marshalCustomAttributes(gatewayAttrs, app),
	}
	if err := app.user.CreateContact(&contact); err != nil {
		return models.User{}, nil, err
	}

	if len(gatewayAttrs) > 0 {
		if err := app.user.SaveCustomAttributes(contact.ID, gatewayAttrs, false); err != nil {
			app.lo.Error("error saving gateway contact attributes", "contact_id", contact.ID, "error", err)
		}
	}

	return contact, gatewayAttrs, nil
}

func parseGatewayContactAttributes(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}

	attrs := map[string]any{}
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return nil
	}
	return attrs
}
func fetchGatewayIdentity(ctx gatewayContext) (*gatewayIdentity, error) {
	identityURL := strings.TrimSpace(ctx.application.IdentityURL)
	if identityURL == "" {
		return nil, fmt.Errorf("identity_url is not configured for application %s", ctx.application.Slug)
	}

	req, err := http.NewRequest(http.MethodGet, identityURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating identity request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set(gatewayHeaderIdentityUID, ctx.externalUserID)
	req.Header.Set(gatewayHeaderAppID, ctx.application.GatewayAppID)
	req.Header.Set(gatewayHeaderExternalUID, ctx.externalUserID)
	if ctx.externalOrgID != "" {
		req.Header.Set(gatewayHeaderExternalOrg, ctx.externalOrgID)
	}
	if ctx.externalDependencyID != "" {
		req.Header.Set(gatewayHeaderExternalDep, ctx.externalDependencyID)
	}
	if ctx.requestID != "" {
		req.Header.Set(gatewayHeaderRequestID, ctx.requestID)
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling identity_url: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading identity response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity lookup failed with status %d: %s", resp.StatusCode, summarizeGatewayBody(body))
	}

	var envelopeResponse gatewayIdentityEnvelope
	if err := json.Unmarshal(body, &envelopeResponse); err == nil && envelopeResponse.Data.ID != "" {
		return &envelopeResponse.Data, nil
	}

	var identity gatewayIdentity
	if err := json.Unmarshal(body, &identity); err != nil {
		return nil, fmt.Errorf("decoding identity response: %w", err)
	}
	if identity.ID == "" {
		return nil, fmt.Errorf("identity response did not include a user id")
	}
	return &identity, nil
}

func buildGatewayIdentityAttributes(ctx gatewayContext, identity gatewayIdentity) map[string]any {
	attrs := map[string]any{
		"requester_user_id":     identity.ID,
		"requester_full_name":   identity.FullName,
		"requester_first_name":  identity.FirstName,
		"requester_last_name":   identity.LastName,
		"requester_email":       identity.Email,
		"requester_cedula":      identity.Cedula,
		"requester_status":      identity.Status,
		"requester_roles":       identity.Roles,
		"requester_permissions": identity.Permissions,
	}
	if identity.AreaID != nil {
		attrs["requester_area_id"] = *identity.AreaID
	}
	if identity.ProjectID != nil {
		attrs["requester_project_id"] = *identity.ProjectID
	}
	if identity.CreatedBy != "" {
		attrs["requester_created_by"] = identity.CreatedBy
	}

	if identity.Organization != nil {
		attrs["requester_organization_id"] = identity.Organization.ID
		attrs["requester_organization_name"] = identity.Organization.Name
		attrs["requester_organization_nit"] = identity.Organization.Nit
		attrs["requester_organization_status"] = identity.Organization.Status
		attrs["requester_organization_codigo_empresa"] = identity.Organization.CodigoEmpresa
	}

	if dep := selectGatewayDependency(ctx.externalDependencyID, identity.Dependencies); dep != nil {
		attrs["requester_dependency_id"] = dep.ID
		attrs["requester_dependency_name"] = dep.Name
		attrs["requester_dependency_nit"] = dep.Nit
		attrs["requester_dependency_codigo_empresa"] = dep.CodigoEmpresa
		attrs["requester_dependency_default"] = dep.IsDefault
		attrs["requester_dependency_active"] = dep.Active
	}

	return attrs
}

func selectGatewayDependency(externalDependencyID string, dependencies []gatewayIdentityDependency) *gatewayIdentityDependency {
	if len(dependencies) == 0 {
		return nil
	}

	if externalDependencyID != "" {
		for i := range dependencies {
			if strings.EqualFold(strings.TrimSpace(dependencies[i].ID), strings.TrimSpace(externalDependencyID)) {
				return &dependencies[i]
			}
		}
	}

	for i := range dependencies {
		if dependencies[i].IsDefault {
			return &dependencies[i]
		}
	}

	return &dependencies[0]
}

func mergeGatewayAttributes(base, extra map[string]any) map[string]any {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}

	merged := make(map[string]any, len(base)+len(extra))
	mergeInto(merged, base)
	mergeInto(merged, extra)
	return merged
}

func mergeInto(dst, src map[string]any) {
	for key, value := range src {
		dst[key] = value
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func valueOrEmpty(identity *gatewayIdentity, getter func(*gatewayIdentity) string) string {
	if identity == nil {
		return ""
	}
	return getter(identity)
}

func summarizeGatewayBody(body []byte) string {
	message := strings.TrimSpace(string(body))
	if message == "" {
		return "empty response body"
	}
	if len(message) > 300 {
		return message[:300] + "..."
	}
	return message
}

// handleGatewayMediaUpload allows gateway clients to upload attachments that
// can later be linked to a ticket message via attachment_ids.
func handleGatewayMediaUpload(r *fastglue.Request) error {
	var (
		app     = r.Context.(*App)
		cleanUp = false
	)

	form, err := r.RequestCtx.MultipartForm()
	if err != nil {
		app.lo.Error("error parsing gateway upload form", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("errors.parsingRequest"), nil, envelope.GeneralError)
	}

	files, ok := form.File["files"]
	if !ok || len(files) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("validation.notFoundFile"), nil, envelope.InputError)
	}

	fileHeader := files[0]
	file, err := fileHeader.Open()
	if err != nil {
		app.lo.Error("error reading gateway uploaded file", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}
	defer file.Close()

	consts := app.consts.Load().(*constants)
	if bytesToMegabytes(fileHeader.Size) > float64(consts.MaxFileUploadSizeMB) {
		return r.SendErrorEnvelope(
			fasthttp.StatusRequestEntityTooLarge,
			app.i18n.Ts("media.fileSizeTooLarge", "size", fmt.Sprintf("%dMB", consts.MaxFileUploadSizeMB)),
			nil,
			envelope.GeneralError,
		)
	}

	srcFileName := stringutil.SanitizeFilename(fileHeader.Filename)
	srcContentType := fileHeader.Header.Get("Content-Type")
	srcExt := strings.TrimPrefix(strings.ToLower(filepath.Ext(srcFileName)), ".")
	if !slices.Contains(consts.AllowedUploadFileExtensions, "*") && !slices.Contains(consts.AllowedUploadFileExtensions, srcExt) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("media.fileTypeNotAllowed"), nil, envelope.InputError)
	}

	var id = uuid.New()
	thumbName := image.ThumbPrefix + id.String()
	defer func() {
		if cleanUp {
			app.media.Delete(id.String())
			app.media.Delete(thumbName)
		}
	}()

	var meta = []byte("{}")
	if slices.Contains(image.Exts, srcExt) && image.IsImageByContent(file) {
		file.Seek(0, 0)
		thumbFile, err := image.CreateThumb(image.DefThumbSize, file)
		if err != nil {
			app.lo.Error("error creating thumb image", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
		}
		thumbName, _, err = app.media.Upload(thumbName, srcContentType, thumbFile)
		if err != nil {
			return sendErrorEnvelope(r, err)
		}

		file.Seek(0, 0)
		width, height, err := image.GetDimensions(file)
		if err != nil {
			cleanUp = true
			app.lo.Error("error getting image dimensions", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.errorUploadingFile"), nil, envelope.GeneralError)
		}
		meta, _ = json.Marshal(map[string]interface{}{
			"width":  width,
			"height": height,
		})
	}

	file.Seek(0, 0)
	_, srcContentType, err = app.media.Upload(id.String(), srcContentType, file)
	if err != nil {
		cleanUp = true
		app.lo.Error("error uploading gateway file", "error", err)
		return sendErrorEnvelope(r, err)
	}

	media, err := app.media.Insert(
		null.StringFrom(attachment.DispositionAttachment),
		srcFileName,
		srcContentType,
		"",
		null.NewString(mmodels.ModelMessages, true),
		id.String(),
		null.Int{},
		int(fileHeader.Size),
		meta,
	)
	if err != nil {
		cleanUp = true
		app.lo.Error("error inserting gateway media metadata", "error", err)
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(media)
}
