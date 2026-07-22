ALTER TABLE shift ADD COLUMN IF NOT EXISTS break_minutes INTEGER NOT NULL DEFAULT 0 CHECK (break_minutes >= 0);
ALTER TABLE shift ADD COLUMN IF NOT EXISTS late_grace_minutes INTEGER NOT NULL DEFAULT 0 CHECK (late_grace_minutes >= 0);
ALTER TABLE shift ADD COLUMN IF NOT EXISTS early_grace_minutes INTEGER NOT NULL DEFAULT 0 CHECK (early_grace_minutes >= 0);
ALTER TABLE shift ADD COLUMN IF NOT EXISTS max_working_minutes INTEGER NOT NULL DEFAULT 0 CHECK (max_working_minutes >= 0);
ALTER TABLE shift ADD COLUMN IF NOT EXISTS timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Ho_Chi_Minh';
CREATE TABLE sync_cursor (device_id UUID PRIMARY KEY REFERENCES device(id) ON DELETE CASCADE, attendance_cursor TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
