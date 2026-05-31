-- Performance indexes for optimal query performance

-- Messages table indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_chat_id_created_at 
ON messages(chat_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_sender_id_created_at 
ON messages(sender_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_type_created_at 
ON messages(type, created_at DESC) WHERE type IN ('file', 'voice');

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_reply_to 
ON messages(reply_to) WHERE reply_to IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_thread_id 
ON messages(thread_id) WHERE thread_id IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_forwarded_from 
ON messages(forwarded_from) WHERE forwarded_from IS NOT NULL;

-- Full-text search index for message content
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_content_gin 
ON messages USING gin(to_tsvector('russian', content));

-- Chat members table indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_members_user_id_role 
ON chat_members(user_id, role) WHERE role IN ('owner', 'admin');

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_members_chat_id_joined_at 
ON chat_members(chat_id, joined_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_members_muted_pinned 
ON chat_members(muted, pinned) WHERE muted = true OR pinned = true;

-- Users table indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email_lower 
ON users(LOWER(email));

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_username_lower 
ON users(LOWER(username));

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at 
ON users(created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_last_online 
ON users(last_online DESC) WHERE last_online IS NOT NULL;

-- Audit logs table indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_user_id_created_at 
ON audit_logs(user_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_action_created_at 
ON audit_logs(action, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_ip_address 
ON audit_logs(ip_address);

-- Notification settings indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_notification_settings_push_enabled 
ON notification_settings(push_enabled) WHERE push_enabled = true;

-- Device tokens indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_device_tokens_user_id_platform 
ON device_tokens(user_id, platform);

-- Web push subscriptions indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_web_push_subscriptions_user_id 
ON web_push_subscriptions(user_id);

-- Bookmarks indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bookmarks_user_id_created_at 
ON bookmarks(user_id, created_at DESC);

-- Blocked users indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_blocked_users_blocker_id_created_at 
ON blocked_users(blocker_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_blocked_users_blocked_id 
ON blocked_users(blocked_id);

-- Login attempts indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_login_attempts_email_created_at 
ON login_attempts(email, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_login_attempts_ip_address_created_at 
ON login_attempts(ip_address, created_at DESC);

-- Encryption keys indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_encryption_keys_user_id_created_at 
ON encryption_keys(user_id, created_at DESC);

-- Message reads indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_message_reads_message_id_user_id 
ON message_reads(message_id, user_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_message_reads_user_id_read_at 
ON message_reads(user_id, read_at DESC);

-- Reactions indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reactions_message_id_emoji 
ON reactions(message_id, emoji);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reactions_user_id_created_at 
ON reactions(user_id, created_at DESC);

-- Pinned messages indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_pinned_messages_chat_id_pinned_at 
ON pinned_messages(chat_id, pinned_at DESC);

-- Unread counts indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_unread_counts_user_id_chat_id 
ON unread_counts(user_id, chat_id);

-- Mentions indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_mentions_user_id_created_at 
ON mentions(user_id, created_at DESC);

-- XMPP reconciliation indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_xmpp_reconciliation_chat_id_status 
ON xmpp_reconciliation(chat_id, status) WHERE status != 'resolved';

-- Scheduler failures indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_scheduler_failures_message_id_created_at 
ON scheduler_failures(message_id, created_at DESC);
