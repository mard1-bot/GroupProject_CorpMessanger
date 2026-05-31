-- Move password_hash from users table to user_credentials table

-- Create user_credentials table if not exists
CREATE TABLE IF NOT EXISTS user_credentials (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_hash VARCHAR(255) NOT NULL
);

-- Add missing columns if they don't exist
ALTER TABLE user_credentials ADD COLUMN IF NOT EXISTS created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();
ALTER TABLE user_credentials ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

-- Migrate existing passwords from users table to user_credentials
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'password_hash') THEN
        INSERT INTO user_credentials (user_id, password_hash)
        SELECT id, password_hash FROM users
        WHERE password_hash IS NOT NULL
        ON CONFLICT (user_id) DO NOTHING;
    END IF;
END $$;

-- Drop password_hash column from users table (no longer needed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'password_hash') THEN
        ALTER TABLE users DROP COLUMN password_hash;
    END IF;
END $$;
