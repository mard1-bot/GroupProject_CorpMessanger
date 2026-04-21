-- Add pinned column to chat_members table
ALTER TABLE chat_members ADD COLUMN pinned BOOLEAN DEFAULT FALSE;

-- Create index for faster queries
CREATE INDEX idx_chat_members_pinned ON chat_members(user_id, pinned) WHERE pinned = TRUE;
