-- Add protocol_preference column to notification_settings
ALTER TABLE notification_settings 
ADD COLUMN protocol_preference VARCHAR(20) DEFAULT 'websocket' 
CHECK (protocol_preference IN ('websocket', 'xmpp'));

-- Update existing records to default to websocket
UPDATE notification_settings 
SET protocol_preference = 'websocket' 
WHERE protocol_preference IS NULL;
