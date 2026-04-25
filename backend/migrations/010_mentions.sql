-- Add mentions support
CREATE TABLE IF NOT EXISTS mentions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    mentioned_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(message_id, mentioned_user_id)
);

CREATE INDEX IF NOT EXISTS idx_mentions_message_id ON mentions(message_id);
CREATE INDEX IF NOT EXISTS idx_mentions_user_id ON mentions(mentioned_user_id);
