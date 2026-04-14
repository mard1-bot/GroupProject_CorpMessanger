DELETE FROM users WHERE email = 'demo@demo.com';
DELETE FROM users WHERE email = 'admin@admin.com';

INSERT INTO users (id, email, password_hash, first_name, last_name, status, role, created_at, updated_at) 
VALUES 
('550e8400-e29b-41d4-a716-446655440000', 'demo@demo.com', '$2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqhmM6JGKpS4G3R1G2JH8YpfB0Bqy', 'Демо', 'Пользователь', 'active', 'user', NOW(), NOW()),
('550e8400-e29b-41d4-a716-446655440001', 'admin@admin.com', '$2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqhmM6JGKpS4G3R1G2JH8YpfB0Bqy', 'Админ', 'Админов', 'active', 'admin', NOW(), NOW());
