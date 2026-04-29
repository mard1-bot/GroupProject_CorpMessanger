-- Add fields for XMPP message synchronization
ALTER TABLE messages ADD COLUMN synced_to_xmpp BOOLEAN DEFAULT FALSE;
ALTER TABLE messages ADD COLUMN xmpp_message_id VARCHAR(255);

-- Add indexes for better query performance
CREATE INDEX idx_messages_synced_to_xmpp ON messages(synced_to_xmpp) WHERE synced_to_xmpp = FALSE;
CREATE INDEX idx_messages_xmpp_message_id ON messages(xmpp_message_id) WHERE xmpp_message_id IS NOT NULL;
