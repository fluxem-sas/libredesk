package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V2_8_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS gateway_idempotency (
			id SERIAL PRIMARY KEY,
			application_id INTEGER NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			idempotency_key TEXT NOT NULL,
			request_path TEXT NOT NULL DEFAULT '',
			response_conversation_uuid TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(application_id, idempotency_key)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_gateway_idempotency_lookup ON gateway_idempotency(application_id, idempotency_key);`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}
