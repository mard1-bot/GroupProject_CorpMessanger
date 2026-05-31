-- Ensure ALL columns exist in messages table (fix for databases created before schema updates)
-- Note: REFERENCES omitted in ADD COLUMN to avoid FK constraint errors on existing tables
ALTER TABLE messages ADD COLUMN IF NOT EXISTS file_url TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS reply_to_content TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS reply_to_sender_name VARCHAR(255);
ALTER TABLE messages ADD COLUMN IF NOT EXISTS pinned BOOLEAN DEFAULT false;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS duration FLOAT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS thread_id UUID;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS forwarded_from UUID;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS forwarded_sender_name VARCHAR(255);
ALTER TABLE messages ADD COLUMN IF NOT EXISTS encrypted_content TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS encryption_key_id UUID;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS encrypted_keys JSONB;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS location JSONB;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS synced_to_xmpp BOOLEAN DEFAULT false;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS xmpp_message_id VARCHAR(255);

-- Ensure ALL columns exist in chats table
ALTER TABLE chats ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE chats ADD COLUMN IF NOT EXISTS avatar VARCHAR(500);

-- Ensure ALL columns exist in chat_members table
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS muted BOOLEAN DEFAULT false;
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS pinned BOOLEAN DEFAULT false;
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS archived BOOLEAN DEFAULT false;
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS last_read_at TIMESTAMP WITH TIME ZONE;

-- Ensure ALL columns exist in users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(50);
ALTER TABLE users ADD COLUMN IF NOT EXISTS middle_name VARCHAR(100);
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar VARCHAR(500);
ALTER TABLE users ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'active';
ALTER TABLE users ADD COLUMN IF NOT EXISTS custom_status VARCHAR(200);
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_online TIMESTAMP WITH TIME ZONE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) DEFAULT 'user';

-- Ensure ALL columns exist in files table
ALTER TABLE files ADD COLUMN IF NOT EXISTS thumbnail_url TEXT;

-- Drop old latitude/longitude/location_address if they still exist
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'messages' AND column_name = 'latitude') THEN
        ALTER TABLE messages DROP COLUMN latitude;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'messages' AND column_name = 'longitude') THEN
        ALTER TABLE messages DROP COLUMN longitude;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'messages' AND column_name = 'location_address') THEN
        ALTER TABLE messages DROP COLUMN location_address;
    END IF;
END $$;
