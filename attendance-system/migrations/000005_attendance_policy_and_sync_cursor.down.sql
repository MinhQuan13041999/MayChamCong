DROP TABLE IF EXISTS sync_cursor;
ALTER TABLE shift DROP COLUMN IF EXISTS timezone, DROP COLUMN IF EXISTS max_working_minutes, DROP COLUMN IF EXISTS early_grace_minutes, DROP COLUMN IF EXISTS late_grace_minutes, DROP COLUMN IF EXISTS break_minutes;
