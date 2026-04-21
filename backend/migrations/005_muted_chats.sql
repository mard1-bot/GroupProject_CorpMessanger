-- Add muted column to chat_members for chat muting functionality
ALTER TABLE chat_members ADD COLUMN muted BOOLEAN NOT NULL DEFAULT false;

-- Add index for efficient queries of muted chats
CREATE INDEX idx_chat_members_muted ON chat_members(user_id, muted) WHERE muted = true;
