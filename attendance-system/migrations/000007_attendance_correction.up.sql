-- Đơn xin sửa giờ chấm công (khi quên quét thẻ check-in/out)
CREATE TABLE attendance_correction (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    corrected_time TIMESTAMPTZ NOT NULL,
    check_type VARCHAR(10) NOT NULL, -- 'in' hoặc 'out'
    reason TEXT,
    status VARCHAR(20) DEFAULT 'pending', -- pending, approved, rejected
    approved_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);
