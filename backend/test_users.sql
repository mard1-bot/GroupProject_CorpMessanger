-- Create test users with password: test123
-- Password hash for 'test123': $2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqhmM6JGKpS4G3R1G2JH8YpfB0Bqy

INSERT INTO users (id, email, password_hash, first_name, last_name, status, role, created_at, updated_at) VALUES
('11111111-1111-1111-1111-111111111111', 'test1@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqhmM6JGKpS4G3R1G2JH8YpfB0Bqy', 'Иван', 'Тестовый', 'active', 'user', NOW(), NOW()),
('22222222-2222-2222-2222-222222222222', 'test2@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqhmM6JGKpS4G3R1G2JH8YpfB0Bqy', 'Мария', 'Тестовая', 'active', 'user', NOW(), NOW()),
('33333333-3333-3333-3333-333333333333', 'test3@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqhmM6JGKpS4G3R1G2JH8YpfB0Bqy', 'Петр', 'Тестовый', 'active', 'user', NOW(), NOW()),
('44444444-4444-4444-4444-444444444444', 'vladimir@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqhmM6JGKpS4G3R1G2JH8YpfB0Bqy', 'Владимир', 'Пододо', 'active', 'user', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    first_name = EXCLUDED.first_name, 
    last_name = EXCLUDED.last_name,
    email = EXCLUDED.email,
    password_hash = EXCLUDED.password_hash;
