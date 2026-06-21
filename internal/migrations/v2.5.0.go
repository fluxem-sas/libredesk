package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V2_5_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	stmts := []string{
		`
		CREATE TABLE IF NOT EXISTS applications (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			uuid UUID DEFAULT gen_random_uuid() NOT NULL UNIQUE,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			logo_url TEXT NOT NULL DEFAULT '',
			identity_url TEXT NOT NULL DEFAULT '',
			gateway_app_id TEXT NOT NULL UNIQUE,
			gateway_api_key_hash TEXT NOT NULL,
			enabled BOOL DEFAULT TRUE NOT NULL,
			CONSTRAINT constraint_applications_on_name CHECK (length(name) <= 140),
			CONSTRAINT constraint_applications_on_slug CHECK (length(slug) <= 140),
			CONSTRAINT constraint_applications_on_description CHECK (length(description) <= 300),
			CONSTRAINT constraint_applications_on_logo_url CHECK (length(logo_url) <= 2048),
			CONSTRAINT constraint_applications_on_identity_url CHECK (length(identity_url) <= 2048),
			CONSTRAINT constraint_applications_on_gateway_app_id CHECK (length(gateway_app_id) <= 140),
			CONSTRAINT constraint_applications_on_gateway_api_key_hash CHECK (length(gateway_api_key_hash) <= 255)
		);
		`,
		`
		UPDATE roles
		SET permissions = array_append(permissions, 'applications:manage')
		WHERE name = 'Admin' AND NOT ('applications:manage' = ANY(permissions));
		`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}
