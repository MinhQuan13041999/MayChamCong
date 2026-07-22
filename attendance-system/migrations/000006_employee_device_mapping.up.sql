CREATE TABLE employee_device_mapping (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES device(id) ON DELETE CASCADE,
    device_user_id VARCHAR(100) NOT NULL,
    sync_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    fingerprint_enrolled BOOLEAN NOT NULL DEFAULT false,
    fingerprint_enrolled_at TIMESTAMPTZ,
    last_synced_at TIMESTAMPTZ,
    last_error TEXT,
    UNIQUE (employee_id, device_id),
    UNIQUE (device_id, device_user_id)
);
