-- Migration to create first admin user
-- This should be run after initial schema setup
-- IMPORTANT: Do NOT run this migration in production with default values!

-- Insert first admin user (without password)
-- Password should be set via the admin API or using the setadminpass tool
INSERT INTO users (email, first_name, last_name, role, status, created_at, updated_at)
VALUES ('admin@corpmessenger.com', 'System', 'Admin', 'admin', 'active', NOW(), NOW())
ON CONFLICT (email) DO NOTHING;

-- NOTE: To set the admin password, use the setadminpass tool:
-- cd backend/cmd/setadminpass && go run main.go "YourSecurePassword123!"
-- Or use the admin API endpoint: POST /api/v1/admin/users/{id}/password
