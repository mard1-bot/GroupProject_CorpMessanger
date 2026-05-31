-- Add hybrid mode preference to notification_settings
ALTER TABLE notification_settings 
ADD COLUMN IF NOT EXISTS hybrid_mode_enabled BOOLEAN DEFAULT false;

-- Add comment
COMMENT ON COLUMN notification_settings.hybrid_mode_enabled IS 'Enable XMPP hybrid mode for this user (WebSocket + XMPP sync)';
