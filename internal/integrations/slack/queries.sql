-- name: get-slack-integration
SELECT
    id,
    created_at,
    updated_at,
    team_id,
    team_name,
    bot_token,
    bot_user_id,
    installed_by,
    is_active
FROM
    slack_integrations
ORDER BY created_at DESC
LIMIT 1;

-- name: get-slack-integration-by-id
SELECT
    id,
    created_at,
    updated_at,
    team_id,
    team_name,
    bot_token,
    bot_user_id,
    installed_by,
    is_active
FROM
    slack_integrations
WHERE
    id = $1;

-- name: get-slack-integration-by-team
SELECT
    id,
    created_at,
    updated_at,
    team_id,
    team_name,
    bot_token,
    bot_user_id,
    installed_by,
    is_active
FROM
    slack_integrations
WHERE
    team_id = $1;

-- name: upsert-slack-integration
INSERT INTO slack_integrations (team_id, team_name, bot_token, bot_user_id, installed_by, is_active)
VALUES ($1, $2, $3, $4, $5, true)
ON CONFLICT (team_id) DO UPDATE SET
    team_name = EXCLUDED.team_name,
    bot_token = EXCLUDED.bot_token,
    bot_user_id = EXCLUDED.bot_user_id,
    installed_by = EXCLUDED.installed_by,
    is_active = true,
    updated_at = NOW()
RETURNING *;

-- name: delete-slack-integration
DELETE FROM slack_integrations WHERE id = $1;

-- name: toggle-slack-integration
UPDATE slack_integrations
SET is_active = NOT is_active, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: get-all-slack-rules
SELECT
    r.id,
    r.created_at,
    r.updated_at,
    r.integration_id,
    r.name,
    r.inbox_id,
    i.name AS inbox_name,
    r.events,
    r.slack_channel_id,
    r.slack_channel_name,
    r.is_active
FROM
    slack_routing_rules r
    LEFT JOIN inboxes i ON i.id = r.inbox_id
ORDER BY r.created_at DESC;

-- name: get-slack-rule
SELECT
    r.id,
    r.created_at,
    r.updated_at,
    r.integration_id,
    r.name,
    r.inbox_id,
    i.name AS inbox_name,
    r.events,
    r.slack_channel_id,
    r.slack_channel_name,
    r.is_active
FROM
    slack_routing_rules r
    LEFT JOIN inboxes i ON i.id = r.inbox_id
WHERE
    r.id = $1;

-- name: get-active-slack-rules-by-event
SELECT
    r.id,
    r.created_at,
    r.updated_at,
    r.integration_id,
    r.name,
    r.inbox_id,
    i.name AS inbox_name,
    r.events,
    r.slack_channel_id,
    r.slack_channel_name,
    r.is_active
FROM
    slack_routing_rules r
    LEFT JOIN inboxes i ON i.id = r.inbox_id
    JOIN slack_integrations s ON s.id = r.integration_id
WHERE
    r.is_active = true
    AND s.is_active = true
    AND $1 = ANY(r.events);

-- name: insert-slack-rule
INSERT INTO slack_routing_rules (
    integration_id, name, inbox_id, events, slack_channel_id, slack_channel_name, is_active
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: update-slack-rule
UPDATE slack_routing_rules
SET
    name = $2,
    inbox_id = $3,
    events = $4,
    slack_channel_id = $5,
    slack_channel_name = $6,
    is_active = $7,
    updated_at = NOW()
WHERE id = $1;

-- name: delete-slack-rule
DELETE FROM slack_routing_rules WHERE id = $1;

-- name: toggle-slack-rule
UPDATE slack_routing_rules
SET is_active = NOT is_active, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: count-slack-rules
SELECT COUNT(*) FROM slack_routing_rules WHERE integration_id = $1;
