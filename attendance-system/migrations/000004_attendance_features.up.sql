-- Phân ca làm việc (nhân viên nào làm ca nào)
CREATE TABLE employee_shift (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
    shift_id UUID NOT NULL REFERENCES shift(id) ON DELETE CASCADE,
    start_date DATE NOT NULL,
    end_date DATE, -- NULL nếu áp dụng lâu dài
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Đăng ký nghỉ phép (nghỉ ốm, phép năm, không lương, đi công tác...)
CREATE TABLE leave_request (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
    leave_type VARCHAR(50) NOT NULL, -- annual, sick, unpaid, business_trip
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    reason TEXT,
    status VARCHAR(20) DEFAULT 'pending', -- pending, approved, rejected
    approved_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Bảng công tổng hợp hàng ngày (Dữ liệu đã được xử lý từ log thô)
CREATE TABLE daily_attendance (
    id BIGSERIAL PRIMARY KEY,
    employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    shift_id UUID REFERENCES shift(id) ON DELETE SET NULL,
    first_in TIMESTAMPTZ,              -- Check-in đầu tiên trong ngày
    last_out TIMESTAMPTZ,              -- Check-out cuối cùng trong ngày
    late_minutes INTEGER DEFAULT 0,    -- Số phút đi muộn
    early_minutes INTEGER DEFAULT 0,   -- Số phút về sớm
    working_hours NUMERIC(4,2) DEFAULT 0.00, -- Số giờ làm việc thực tế
    attendance_status VARCHAR(30) DEFAULT 'absent', -- present, late, early, absent, leave
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(employee_id, date)
);
CREATE INDEX idx_daily_att_date ON daily_attendance(date);

-- Đăng ký làm thêm giờ (OT)
CREATE TABLE overtime_request (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    status VARCHAR(20) DEFAULT 'pending', -- pending, approved, rejected
    approved_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Nhật ký hoạt động hệ thống (audit log của admin/HR)
CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,      -- CREATE_DEVICE, UPDATE_EMPLOYEE...
    object_type VARCHAR(50) NOT NULL,  -- device, employee, sync_history
    object_id VARCHAR(100),
    description TEXT,
    ip_address VARCHAR(50),
    created_at TIMESTAMPTZ DEFAULT now()
);
