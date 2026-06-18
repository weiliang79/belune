-- Per-recipient notification rows. Each row is one user's copy of an event, so
-- read-state is a simple boolean with no join table (decided default for v0.0.16).
-- Notifications are a sibling of audit_logs, not chained off them: audit records
-- "who did what" (immutable, mostly non-user-facing), notifications are the
-- user-facing feed and some have no actor (system events).
CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type       TEXT NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    link       TEXT,
    read       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Feed query: most-recent-first per user.
CREATE INDEX idx_notifications_user_created ON notifications (user_id, created_at DESC);

-- Unread-count query: partial index keeps it cheap as read rows accumulate.
CREATE INDEX idx_notifications_user_unread ON notifications (user_id) WHERE read = FALSE;
