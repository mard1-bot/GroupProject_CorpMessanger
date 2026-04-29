-- XMPP reconciliation table for tracking chats that need XMPP sync
CREATE TABLE IF NOT EXISTS xmpp_reconciliation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    reason TEXT NOT NULL, -- 'room_not_created', 'member_not_added', etc.
    details JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    attempts INT DEFAULT 0,
    max_attempts INT DEFAULT 5,
    last_attempt_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(chat_id, reason, resolved_at)
);

CREATE INDEX IF NOT EXISTS idx_xmpp_reconciliation_chat_id ON xmpp_reconciliation(chat_id);
CREATE INDEX IF NOT EXISTS idx_xmpp_reconciliation_resolved_at ON xmpp_reconciliation(resolved_at) WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_xmpp_reconciliation_created_at ON xmpp_reconciliation(created_at);
