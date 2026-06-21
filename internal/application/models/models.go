package models

import (
	"strings"
	"time"

	"github.com/abhinavxd/libredesk/internal/stringutil"
)

// Application represents a support application configuration.
type Application struct {
	ID                int       `db:"id" json:"id"`
	UUID              string    `db:"uuid" json:"uuid"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
	Name              string    `db:"name" json:"name"`
	Slug              string    `db:"slug" json:"slug"`
	Description       string    `db:"description" json:"description"`
	LogoURL           string    `db:"logo_url" json:"logo_url"`
	IdentityURL       string    `db:"identity_url" json:"identity_url"`
	GatewayAppID      string    `db:"gateway_app_id" json:"gateway_app_id"`
	GatewayAPIKeyHash string    `db:"gateway_api_key_hash" json:"-"`
	GatewayAPIKey     string    `db:"-" json:"gateway_api_key,omitempty"`
	Enabled           bool      `db:"enabled" json:"enabled"`
	HasGatewayAPIKey  bool      `db:"-" json:"has_gateway_api_key"`
}

// ClearSecrets masks sensitive fields with dummy values for API responses.
func (a *Application) ClearSecrets() {
	a.HasGatewayAPIKey = a.GatewayAPIKeyHash != ""
	if a.HasGatewayAPIKey {
		a.GatewayAPIKey = strings.Repeat(stringutil.PasswordDummy, 10)
	}
}
