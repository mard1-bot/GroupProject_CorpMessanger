-- XMPP sync dead letter queue for tracking failed message syncs
CREATE TABLE IF NOT EXISTS xmpp_sync_dead_letter (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    error_message TEXT NOT NULL,
    error_details JSONB,
    attempts INT DEFAULT 1,
    max_attempts INT DEFAULT 5,
    last_attempt_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(message_id)
);

CREATE INDEX IF NOT EXISTS idx_xmpp_sync_dead_letter_message_id ON xmpp_sync_dead_letter(message_id);
CREATE INDEX IF NOT EXISTS idx_xmpp_sync_dead_letter_chat_id ON xmpp_sync_dead_letter(chat_id);
CREATE INDEX IF NOT EXISTS idx_xmpp_sync_dead_letter_resolved_at ON xmpp_sync_dead_letter(resolved_at) WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_xmpp_sync_dead_letter_created_at ON xmpp_sync_dead_letter(created_at);
