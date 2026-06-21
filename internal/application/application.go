package application

import (
	"database/sql"
	"embed"
	"strings"

	"github.com/abhinavxd/libredesk/internal/application/models"
	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/go-i18n"
	"github.com/zerodha/logf"
	"golang.org/x/crypto/bcrypt"
)

var (
	//go:embed queries.sql
	efs embed.FS
)

type Manager struct {
	q    queries
	lo   *logf.Logger
	i18n *i18n.I18n
}

type Opts struct {
	DB   *sqlx.DB
	Lo   *logf.Logger
	I18n *i18n.I18n
}

type queries struct {
	GetAllApplications       *sqlx.Stmt `query:"get-all-applications"`
	GetApplication           *sqlx.Stmt `query:"get-application"`
	GetApplicationByAppID    *sqlx.Stmt `query:"get-application-by-gateway-app-id"`
	GetApplicationAPIKeyHash *sqlx.Stmt `query:"get-application-api-key-hash"`
	InsertApplication        *sqlx.Stmt `query:"insert-application"`
	UpdateApplication        *sqlx.Stmt `query:"update-application"`
	DeleteApplication        *sqlx.Stmt `query:"delete-application"`
	ToggleApplication        *sqlx.Stmt `query:"toggle-application"`
}

func New(opts Opts) (*Manager, error) {
	var q queries
	if err := dbutil.ScanSQLFile("queries.sql", &q, opts.DB, efs); err != nil {
		return nil, err
	}
	return &Manager{
		q:    q,
		lo:   opts.Lo,
		i18n: opts.I18n,
	}, nil
}

func (m *Manager) GetAll() ([]models.Application, error) {
	var out = make([]models.Application, 0)
	if err := m.q.GetAllApplications.Select(&out); err != nil {
		m.lo.Error("error fetching applications", "error", err)
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	for i := range out {
		out[i].ClearSecrets()
	}
	return out, nil
}

func (m *Manager) Get(id int) (models.Application, error) {
	var out models.Application
	if err := m.q.GetApplication.Get(&out, id); err != nil {
		if err == sql.ErrNoRows {
			return out, envelope.NewError(envelope.NotFoundError, m.i18n.T("globals.messages.notFound"), nil)
		}
		m.lo.Error("error fetching application", "id", id, "error", err)
		return out, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	out.ClearSecrets()
	return out, nil
}

func (m *Manager) Create(app models.Application) (models.Application, error) {
	var out models.Application
	normalized, hashedKey, err := m.normalizeForWrite(app, 0, false)
	if err != nil {
		return out, err
	}

	if err := m.q.InsertApplication.Get(
		&out,
		normalized.Name,
		normalized.Slug,
		normalized.Description,
		normalized.LogoURL,
		normalized.IdentityURL,
		normalized.GatewayAppID,
		hashedKey,
		normalized.Enabled,
	); err != nil {
		if dbutil.IsUniqueViolationError(err) {
			return out, envelope.NewError(envelope.ConflictError, m.i18n.T("globals.messages.errorAlreadyExists"), nil)
		}
		m.lo.Error("error inserting application", "error", err)
		return out, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	out.ClearSecrets()
	return out, nil
}

func (m *Manager) Update(id int, app models.Application) (models.Application, error) {
	var out models.Application
	normalized, hashedKey, err := m.normalizeForWrite(app, id, true)
	if err != nil {
		return out, err
	}

	if err := m.q.UpdateApplication.Get(
		&out,
		id,
		normalized.Name,
		normalized.Slug,
		normalized.Description,
		normalized.LogoURL,
		normalized.IdentityURL,
		normalized.GatewayAppID,
		hashedKey,
		normalized.Enabled,
	); err != nil {
		if dbutil.IsUniqueViolationError(err) {
			return out, envelope.NewError(envelope.ConflictError, m.i18n.T("globals.messages.errorAlreadyExists"), nil)
		}
		m.lo.Error("error updating application", "id", id, "error", err)
		return out, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	out.ClearSecrets()
	return out, nil
}

func (m *Manager) Delete(id int) error {
	if _, err := m.q.DeleteApplication.Exec(id); err != nil {
		m.lo.Error("error deleting application", "id", id, "error", err)
		return envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return nil
}

func (m *Manager) Toggle(id int) (models.Application, error) {
	var out models.Application
	if err := m.q.ToggleApplication.Get(&out, id); err != nil {
		m.lo.Error("error toggling application", "id", id, "error", err)
		return out, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	out.ClearSecrets()
	return out, nil
}

func (m *Manager) ValidateGatewayAPIKey(gatewayAppID, apiKey string) (models.Application, error) {
	var app models.Application
	if err := m.q.GetApplicationByAppID.Get(&app, gatewayAppID); err != nil {
		if err == sql.ErrNoRows {
			return app, envelope.NewError(envelope.UnauthorizedError, m.i18n.T("validation.invalidCredential"), nil)
		}
		m.lo.Error("error fetching application by gateway app id", "gateway_app_id", gatewayAppID, "error", err)
		return app, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(app.GatewayAPIKeyHash), []byte(apiKey)); err != nil {
		return app, envelope.NewError(envelope.UnauthorizedError, m.i18n.T("validation.invalidCredential"), nil)
	}
	app.ClearSecrets()
	return app, nil
}

func (m *Manager) normalizeForWrite(app models.Application, id int, isUpdate bool) (models.Application, string, error) {
	app.Name = strings.TrimSpace(app.Name)
	app.Slug = normalizeSlug(app.Slug)
	app.Description = strings.TrimSpace(app.Description)
	app.LogoURL = strings.TrimSpace(app.LogoURL)
	app.IdentityURL = strings.TrimSpace(app.IdentityURL)
	app.GatewayAppID = strings.TrimSpace(app.GatewayAppID)

	if app.Name == "" || app.Slug == "" {
		return app, "", envelope.NewError(envelope.InputError, m.i18n.T("globals.messages.required"), nil)
	}

	if app.GatewayAppID == "" {
		randomID, err := stringutil.RandomAlphanumeric(24)
		if err != nil {
			m.lo.Error("error generating gateway app id", "error", err)
			return app, "", envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		app.GatewayAppID = strings.ToLower(randomID)
	}

	if !isUpdate && app.GatewayAPIKey == "" {
		return app, "", envelope.NewError(envelope.InputError, m.i18n.T("globals.messages.required"), nil)
	}

	if isUpdate && strings.Contains(app.GatewayAPIKey, stringutil.PasswordDummy) {
		var existingHash string
		if err := m.q.GetApplicationAPIKeyHash.Get(&existingHash, id); err != nil {
			m.lo.Error("error fetching existing application api key hash", "id", id, "error", err)
			return app, "", envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		return app, existingHash, nil
	}

	if isUpdate && app.GatewayAPIKey == "" {
		var existingHash string
		if err := m.q.GetApplicationAPIKeyHash.Get(&existingHash, id); err != nil {
			m.lo.Error("error fetching existing application api key hash", "id", id, "error", err)
			return app, "", envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		return app, existingHash, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(app.GatewayAPIKey), bcrypt.DefaultCost)
	if err != nil {
		m.lo.Error("error hashing application api key", "error", err)
		return app, "", envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return app, string(hash), nil
}

func normalizeSlug(in string) string {
	in = strings.TrimSpace(strings.ToLower(in))
	if in == "" {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range in {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	return out
}
