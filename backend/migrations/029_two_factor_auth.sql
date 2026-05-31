-- Two Factor Authentication (2FA) settings table
CREATE TABLE IF NOT EXISTS two_factor_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret TEXT NOT NULL,
    backup_codes TEXT[] NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Index for faster lookups
CREATE INDEX IF NOT EXISTS idx_two_factor_settings_user_id 
ON two_factor_settings(user_id);

CREATE INDEX IF NOT EXISTS idx_two_factor_settings_enabled 
ON two_factor_settings(enabled) WHERE enabled = true;

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_two_factor_settings_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_two_factor_settings_updated_at
    BEFORE UPDATE ON two_factor_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_two_factor_settings_updated_at();
