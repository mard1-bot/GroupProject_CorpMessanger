-- Restore users data
INSERT INTO users (id, email, password_hash, first_name, last_name, status, role) VALUES
('07155a20-dfc8-48ae-8744-2bdc4c84a780', 'vladimir@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqhmM6JGKpS4G3R1G2JH8YpfB0Bqy', 'Владимир', 'Пододо', 'active', 'user'),
('526f9b0f-f9b8-4f7a-b447-ce3f8f0d25ff', 'arsendos@arsendo.sru', '$2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqhmM6JGKpS4G3R1G2JH8YpfB0Bqy', 'Арсений', 'Видюлин', 'active', 'user')
ON CONFLICT (id) DO UPDATE SET 
    first_name = EXCLUDED.first_name, 
    last_name = EXCLUDED.last_name,
    email = EXCLUDED.email;
