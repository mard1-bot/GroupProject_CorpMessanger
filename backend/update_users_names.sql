-- Update existing users with first_name and last_name
-- This script updates users that may have empty names

UPDATE users SET first_name = 'Иван', last_name = 'Тестовый' WHERE email = 'test1@example.com' AND (first_name IS NULL OR first_name = '');
UPDATE users SET first_name = 'Мария', last_name = 'Тестовая' WHERE email = 'test2@example.com' AND (first_name IS NULL OR first_name = '');
UPDATE users SET first_name = 'Петр', last_name = 'Тестовый' WHERE email = 'test3@example.com' AND (first_name IS NULL OR first_name = '');
UPDATE users SET first_name = 'Владимир', last_name = 'Пододо' WHERE email = 'vladimir@example.com' AND (first_name IS NULL OR first_name = '');

-- Insert test users if they don't exist
INSERT INTO users (id, email, first_name, last_name, status, role, created_at, updated_at) VALUES
('11111111-1111-1111-1111-111111111111', 'test1@example.com', 'Иван', 'Тестовый', 'active', 'user', NOW(), NOW()),
('22222222-2222-2222-2222-222222222222', 'test2@example.com', 'Мария', 'Тестовая', 'active', 'user', NOW(), NOW()),
('33333333-3333-3333-3333-333333333333', 'test3@example.com', 'Петр', 'Тестовый', 'active', 'user', NOW(), NOW()),
('44444444-4444-4444-4444-444444444444', 'vladimir@example.com', 'Владимир', 'Пододо', 'active', 'user', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    first_name = EXCLUDED.first_name, 
    last_name = EXCLUDED.last_name,
    email = EXCLUDED.email;

-- Add passwords for users who don't have them
INSERT INTO user_credentials (user_id, password_hash) VALUES
('11111111-1111-1111-1111-111111111111', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi'),
('22222222-2222-2222-2222-222222222222', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi'),
('33333333-3333-3333-3333-333333333333', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi'),
('44444444-4444-4444-4444-444444444444', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi')
ON CONFLICT (user_id) DO UPDATE SET 
    password_hash = EXCLUDED.password_hash;
