-- Extension cần thiết cho gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Department
CREATE TABLE department (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50) UNIQUE,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Employee
CREATE TABLE employee (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_code VARCHAR(50) UNIQUE NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    department_id UUID REFERENCES department(id),
    card_no VARCHAR(50),
    fingerprint_enrolled BOOLEAN DEFAULT false,
    face_enrolled BOOLEAN DEFAULT false,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Device
CREATE TABLE device (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    device_type VARCHAR(20) NOT NULL, -- zkteco | sunbeam | hikvision
    ip_address VARCHAR(50) NOT NULL,
    port INTEGER NOT NULL,
    serial_number VARCHAR(100),
    status VARCHAR(20) DEFAULT 'offline', -- online | offline
    last_checked_at TIMESTAMPTZ,
    location VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Attendance Log (Raw)
CREATE TABLE attendance_log (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID REFERENCES device(id),
    employee_code VARCHAR(50) NOT NULL,
    check_time TIMESTAMPTZ NOT NULL,
    check_type VARCHAR(20), -- in | out | unknown
    verify_mode VARCHAR(20), -- fingerprint | face | card
    raw_payload JSONB,       -- lưu nguyên payload gốc từ SDK để truy vết
    synced_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(device_id, employee_code, check_time)
);
CREATE INDEX idx_attendance_check_time ON attendance_log(check_time);
CREATE INDEX idx_attendance_employee_code ON attendance_log(employee_code);

-- Sync History
CREATE TABLE sync_history (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID REFERENCES device(id),
    sync_type VARCHAR(30), -- employee | attendance | time_sync
    trigger_type VARCHAR(20), -- manual | scheduled
    status VARCHAR(20), -- success | failed | partial
    record_count INTEGER DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);
CREATE INDEX idx_sync_history_device ON sync_history(device_id);
CREATE INDEX idx_sync_history_status ON sync_history(status);

-- Shift (ca làm việc, phục vụ hệ thống tính công sau này)
CREATE TABLE shift (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Role (RBAC)
CREATE TABLE role (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) UNIQUE NOT NULL
);

-- User (tài khoản đăng nhập hệ thống quản trị)
CREATE TABLE "user" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role_id UUID REFERENCES role(id),
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Permission (quyền hạn chi tiết theo role)
CREATE TABLE permission (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID REFERENCES role(id),
    action VARCHAR(50) NOT NULL,  -- create | read | update | delete
    object VARCHAR(50) NOT NULL,  -- device | employee | attendance_log | ...
    UNIQUE(role_id, action, object)
);

-- Seed roles mặc định
INSERT INTO role (name) VALUES ('admin'), ('hr'), ('viewer');
