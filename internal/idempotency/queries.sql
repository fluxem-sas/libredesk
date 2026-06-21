-- name: get-idempotency-response
SELECT response_conversation_uuid FROM gateway_idempotency
WHERE application_id = $1 AND idempotency_key = $2;

-- name: insert-idempotency-response
INSERT INTO gateway_idempotency (application_id, idempotency_key, request_path, response_conversation_uuid)
VALUES ($1, $2, $3, $4)
ON CONFLICT (application_id, idempotency_key) DO NOTHING;

-- name: delete-old-idempotency-keys
DELETE FROM gateway_idempotency WHERE created_at < NOW() - ($1 || ' seconds')::interval;
