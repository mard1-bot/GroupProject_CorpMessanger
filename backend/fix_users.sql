-- Create test users with correct bcrypt password hash for 'password123'
-- Hash: $2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi

INSERT INTO users (id, email, password_hash, first_name, last_name, status, role, created_at, updated_at) VALUES
('11111111-1111-1111-1111-111111111111', 'test1@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'Иван', 'Тестовый', 'active', 'user', NOW(), NOW()),
('22222222-2222-2222-2222-222222222222', 'test2@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'Мария', 'Тестовая', 'active', 'user', NOW(), NOW()),
('33333333-3333-3333-3333-333333333333', 'test3@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'Петр', 'Тестовый', 'active', 'user', NOW(), NOW()),
('44444444-4444-4444-4444-444444444444', 'vladimir@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'Владимир', 'Пододо', 'active', 'user', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    first_name = EXCLUDED.first_name, 
    last_name = EXCLUDED.last_name,
    email = EXCLUDED.email,
    password_hash = EXCLUDED.password_hash;
