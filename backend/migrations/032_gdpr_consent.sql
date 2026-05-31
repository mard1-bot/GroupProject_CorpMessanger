-- GDPR and 152-ФЗ compliance: User consent and data management

-- User consent tracking
CREATE TABLE IF NOT EXISTS user_consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    consent_type VARCHAR(50) NOT NULL, -- 'data_processing', 'marketing', 'analytics'
    consent_given BOOLEAN NOT NULL DEFAULT false,
    consent_text TEXT NOT NULL, -- The exact text user agreed to
    consent_version VARCHAR(20) NOT NULL, -- Version of consent text
    ip_address VARCHAR(45), -- IP address when consent was given
    user_agent TEXT, -- Browser/app info
    consented_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, consent_type)
);

CREATE INDEX IF NOT EXISTS idx_user_consents_user_id ON user_consents(user_id);
CREATE INDEX IF NOT EXISTS idx_user_consents_type ON user_consents(consent_type);
CREATE INDEX IF NOT EXISTS idx_user_consents_given ON user_consents(consent_given);

-- Data export requests (GDPR Article 15 - Right of Access)
CREATE TABLE IF NOT EXISTS data_export_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    request_type VARCHAR(20) NOT NULL, -- 'export', 'delete'
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'
    requested_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    export_url TEXT, -- S3/storage URL for download
    expires_at TIMESTAMP WITH TIME ZONE, -- Export link expiration
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_data_export_user_id ON data_export_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_data_export_status ON data_export_requests(status);

-- Data deletion log (for compliance audit trail)
CREATE TABLE IF NOT EXISTS data_deletion_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL, -- Not a foreign key since user may be deleted
    user_email VARCHAR(255) NOT NULL,
    deletion_type VARCHAR(20) NOT NULL, -- 'full', 'partial', 'anonymization'
    tables_affected TEXT[], -- Array of table names
    records_deleted JSONB, -- {"messages": 150, "files": 20, etc}
    requested_by UUID, -- Admin or user who requested
    reason TEXT,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_data_deletion_user_id ON data_deletion_log(user_id);
CREATE INDEX IF NOT EXISTS idx_data_deletion_date ON data_deletion_log(deleted_at);

-- Add consent fields to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS data_processing_consent BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS consent_date TIMESTAMP WITH TIME ZONE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS data_retention_until TIMESTAMP WITH TIME ZONE;

-- Function to check if user has valid consent
CREATE OR REPLACE FUNCTION has_valid_consent(p_user_id UUID, p_consent_type VARCHAR(50))
RETURNS BOOLEAN AS $$
DECLARE
    v_consent_given BOOLEAN;
    v_revoked_at TIMESTAMP WITH TIME ZONE;
BEGIN
    SELECT consent_given, revoked_at
    INTO v_consent_given, v_revoked_at
    FROM user_consents
    WHERE user_id = p_user_id AND consent_type = p_consent_type;

    -- No consent record found
    IF NOT FOUND THEN
        RETURN false;
    END IF;

    -- Consent was revoked
    IF v_revoked_at IS NOT NULL THEN
        RETURN false;
    END IF;

    RETURN v_consent_given;
END;
$$ LANGUAGE plpgsql;

-- Function to anonymize user data (GDPR Article 17 - Right to Erasure)
CREATE OR REPLACE FUNCTION anonymize_user_data(p_user_id UUID)
RETURNS JSONB AS $$
DECLARE
    v_records_affected JSONB := '{}';
    v_message_count INT;
    v_file_count INT;
BEGIN
    -- Anonymize messages (keep for chat history but remove personal identifiers)
    UPDATE messages
    SET 
        content = '[Deleted User Message]',
        updated_at = NOW()
    WHERE sender_id = p_user_id
    RETURNING COUNT(*) INTO v_message_count;
    
    v_records_affected := jsonb_set(v_records_affected, '{messages}', to_jsonb(v_message_count));

    -- Delete files
    DELETE FROM files WHERE uploader_id = p_user_id;
    GET DIAGNOSTICS v_file_count = ROW_COUNT;
    v_records_affected := jsonb_set(v_records_affected, '{files}', to_jsonb(v_file_count));

    -- Anonymize user record
    UPDATE users
    SET
        email = 'deleted_' || id || '@anonymized.local',
        phone = NULL,
        first_name = 'Deleted',
        last_name = 'User',
        middle_name = NULL,
        avatar = NULL,
        status = 'deleted',
        updated_at = NOW()
    WHERE id = p_user_id;

    RETURN v_records_affected;
END;
$$ LANGUAGE plpgsql;
