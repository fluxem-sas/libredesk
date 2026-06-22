// Package slack manages Slack workspace integrations and notification routing.
package slack

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"sync"
	"time"

	"github.com/abhinavxd/libredesk/internal/crypto"
	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/integrations/slack/models"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/go-i18n"
	"github.com/lib/pq"
	"github.com/volatiletech/null/v9"
	"github.com/zerodha/logf"
)

var (
	//go:embed queries.sql
	efs embed.FS
)

// DeliveryTask represents an async Slack notification task.
type DeliveryTask struct {
	Event string
	Data  any
}

type oauthState struct {
	UserID    int
	ExpiresAt time.Time
}

type settingsStore interface {
	GetAppRootURL() (string, error)
}

// Manager handles Slack integration operations.
type Manager struct {
	q             queries
	db            *sqlx.DB
	lo            *logf.Logger
	i18n          *i18n.I18n
	setting       settingsStore
	client        *Client
	clientID      string
	clientSecret  string
	encryptionKey string
	deliveryQueue chan DeliveryTask
	workers       int
	oauthStates   map[string]oauthState
	oauthMu       sync.Mutex
	closed        bool
	closedMu      sync.RWMutex
	wg            sync.WaitGroup
}

// Opts contains options for initializing the Manager.
type Opts struct {
	DB            *sqlx.DB
	Lo            *logf.Logger
	I18n          *i18n.I18n
	Setting       settingsStore
	ClientID      string
	ClientSecret  string
	EncryptionKey string
	Workers       int
	QueueSize     int
	Timeout       time.Duration
}

type queries struct {
	GetIntegration        *sqlx.Stmt `query:"get-slack-integration"`
	GetIntegrationByID    *sqlx.Stmt `query:"get-slack-integration-by-id"`
	GetIntegrationByTeam  *sqlx.Stmt `query:"get-slack-integration-by-team"`
	UpsertIntegration     *sqlx.Stmt `query:"upsert-slack-integration"`
	DeleteIntegration     *sqlx.Stmt `query:"delete-slack-integration"`
	ToggleIntegration     *sqlx.Stmt `query:"toggle-slack-integration"`
	GetAllRules           *sqlx.Stmt `query:"get-all-slack-rules"`
	GetRule               *sqlx.Stmt `query:"get-slack-rule"`
	GetActiveRulesByEvent *sqlx.Stmt `query:"get-active-slack-rules-by-event"`
	InsertRule            *sqlx.Stmt `query:"insert-slack-rule"`
	UpdateRule            *sqlx.Stmt `query:"update-slack-rule"`
	DeleteRule            *sqlx.Stmt `query:"delete-slack-rule"`
	ToggleRule            *sqlx.Stmt `query:"toggle-slack-rule"`
	CountRules            *sqlx.Stmt `query:"count-slack-rules"`
}

// New creates a new Slack manager.
func New(opts Opts) (*Manager, error) {
	var q queries
	if err := dbutil.ScanSQLFile("queries.sql", &q, opts.DB, efs); err != nil {
		return nil, err
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = 2
	}
	queueSize := opts.QueueSize
	if queueSize <= 0 {
		queueSize = 256
	}

	return &Manager{
		q:             q,
		db:            opts.DB,
		lo:            opts.Lo,
		i18n:          opts.I18n,
		setting:       opts.Setting,
		client:        NewClient(opts.Timeout),
		clientID:      opts.ClientID,
		clientSecret:  opts.ClientSecret,
		encryptionKey: opts.EncryptionKey,
		deliveryQueue: make(chan DeliveryTask, queueSize),
		workers:       workers,
		oauthStates:   make(map[string]oauthState),
	}, nil
}

// Configured returns true when Slack OAuth credentials are set.
func (m *Manager) Configured() bool {
	return m.clientID != "" && m.clientSecret != ""
}

