-- Seed admin user mặc định
-- Password: admin123  (bcrypt cost=10 hash)
INSERT INTO "user" (username, password_hash, role_id)
SELECT
    'admin',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    r.id
FROM role r
WHERE r.name = 'admin'
ON CONFLICT (username) DO NOTHING;
