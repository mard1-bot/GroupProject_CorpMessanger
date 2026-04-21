-- Track unread counts for badge
CREATE TABLE unread_counts (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    chat_id UUID REFERENCES chats(id) ON DELETE CASCADE,
    count INTEGER DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (user_id, chat_id)
);

CREATE INDEX idx_unread_counts_user_id ON unread_counts(user_id);

-- Function to update unread count
CREATE OR REPLACE FUNCTION update_unread_count()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO unread_counts (user_id, chat_id, count, updated_at)
    SELECT cm.user_id, NEW.chat_id, 1, NOW()
    FROM chat_members cm
    WHERE cm.chat_id = NEW.chat_id AND cm.user_id != NEW.sender_id
    ON CONFLICT (user_id, chat_id)
    DO UPDATE SET count = unread_counts.count + 1, updated_at = NOW();
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER message_unread_count
    AFTER INSERT ON messages
    FOR EACH ROW
    EXECUTE FUNCTION update_unread_count();
