-- Add scheduled_at column to messages table for scheduled messages
ALTER TABLE messages ADD COLUMN scheduled_at TIMESTAMP;
CREATE INDEX idx_messages_scheduled_at ON messages(scheduled_at);
