DROP INDEX IF EXISTS idx_attendance_log_overtime_request;
DROP INDEX IF EXISTS idx_attendance_log_work_segment;

ALTER TABLE daily_attendance
    DROP COLUMN IF EXISTS regular_working_minutes;

ALTER TABLE attendance_log
    DROP COLUMN IF EXISTS overtime_request_id,
    DROP COLUMN IF EXISTS work_segment;

