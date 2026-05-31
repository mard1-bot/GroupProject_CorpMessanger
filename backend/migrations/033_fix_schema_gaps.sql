-- Add missing columns to chats table
ALTER TABLE chats ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE chats ADD COLUMN IF NOT EXISTS avatar VARCHAR(500);

-- Add location JSONB column to messages if it doesn't exist
ALTER TABLE messages ADD COLUMN IF NOT EXISTS location JSONB;

-- Migrate data from old latitude/longitude/location_address columns if they exist
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'messages' AND column_name = 'latitude') THEN
        -- Copy data to JSONB
        UPDATE messages
        SET location = jsonb_build_object(
            'latitude', latitude,
            'longitude', longitude,
            'address', COALESCE(location_address, '')
        )
        WHERE latitude IS NOT NULL AND longitude IS NOT NULL AND location IS NULL;

        -- Drop old columns
        ALTER TABLE messages DROP COLUMN IF EXISTS latitude;
        ALTER TABLE messages DROP COLUMN IF EXISTS longitude;
        ALTER TABLE messages DROP COLUMN IF EXISTS location_address;
        DROP INDEX IF EXISTS idx_messages_location;
    END IF;
END $$;

-- Add consent fields to users if missing
ALTER TABLE users ADD COLUMN IF NOT EXISTS data_processing_consent BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS consent_date TIMESTAMP WITH TIME ZONE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS data_retention_until TIMESTAMP WITH TIME ZONE;
