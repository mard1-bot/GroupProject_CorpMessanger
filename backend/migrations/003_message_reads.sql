-- Message read receipts table
CREATE TABLE IF NOT EXISTS message_reads (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    read_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id)
);

-- Index for fetching read status by message
CREATE INDEX idx_message_reads_message_id ON message_reads(message_id);

-- Index for fetching read status by user
CREATE INDEX idx_message_reads_user_id ON message_reads(user_id);
