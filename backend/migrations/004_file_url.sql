-- Add file_url column to messages table for file attachments
ALTER TABLE messages ADD COLUMN IF NOT EXISTS file_url TEXT;
