package main

import (
	"strconv"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	smodels "github.com/abhinavxd/libredesk/internal/integrations/slack/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

func handleGetSlackIntegration(r *fastglue.Request) error {
	app := r.Context.(*App)
	integration, err := app.slack.GetIntegration()
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(integration)
}

func handleSlackOAuthStart(r *fastglue.Request) error {
	app := r.Context.(*App)
	auser := r.RequestCtx.UserValue("user").(amodels.User)
	url, err := app.slack.StartOAuth(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(map[string]string{"url": url})
}

func handleSlackOAuthCallback(r *fastglue.Request) error {
	app := r.Context.(*App)
	state := string(r.RequestCtx.QueryArgs().Peek("state"))
	code := string(r.RequestCtx.QueryArgs().Peek("code"))
	slackErr := string(r.RequestCtx.QueryArgs().Peek("error"))

	rootURL, err := app.setting.GetAppRootURL()
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	redirectBase := rootURL + "/admin/integrations/slack"

	if slackErr != "" {
		return r.Redirect(redirectBase+"?slack=error", fasthttp.StatusFound, nil, "")
	}

	if _, err := app.slack.CompleteOAuth(state, code); err != nil {
		app.lo.Error("slack oauth callback failed", "error", err)
		return r.Redirect(redirectBase+"?slack=error", fasthttp.StatusFound, nil, "")
	}

	return r.Redirect(redirectBase+"?slack=connected", fasthttp.StatusFound, nil, "")
}

func handleDisconnectSlack(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, _ := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if id <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	if err := app.slack.Disconnect(id); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

func handleToggleSlackIntegration(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, _ := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if id <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	integration, err := app.slack.ToggleIntegration(id)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(integration)
}

func handleGetSlackChannels(r *fastglue.Request) error {
	app := r.Context.(*App)
	channels, err := app.slack.ListChannels()
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(channels)
}

func handleGetSlackRules(r *fastglue.Request) error {
	app := r.Context.(*App)
	rules, err := app.slack.GetAllRules()
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(rules)
}

func handleGetSlackRule(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, _ := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if id <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	rule, err := app.slack.GetRule(id)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(rule)
}

func handleCreateSlackRule(r *fastglue.Request) error {
	app := r.Context.(*App)
	rule := smodels.RoutingRule{IsActive: true}
	if err := r.Decode(&rule, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), err.Error(), envelope.InputError)
	}
	created, err := app.slack.CreateRule(rule)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(created)
}

func handleUpdateSlackRule(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, _ := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if id <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	rule := smodels.RoutingRule{}
	if err := r.Decode(&rule, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), err.Error(), envelope.InputError)
	}
	updated, err := app.slack.UpdateRule(id, rule)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(updated)
}

func handleDeleteSlackRule(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, _ := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if id <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	if err := app.slack.DeleteRule(id); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

func handleToggleSlackRule(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, _ := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if id <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	rule, err := app.slack.ToggleRule(id)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(rule)
}

func handleTestSlackChannel(r *fastglue.Request) error {
	app := r.Context.(*App)
	var req struct {
		ChannelID string `json:"channel_id"`
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), err.Error(), envelope.InputError)
	}
	if req.ChannelID == "" {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.Ts("globals.messages.empty", "name", "`channel_id`"), nil))
	}
	if err := app.slack.SendTest(req.ChannelID); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

func handleGetSlackSupportedEvents(r *fastglue.Request) error {
	return r.SendEnvelope(smodels.SupportedEvents)
}
