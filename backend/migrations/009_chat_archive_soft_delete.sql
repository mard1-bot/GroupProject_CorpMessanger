-- Add archive and soft delete support to chat_members
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS archived BOOLEAN DEFAULT false;
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;

-- Add index for filtering archived/deleted chats
CREATE INDEX IF NOT EXISTS idx_chat_members_archived ON chat_members(archived);
CREATE INDEX IF NOT EXISTS idx_chat_members_deleted_at ON chat_members(deleted_at);
