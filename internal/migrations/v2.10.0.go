package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V2_10_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS slack_integrations (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			team_id TEXT NOT NULL UNIQUE,
			team_name TEXT NOT NULL DEFAULT '',
			bot_token TEXT NOT NULL DEFAULT '',
			bot_user_id TEXT NOT NULL DEFAULT '',
			installed_by INT REFERENCES users(id) ON DELETE SET NULL,
			is_active BOOLEAN DEFAULT true,
			CONSTRAINT constraint_slack_integrations_on_team_id CHECK (length(team_id) <= 64),
			CONSTRAINT constraint_slack_integrations_on_team_name CHECK (length(team_name) <= 255)
		);`,
		`CREATE TABLE IF NOT EXISTS slack_routing_rules (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			integration_id INT NOT NULL REFERENCES slack_integrations(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			inbox_id INT REFERENCES inboxes(id) ON DELETE CASCADE,
			events TEXT[] NOT NULL DEFAULT '{}',
			slack_channel_id TEXT NOT NULL,
			slack_channel_name TEXT NOT NULL DEFAULT '',
			is_active BOOLEAN DEFAULT true,
			CONSTRAINT constraint_slack_routing_rules_on_name CHECK (length(name) <= 255),
			CONSTRAINT constraint_slack_routing_rules_on_channel CHECK (length(slack_channel_id) <= 64),
			CONSTRAINT constraint_slack_routing_rules_on_events_not_empty CHECK (array_length(events, 1) > 0)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_slack_routing_rules_integration ON slack_routing_rules(integration_id);`,
		`CREATE INDEX IF NOT EXISTS idx_slack_routing_rules_inbox ON slack_routing_rules(inbox_id);`,
		`UPDATE roles
		 SET permissions = array_append(permissions, 'slack:manage')
		 WHERE name = 'Admin' AND NOT ('slack:manage' = ANY(permissions));`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}
