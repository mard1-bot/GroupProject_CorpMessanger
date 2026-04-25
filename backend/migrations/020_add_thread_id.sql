-- Add thread_id column to messages table for message threads
ALTER TABLE messages ADD COLUMN thread_id UUID;
CREATE INDEX idx_messages_thread_id ON messages(thread_id);
