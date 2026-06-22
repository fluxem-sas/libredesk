-- name: get-all-applications
SELECT id, uuid, created_at, updated_at, name, slug, description, logo_url, identity_url, gateway_app_id, gateway_api_key_hash, enabled
FROM applications
ORDER BY updated_at DESC, id DESC;

-- name: get-application
SELECT id, uuid, created_at, updated_at, name, slug, description, logo_url, identity_url, gateway_app_id, gateway_api_key_hash, enabled
FROM applications
WHERE id = $1;

-- name: get-application-by-gateway-app-id
SELECT id, uuid, created_at, updated_at, name, slug, description, logo_url, identity_url, gateway_app_id, gateway_api_key_hash, enabled
FROM applications
WHERE gateway_app_id = $1 AND enabled = TRUE;

-- name: get-application-api-key-hash
SELECT gateway_api_key_hash
FROM applications
WHERE id = $1;

-- name: insert-application
INSERT INTO applications (name, slug, description, logo_url, identity_url, gateway_app_id, gateway_api_key_hash, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: update-application
UPDATE applications
SET
	name = $2,
	slug = $3,
	description = $4,
	logo_url = $5,
	identity_url = $6,
	gateway_app_id = $7,
	gateway_api_key_hash = $8,
	enabled = $9,
	updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: regenerate-application-api-key
UPDATE applications
SET gateway_api_key_hash = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: delete-application
DELETE FROM applications
WHERE id = $1;

-- name: toggle-application
UPDATE applications
SET enabled = NOT enabled, updated_at = NOW()
WHERE id = $1
RETURNING *;
