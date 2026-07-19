-- Notification channels route the domain events that already fire (deployment,
-- database backup/restore, TLS status) out to third-party providers. The in-app
-- bell (notifications table) is unchanged; this is a delivery-routing layer that
-- fans a persisted notification out to any enabled channel subscribed to its type.
--
-- config_encrypted holds the whole provider config JSON (webhook URLs, bot
-- tokens, recipients...) keyring-encrypted, mirroring backup_destinations —
-- every value in it is potentially a secret, so reads are masked to presence only
-- and edits replace the whole blob.

CREATE TABLE notification_channels (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL UNIQUE,
    type             TEXT NOT NULL
        CHECK (type IN ('discord', 'telegram', 'slack', 'webhook', 'ntfy', 'gotify', 'email')),
    config_encrypted BYTEA NOT NULL,
    -- Subscribed event types (e.g. 'deployment.failed'); a channel receives an
    -- event only when its type is present here.
    events           TEXT[] NOT NULL DEFAULT '{}',
    enabled          BOOLEAN NOT NULL DEFAULT true,
    -- Delivery state, surfaced on the channel row in the UI.
    last_sent_at     TIMESTAMPTZ,
    last_error       TEXT,
    created_by       UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Dispatch loads enabled channels whose events array contains the fired type;
-- a GIN index over events keeps that membership test cheap as channels grow.
CREATE INDEX idx_notification_channels_events ON notification_channels USING GIN (events);
