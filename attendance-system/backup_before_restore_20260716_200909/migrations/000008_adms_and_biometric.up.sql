-- Migration 008: ADMS Push Protocol + Biometric Sync
-- Thêm hỗ trợ ZKTeco ADMS (thiết bị tự đẩy log) và lưu trữ vân tay tập trung

-- 1. Mở rộng bảng device: thêm cột ADMS
ALTER TABLE device
    ADD COLUMN IF NOT EXISTS serial_number_adms  VARCHAR(100),
    ADD COLUMN IF NOT EXISTS last_heartbeat_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS adms_enabled        BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS firmware_version    VARCHAR(100),
    ADD COLUMN IF NOT EXISTS mac_address         VARCHAR(50),
    ADD COLUMN IF NOT EXISTS last_online_at      TIMESTAMPTZ;

-- Tạo UNIQUE index riêng (không phải UNIQUE constraint trên cột vì có thể NULL)
CREATE UNIQUE INDEX IF NOT EXISTS uix_device_serial_adms
    ON device(serial_number_adms)
    WHERE serial_number_adms IS NOT NULL;

-- 2. Hàng đợi lệnh gửi xuống thiết bị (ADMS Command Queue)
CREATE TABLE IF NOT EXISTS device_command_queue (
    id          BIGSERIAL PRIMARY KEY,
    device_id   UUID NOT NULL REFERENCES device(id) ON DELETE CASCADE,
    command_id  BIGINT NOT NULL,              -- ID tăng dần, dùng để ACK
    command     TEXT NOT NULL,               -- Nội dung lệnh theo định dạng ZKTeco
    status      VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending | sent | ack | failed
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at     TIMESTAMPTZ,
    acked_at    TIMESTAMPTZ,
    UNIQUE(device_id, command_id)
);
CREATE INDEX IF NOT EXISTS idx_cmd_queue_device_status
    ON device_command_queue(device_id, status);

-- Sequence per-device command_id: dùng global sequence, ổn vì command_id chỉ cần unique per-device
CREATE SEQUENCE IF NOT EXISTS device_command_id_seq START 1;

-- 3. Bảng lưu template vân tay tập trung
CREATE TABLE IF NOT EXISTS employee_fingerprint (
    id               BIGSERIAL PRIMARY KEY,
    employee_id      UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
    finger_index     INT  NOT NULL CHECK (finger_index BETWEEN 0 AND 9),
    template_data    TEXT NOT NULL,           -- Base64 encoded ZKTeco template
    template_size    INT  NOT NULL DEFAULT 0,
    algo_version     VARCHAR(20) NOT NULL DEFAULT '10.0',
    source_device_id UUID REFERENCES device(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(employee_id, finger_index)
);
CREATE INDEX IF NOT EXISTS idx_fingerprint_employee
    ON employee_fingerprint(employee_id);
