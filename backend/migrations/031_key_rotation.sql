-- Key rotation support for encryption keys
-- Archive table for old encryption keys
CREATE TABLE IF NOT EXISTS encryption_keys_archive (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    public_key TEXT NOT NULL,
    private_key TEXT,
    key_version INT NOT NULL,
    archived_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_encryption_keys_archive_user_id ON encryption_keys_archive(user_id);
CREATE INDEX IF NOT EXISTS idx_encryption_keys_archive_version ON encryption_keys_archive(user_id, key_version);

-- Add rotation metadata to encryption_keys
ALTER TABLE encryption_keys ADD COLUMN IF NOT EXISTS rotated_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE encryption_keys ADD COLUMN IF NOT EXISTS rotation_reason VARCHAR(100);

-- Function to rotate encryption key
CREATE OR REPLACE FUNCTION rotate_encryption_key(
    p_user_id UUID,
    p_new_public_key TEXT,
    p_new_private_key TEXT,
    p_rotation_reason VARCHAR(100)
) RETURNS TABLE(new_version INT, archived_id UUID) AS $$
DECLARE
    v_old_version INT;
    v_new_version INT;
    v_archived_id UUID;
BEGIN
    -- Get current version
    SELECT key_version INTO v_old_version
    FROM encryption_keys
    WHERE user_id = p_user_id;

    IF v_old_version IS NULL THEN
        RAISE EXCEPTION 'No encryption key found for user %', p_user_id;
    END IF;

    -- Archive old key
    INSERT INTO encryption_keys_archive (
        user_id, public_key, private_key, key_version, created_at, updated_at
    )
    SELECT user_id, public_key, private_key, key_version, created_at, updated_at
    FROM encryption_keys
    WHERE user_id = p_user_id
    RETURNING id INTO v_archived_id;

    -- Calculate new version
    v_new_version := v_old_version + 1;

    -- Update with new key
    UPDATE encryption_keys
    SET 
        public_key = p_new_public_key,
        private_key = p_new_private_key,
        key_version = v_new_version,
        rotated_at = NOW(),
        rotation_reason = p_rotation_reason,
        updated_at = NOW()
    WHERE user_id = p_user_id;

    RETURN QUERY SELECT v_new_version, v_archived_id;
END;
$$ LANGUAGE plpgsql;

-- Function to get key by version (for decrypting old messages)
CREATE OR REPLACE FUNCTION get_encryption_key_by_version(
    p_user_id UUID,
    p_version INT
) RETURNS TABLE(public_key TEXT, private_key TEXT, key_version INT) AS $$
BEGIN
    -- Check current key first
    IF EXISTS (
        SELECT 1 FROM encryption_keys 
        WHERE user_id = p_user_id AND key_version = p_version
    ) THEN
        RETURN QUERY 
        SELECT e.public_key, e.private_key, e.key_version
        FROM encryption_keys e
        WHERE e.user_id = p_user_id AND e.key_version = p_version;
    ELSE
        -- Check archive
        RETURN QUERY 
        SELECT a.public_key, a.private_key, a.key_version
        FROM encryption_keys_archive a
        WHERE a.user_id = p_user_id AND a.key_version = p_version;
    END IF;
END;
$$ LANGUAGE plpgsql;
