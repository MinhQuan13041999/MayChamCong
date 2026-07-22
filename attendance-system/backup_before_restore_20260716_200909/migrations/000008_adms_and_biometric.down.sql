DROP TABLE IF EXISTS employee_fingerprint;
DROP TABLE IF EXISTS device_command_queue;
DROP SEQUENCE IF EXISTS device_command_id_seq;
DROP INDEX IF EXISTS uix_device_serial_adms;
ALTER TABLE device
    DROP COLUMN IF EXISTS serial_number_adms,
    DROP COLUMN IF EXISTS last_heartbeat_at,
    DROP COLUMN IF EXISTS adms_enabled,
    DROP COLUMN IF EXISTS firmware_version,
    DROP COLUMN IF EXISTS mac_address,
    DROP COLUMN IF EXISTS last_online_at;
