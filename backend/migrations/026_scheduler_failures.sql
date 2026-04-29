-- Scheduler failures table for tracking failed scheduled message deliveries
CREATE TABLE IF NOT EXISTS scheduler_failures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    error_message TEXT NOT NULL,
    error_details JSONB,
    attempts INT DEFAULT 1,
    max_attempts INT DEFAULT 5,
    next_retry_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() + INTERVAL '1 minute',
    last_attempt_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(message_id)
);

CREATE INDEX IF NOT EXISTS idx_scheduler_failures_message_id ON scheduler_failures(message_id);
CREATE INDEX IF NOT EXISTS idx_scheduler_failures_chat_id ON scheduler_failures(chat_id);
CREATE INDEX IF NOT EXISTS idx_scheduler_failures_next_retry_at ON scheduler_failures(next_retry_at) WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_scheduler_failures_resolved_at ON scheduler_failures(resolved_at) WHERE resolved_at IS NULL;
