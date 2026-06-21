package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V2_6_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	stmts := []string{
		`
		ALTER TABLE inboxes
		ADD COLUMN IF NOT EXISTS application_id INT REFERENCES applications(id) ON DELETE SET NULL ON UPDATE CASCADE;
		`,
		`
		CREATE INDEX IF NOT EXISTS index_inboxes_on_application_id ON inboxes (application_id);
		`,
		`
		ALTER TABLE conversations
		ADD COLUMN IF NOT EXISTS application_id INT REFERENCES applications(id) ON DELETE SET NULL ON UPDATE CASCADE;
		`,
		`
		UPDATE conversations c
		SET application_id = i.application_id
		FROM inboxes i
		WHERE c.inbox_id = i.id
		  AND c.application_id IS NULL;
		`,
		`
		CREATE INDEX IF NOT EXISTS index_conversations_on_application_id ON conversations (application_id);
		`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}
