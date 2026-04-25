-- Add last_online field to users for online status tracking
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_online TIMESTAMP WITH TIME ZONE;