// OAuthRedirectURI returns the OAuth callback URL.
func (m *Manager) OAuthRedirectURI() (string, error) {
	root, err := m.setting.GetAppRootURL()
	if err != nil {
		return "", err
	}
	return root + "/api/v1/integrations/slack/oauth/callback", nil
}

// StartOAuth generates an OAuth URL for connecting Slack.
func (m *Manager) StartOAuth(userID int) (string, error) {
	if !m.Configured() {
		return "", envelope.NewError(envelope.GeneralError, m.i18n.T("slack.notConfigured"), nil)
	}

	state, err := randomState()
	if err != nil {
		return "", envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	m.oauthMu.Lock()
	m.oauthStates[state] = oauthState{UserID: userID, ExpiresAt: time.Now().Add(10 * time.Minute)}
	m.oauthMu.Unlock()

	redirectURI, err := m.OAuthRedirectURI()
	if err != nil {
		return "", err
	}

	return AuthorizeURL(m.clientID, redirectURI, state), nil
}

// CompleteOAuth exchanges the OAuth code and stores the workspace integration.
func (m *Manager) CompleteOAuth(state, code string) (models.Integration, error) {
	userID, err := m.consumeOAuthState(state)
	if err != nil {
		return models.Integration{}, err
	}

	redirectURI, err := m.OAuthRedirectURI()
	if err != nil {
		return models.Integration{}, err
	}

	resp, err := m.client.ExchangeOAuthCode(context.Background(), m.clientID, m.clientSecret, code, redirectURI)
	if err != nil {
		m.lo.Error("slack oauth exchange failed", "error", err)
		return models.Integration{}, envelope.NewError(envelope.GeneralError, m.i18n.T("slack.oauthFailed"), nil)
	}

	token, err := crypto.Encrypt(resp.AccessToken, m.encryptionKey)
	if err != nil {
		return models.Integration{}, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	var integration models.Integration
	if err := m.q.UpsertIntegration.Get(&integration,
		resp.Team.ID,
		resp.Team.Name,
		token,
		resp.BotUserID,
		null.IntFrom(userID),
	); err != nil {
		m.lo.Error("error saving slack integration", "error", err)
		return integration, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	m.sanitizeIntegration(&integration)
	return integration, nil
}

// GetIntegration returns the active Slack workspace integration.
func (m *Manager) GetIntegration() (models.Integration, error) {
	var integration models.Integration
	if err := m.q.GetIntegration.Get(&integration); err != nil {
		if err == sql.ErrNoRows {
			return integration, nil
		}
		m.lo.Error("error fetching slack integration", "error", err)
		return integration, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	m.sanitizeIntegration(&integration)
	var count int
	if err := m.q.CountRules.Get(&count, integration.ID); err == nil {
		integration.RulesCount = count
	}
	return integration, nil
}

// Disconnect removes the Slack integration and routing rules.
func (m *Manager) Disconnect(id int) error {
	if _, err := m.q.DeleteIntegration.Exec(id); err != nil {
		m.lo.Error("error deleting slack integration", "error", err)
		return envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return nil
}

// ToggleIntegration toggles the integration active flag.
func (m *Manager) ToggleIntegration(id int) (models.Integration, error) {
	var integration models.Integration
	if err := m.q.ToggleIntegration.Get(&integration, id); err != nil {
		if err == sql.ErrNoRows {
			return integration, envelope.NewError(envelope.NotFoundError, m.i18n.T("validation.notFound"), nil)
		}
		m.lo.Error("error toggling slack integration", "error", err)
		return integration, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	m.sanitizeIntegration(&integration)
	return integration, nil
}

// ListChannels returns Slack channels for the connected workspace.
func (m *Manager) ListChannels() ([]models.Channel, error) {
	integration, token, err := m.getActiveIntegrationWithToken()
	if err != nil {
		return nil, err
	}
	_ = integration
	return m.client.ListChannels(context.Background(), token)
}

// GetAllRules returns all routing rules.
func (m *Manager) GetAllRules() ([]models.RoutingRule, error) {
	rules := make([]models.RoutingRule, 0)
	if err := m.q.GetAllRules.Select(&rules); err != nil {
		m.lo.Error("error fetching slack rules", "error", err)
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return rules, nil
}

// GetRule returns a routing rule by ID.
func (m *Manager) GetRule(id int) (models.RoutingRule, error) {
	var rule models.RoutingRule
	if err := m.q.GetRule.Get(&rule, id); err != nil {
		if err == sql.ErrNoRows {
			return rule, envelope.NewError(envelope.NotFoundError, m.i18n.T("validation.notFound"), nil)
		}
		m.lo.Error("error fetching slack rule", "error", err)
		return rule, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return rule, nil
}

// CreateRule creates a routing rule.
func (m *Manager) CreateRule(rule models.RoutingRule) (models.RoutingRule, error) {
	integration, err := m.GetIntegration()
	if err != nil {
		return rule, err
	}
	if integration.ID == 0 {
		return rule, envelope.NewError(envelope.InputError, m.i18n.T("slack.notConnected"), nil)
	}
	if err := validateRule(m, rule); err != nil {
		return rule, err
	}

	var id int
	if err := m.q.InsertRule.Get(&id,
		integration.ID,
		rule.Name,
		rule.InboxID,
		pq.StringArray(rule.Events),
		rule.SlackChannelID,
		rule.SlackChannelName,
		rule.IsActive,
	); err != nil {
		m.lo.Error("error creating slack rule", "error", err)
		return rule, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return m.GetRule(id)
}

// UpdateRule updates a routing rule.
func (m *Manager) UpdateRule(id int, rule models.RoutingRule) (models.RoutingRule, error) {
	if err := validateRule(m, rule); err != nil {
		return rule, err
	}
	if _, err := m.q.UpdateRule.Exec(id,
		rule.Name,
		rule.InboxID,
		pq.StringArray(rule.Events),
		rule.SlackChannelID,
		rule.SlackChannelName,
		rule.IsActive,
	); err != nil {
		m.lo.Error("error updating slack rule", "error", err)
		return rule, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return m.GetRule(id)
}

// DeleteRule deletes a routing rule.
func (m *Manager) DeleteRule(id int) error {
	if _, err := m.q.DeleteRule.Exec(id); err != nil {
		m.lo.Error("error deleting slack rule", "error", err)
		return envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return nil
}

// ToggleRule toggles a routing rule.
func (m *Manager) ToggleRule(id int) (models.RoutingRule, error) {
	var rule models.RoutingRule
	if err := m.q.ToggleRule.Get(&rule, id); err != nil {
		if err == sql.ErrNoRows {
			return rule, envelope.NewError(envelope.NotFoundError, m.i18n.T("validation.notFound"), nil)
		}
		m.lo.Error("error toggling slack rule", "error", err)
		return rule, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return rule, nil
}

// SendTest sends a test message to a Slack channel.
func (m *Manager) SendTest(channelID string) error {
	_, token, err := m.getActiveIntegrationWithToken()
	if err != nil {
		return err
	}
	blocks := []map[string]any{
		{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": "*FluxemDesk × Slack*\nIntegración configurada correctamente.",
			},
		},
	}
	return m.client.PostMessage(context.Background(), token, channelID, blocks, "FluxemDesk Slack test")
}

// TriggerEvent enqueues a Slack notification for matching routing rules.
func (m *Manager) TriggerEvent(event string, data any) {
	m.closedMu.RLock()
	defer m.closedMu.RUnlock()
	if m.closed {
		return
	}

	select {
	case m.deliveryQueue <- DeliveryTask{Event: event, Data: data}:
	default:
		m.lo.Warn("slack delivery queue full, dropping event", "event", event)
	}
}

// Run starts delivery workers.
func (m *Manager) Run(ctx context.Context) {
	for i := 0; i < m.workers; i++ {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.worker(ctx)
		}()
	}
}

// Close stops workers.
func (m *Manager) Close() {
	m.closedMu.Lock()
	defer m.closedMu.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	close(m.deliveryQueue)
	m.wg.Wait()
}

func (m *Manager) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-m.deliveryQueue:
			if !ok {
				return
			}
			m.deliverEvent(ctx, task)
		}
	}
}

func (m *Manager) deliverEvent(ctx context.Context, task DeliveryTask) {
	integration, token, err := m.getActiveIntegrationWithToken()
	if err != nil || integration.ID == 0 {
		return
	}

	rules := make([]models.RoutingRule, 0)
	if err := m.q.GetActiveRulesByEvent.Select(&rules, task.Event); err != nil {
		m.lo.Error("error fetching slack rules for event", "event", task.Event, "error", err)
		return
	}

	inboxID := extractInboxID(task.Data)
	appBaseURL, _ := m.setting.GetAppRootURL()
	blocks, fallback := buildConversationMessage(task.Event, appBaseURL, task.Data)

	for _, rule := range rules {
		if rule.IntegrationID != integration.ID {
			continue
		}
		if rule.InboxID.Valid && inboxID > 0 && int(rule.InboxID.Int) != inboxID {
			continue
		}
		if err := m.client.PostMessage(ctx, token, rule.SlackChannelID, blocks, fallback); err != nil {
			m.lo.Error("slack post message failed",
				"event", task.Event,
				"rule_id", rule.ID,
				"channel", rule.SlackChannelID,
				"error", err,
			)
		}
	}
}

func (m *Manager) getActiveIntegrationWithToken() (models.Integration, string, error) {
	integration, err := m.GetIntegration()
	if err != nil {
		return integration, "", err
	}
	if integration.ID == 0 || !integration.IsActive {
		return integration, "", nil
	}

	var raw models.Integration
	if err := m.q.GetIntegrationByID.Get(&raw, integration.ID); err != nil {
		return integration, "", err
	}
	token, err := crypto.Decrypt(raw.BotToken, m.encryptionKey)
	if err != nil {
		return integration, "", err
	}
	return integration, token, nil
}

func (m *Manager) sanitizeIntegration(integration *models.Integration) {
	integration.BotToken = ""
	integration.Connected = integration.ID > 0 && integration.IsActive
}

func (m *Manager) consumeOAuthState(state string) (int, error) {
	m.oauthMu.Lock()
	defer m.oauthMu.Unlock()

	entry, ok := m.oauthStates[state]
	delete(m.oauthStates, state)
	if !ok || time.Now().After(entry.ExpiresAt) {
		return 0, envelope.NewError(envelope.InputError, m.i18n.T("slack.oauthStateInvalid"), nil)
	}
	return entry.UserID, nil
}

func validateRule(m *Manager, rule models.RoutingRule) error {
	if rule.Name == "" {
		return envelope.NewError(envelope.InputError, m.i18n.Ts("globals.messages.empty", "name", "`name`"), nil)
	}
	if rule.SlackChannelID == "" {
		return envelope.NewError(envelope.InputError, m.i18n.Ts("globals.messages.empty", "name", "`slack_channel_id`"), nil)
	}
	if len(rule.Events) == 0 {
		return envelope.NewError(envelope.InputError, m.i18n.Ts("globals.messages.empty", "name", "`events`"), nil)
	}
	for _, ev := range rule.Events {
		if !isSupportedEvent(ev) {
			return envelope.NewError(envelope.InputError, m.i18n.T("slack.invalidEvent"), nil)
		}
	}
	return nil
}

func isSupportedEvent(event string) bool {
	for _, ev := range models.SupportedEvents {
		if ev == event {
			return true
		}
	}
	return false
}

func extractInboxID(data any) int {
	conv, _ := extractConversation(data)
	if conv != nil {
		return conv.InboxID
	}
	return 0
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
