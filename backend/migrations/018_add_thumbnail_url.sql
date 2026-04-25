-- Add thumbnail_url column to files table
ALTER TABLE files ADD COLUMN thumbnail_url TEXT;
CREATE INDEX idx_files_thumbnail_url ON files(thumbnail_url);
