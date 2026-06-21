package main

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/abhinavxd/libredesk/internal/inbox"
	"github.com/abhinavxd/libredesk/internal/inbox/channel/email"
	imodels "github.com/abhinavxd/libredesk/internal/inbox/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

type resendInboxCandidate struct {
	Record imodels.Inbox
	Config imodels.Config
}

func handleResendWebhook(r *fastglue.Request) error {
	app := r.Context.(*App)

	candidates, err := getEnabledResendInboxes(app)
	if err != nil {
		app.lo.Error("error loading resend inboxes", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, "general_error")
	}
	if len(candidates) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, app.i18n.T("globals.messages.somethingWentWrong"), nil, "input_error")
	}

	rawBody := append([]byte(nil), r.RequestCtx.PostBody()...)
	headers := map[string]string{
		"svix-id":        string(r.RequestCtx.Request.Header.Peek("svix-id")),
		"svix-timestamp": string(r.RequestCtx.Request.Header.Peek("svix-timestamp")),
		"svix-signature": string(r.RequestCtx.Request.Header.Peek("svix-signature")),
	}

	verified := make([]resendInboxCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Config.Resend == nil || strings.TrimSpace(candidate.Config.Resend.WebhookSecret) == "" {
			continue
		}
		if err := email.VerifyResendWebhook(rawBody, headers, candidate.Config.Resend.WebhookSecret); err == nil {
			verified = append(verified, candidate)
		}
	}
	if len(verified) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, app.i18n.T("globals.messages.somethingWentWrong"), nil, "input_error")
	}

	var event email.ResendWebhookEvent
	if err := json.Unmarshal(rawBody, &event); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), err.Error(), "input_error")
	}

	if event.Type != "email.received" {
		return r.SendEnvelope(true)
	}

	recipients := make([]string, 0, len(event.Data.To)+len(event.Data.CC)+len(event.Data.BCC))
	recipients = append(recipients, event.Data.To...)
	recipients = append(recipients, event.Data.CC...)
	recipients = append(recipients, event.Data.BCC...)

	var matched *resendInboxCandidate
	for i := range verified {
		if email.MatchResendInbox(verified[i].Config, recipients) {
			matched = &verified[i]
			break
		}
	}
	if matched == nil {
		app.lo.Info("ignoring resend webhook with no matching inbox", "recipients", recipients)
		return r.SendEnvelope(true)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := email.ProcessResendReceivedEmail(ctx, event, matched.Record.ID, matched.Config, app.conversation, app.user, app.lo); err != nil {
		app.lo.Error("error processing resend webhook", "inbox_id", matched.Record.ID, "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, "general_error")
	}

	return r.SendEnvelope(true)
}

func getEnabledResendInboxes(app *App) ([]resendInboxCandidate, error) {
	records, err := app.inbox.GetAll()
	if err != nil {
		return nil, err
	}

	out := make([]resendInboxCandidate, 0)
	for _, record := range records {
		if record.Channel != inbox.ChannelEmail || !record.Enabled {
			continue
		}

		var cfg imodels.Config
		if err := json.Unmarshal(record.Config, &cfg); err != nil {
			continue
		}

		provider := cfg.Provider
		if provider == "" && cfg.Resend != nil {
			provider = imodels.ProviderResend
		}
		if provider != imodels.ProviderResend {
			continue
		}

		out = append(out, resendInboxCandidate{Record: record, Config: cfg})
	}

	return out, nil
}
