-- Cập nhật password hash chính xác cho admin (password: admin123)
-- Hash được tạo bằng bcrypt cost=10 bằng golang.org/x/crypto/bcrypt
UPDATE "user"
SET password_hash = '$2a$10$i41jXvF6EZRvSqr7Y1gHq./vkDpZjw5mKSK6oVbpSBlMkFCtwqlnG'
WHERE username = 'admin';
