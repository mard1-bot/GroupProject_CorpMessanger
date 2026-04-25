-- Add E2E encryption support
CREATE TABLE IF NOT EXISTS encryption_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE UNIQUE,
    public_key TEXT NOT NULL,
    private_key TEXT,
    key_version INT DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_encryption_keys_user_id ON encryption_keys(user_id);

-- Add encryption fields to messages
ALTER TABLE messages ADD COLUMN IF NOT EXISTS encrypted_content TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS encryption_key_id UUID;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS encrypted_keys JSONB;

CREATE INDEX IF NOT EXISTS idx_messages_encryption_key_id ON messages(encryption_key_id);
