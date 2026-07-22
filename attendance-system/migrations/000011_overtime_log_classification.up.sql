ALTER TABLE attendance_log
    ADD COLUMN IF NOT EXISTS work_segment VARCHAR(20) NOT NULL DEFAULT 'regular',
    ADD COLUMN IF NOT EXISTS overtime_request_id UUID REFERENCES overtime_request(id) ON DELETE SET NULL;

ALTER TABLE daily_attendance
    ADD COLUMN IF NOT EXISTS regular_working_minutes INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_attendance_log_work_segment
    ON attendance_log(work_segment);

CREATE INDEX IF NOT EXISTS idx_attendance_log_overtime_request
    ON attendance_log(overtime_request_id)
    WHERE overtime_request_id IS NOT NULL;

