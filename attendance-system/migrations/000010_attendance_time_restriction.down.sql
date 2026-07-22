ALTER TABLE attendance_log
    DROP COLUMN IF EXISTS invalid_reason,
    DROP COLUMN IF EXISTS is_valid;

