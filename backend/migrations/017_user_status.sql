-- Add user status support
ALTER TABLE users ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'online';
ALTER TABLE users ADD COLUMN IF NOT EXISTS custom_status TEXT;

CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
