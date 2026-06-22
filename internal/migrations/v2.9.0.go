package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V2_9_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_ticket_inbox_per_application
		ON inboxes(application_id)
		WHERE deleted_at IS NULL
		  AND application_id IS NOT NULL
		  AND channel = 'ticket';`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}
