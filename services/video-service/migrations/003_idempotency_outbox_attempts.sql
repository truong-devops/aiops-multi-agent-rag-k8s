ALTER TABLE upload_requests
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS upload_requests_owner_idempotency_key_unique
    ON upload_requests (owner_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0;

ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS last_error TEXT;
