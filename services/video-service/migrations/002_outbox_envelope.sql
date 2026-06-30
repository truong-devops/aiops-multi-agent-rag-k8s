ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS producer TEXT NOT NULL DEFAULT 'video-service';

ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS environment TEXT NOT NULL DEFAULT 'local';

ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS occurred_at TIMESTAMPTZ;

UPDATE outbox_events
SET occurred_at = created_at
WHERE occurred_at IS NULL;

ALTER TABLE outbox_events
    ALTER COLUMN occurred_at SET NOT NULL;

CREATE INDEX IF NOT EXISTS outbox_events_name_version_status_idx
    ON outbox_events (event_name, event_version, status, created_at);
