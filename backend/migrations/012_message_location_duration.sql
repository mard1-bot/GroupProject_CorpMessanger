-- Add location fields to messages
ALTER TABLE messages ADD COLUMN IF NOT EXISTS latitude FLOAT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS longitude FLOAT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS location_address TEXT;

-- Add duration field for voice messages
ALTER TABLE messages ADD COLUMN IF NOT EXISTS duration FLOAT;

CREATE INDEX IF NOT EXISTS idx_messages_location ON messages(latitude, longitude) WHERE latitude IS NOT NULL;
