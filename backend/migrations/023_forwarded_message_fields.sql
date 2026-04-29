-- Add fields to track original sender when message is forwarded
ALTER TABLE messages ADD COLUMN forwarded_from UUID;
ALTER TABLE messages ADD COLUMN forwarded_sender_name VARCHAR(255);

-- Add indexes for better query performance
CREATE INDEX idx_messages_forwarded_from ON messages(forwarded_from) WHERE forwarded_from IS NOT NULL;
