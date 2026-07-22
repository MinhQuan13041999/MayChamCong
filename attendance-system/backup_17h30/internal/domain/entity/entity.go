package entity

import "time"

// Department đại diện cho phòng ban
type Department struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
}

// Employee đại diện cho nhân viên
type Employee struct {
	ID                  string     `json:"id"`
	EmployeeCode        string     `json:"employee_code"`
	FullName            string     `json:"full_name"`
	DepartmentID        string     `json:"department_id"`
	CardNo              string     `json:"card_no"`
	FingerprintEnrolled bool       `json:"fingerprint_enrolled"`
	FaceEnrolled        bool       `json:"face_enrolled"`
	Status              string     `json:"status"`
	Email               string     `json:"email,omitempty"`
	Phone               string     `json:"phone,omitempty"`
	Gender              string     `json:"gender,omitempty"`
	Dob                 *time.Time `json:"dob,omitempty"`
	JoinDate            time.Time  `json:"join_date"`
	JobTitle            string     `json:"job_title,omitempty"`
	AvatarURL           string     `json:"avatar_url,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// EmployeeDeviceMapping links the web employee to the user identifier on one device.
// Biometric templates remain on the device; this record stores status only.
type EmployeeDeviceMapping struct {
	ID                  string     `json:"id"`
	EmployeeID          string     `json:"employee_id"`
	DeviceID            string     `json:"device_id"`
	DeviceUserID        string     `json:"device_user_id"`
	SyncStatus          string     `json:"sync_status"`
	FingerprintEnrolled bool       `json:"fingerprint_enrolled"`
	FingerprintAt       *time.Time `json:"fingerprint_enrolled_at,omitempty"`
	LastSyncedAt        *time.Time `json:"last_synced_at,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
	EmployeeCode        string     `json:"employee_code,omitempty"`
	EmployeeName        string     `json:"employee_name,omitempty"`
}

// DeviceType định danh hãng máy chấm công
type DeviceType string

const (
	DeviceTypeZKTeco    DeviceType = "zkteco"
	DeviceTypeSunbeam   DeviceType = "sunbeam"
	DeviceTypeHikvision DeviceType = "hikvision"
)

// Device đại diện cho máy chấm công
type Device struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	DeviceType       DeviceType `json:"device_type"`
	IPAddress        string     `json:"ip_address"`
	Port             int        `json:"port"`
	SerialNumber     string     `json:"serial_number"`
	SerialNumberADMS string     `json:"serial_number_adms,omitempty"` // SN dùng cho ADMS Push
	ADMSEnabled      bool       `json:"adms_enabled"`
	Status           string     `json:"status"` // online | offline
	LastCheckedAt    *time.Time `json:"last_checked_at,omitempty"`
	LastHeartbeatAt  *time.Time `json:"last_heartbeat_at,omitempty"` // Lần cuối thiết bị ping ADMS
	Location         string     `json:"location"`
	FirmwareVersion  string     `json:"firmware_version,omitempty"`
	LastOnlineAt     *time.Time `json:"last_online_at,omitempty"`
	MacAddress       string     `json:"mac_address,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// DeviceCommandQueue lệnh đang chờ gửi xuống thiết bị qua ADMS
type DeviceCommandQueue struct {
	ID        int64      `json:"id"`
	DeviceID  string     `json:"device_id"`
	CommandID int64      `json:"command_id"` // ID dùng để ACK từ thiết bị
	Command   string     `json:"command"`    // Nội dung lệnh ZKTeco
	Status    string     `json:"status"`     // pending | sent | ack | failed
	CreatedAt time.Time  `json:"created_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
	AckedAt   *time.Time `json:"acked_at,omitempty"`
}

// EmployeeFingerprint template vân tay lưu tập trung
type EmployeeFingerprint struct {
	ID             int64      `json:"id"`
	EmployeeID     string     `json:"employee_id"`
	FingerIndex    int        `json:"finger_index"`    // 0–9
	TemplateData   string     `json:"template_data"`   // Base64
	TemplateSize   int        `json:"template_size"`
	AlgoVersion    string     `json:"algo_version"`    // "10.0" hoặc "9.0"
	SourceDeviceID string     `json:"source_device_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CheckType xác định loại chấm công
type CheckType string

const (
	CheckTypeIn      CheckType = "in"
	CheckTypeOut     CheckType = "out"
	CheckTypeUnknown CheckType = "unknown"
)

// VerifyMode xác định phương thức xác thực
type VerifyMode string

const (
	VerifyModeFingerprint VerifyMode = "fingerprint"
	VerifyModeFace        VerifyMode = "face"
	VerifyModeCard        VerifyMode = "card"
)

// AttendanceLog log chấm công thô (raw), append-only
type AttendanceLog struct {
	ID           int64      `json:"id"`
	DeviceID     string     `json:"device_id"`
	EmployeeCode string     `json:"employee_code"`
	CheckTime    time.Time  `json:"check_time"`
	CheckType    CheckType  `json:"check_type"`
	VerifyMode   VerifyMode `json:"verify_mode"`
	RawPayload   []byte     `json:"raw_payload,omitempty"`
	SyncedAt     time.Time  `json:"synced_at"`
	EmployeeName string     `json:"employee_name,omitempty"`
	DeviceName   string     `json:"device_name,omitempty"`
}

// SyncStatus trạng thái đồng bộ
type SyncStatus string

const (
	SyncStatusSuccess SyncStatus = "success"
	SyncStatusFailed  SyncStatus = "failed"
	SyncStatusPartial SyncStatus = "partial"
)

// SyncTriggerType loại kích hoạt đồng bộ
type SyncTriggerType string

const (
	SyncTriggerManual    SyncTriggerType = "manual"
	SyncTriggerScheduled SyncTriggerType = "scheduled"
)

// SyncType loại dữ liệu đồng bộ
type SyncType string

const (
	SyncTypeEmployee   SyncType = "employee"
	SyncTypeAttendance SyncType = "attendance"
	SyncTypeTimeSync   SyncType = "time_sync"
)

// SyncHistory lịch sử mỗi lần đồng bộ
type SyncHistory struct {
	ID           int64           `json:"id"`
	DeviceID     string          `json:"device_id"`
	SyncType     SyncType        `json:"sync_type"`
	TriggerType  SyncTriggerType `json:"trigger_type"`
	Status       SyncStatus      `json:"status"`
	RecordCount  int             `json:"record_count"`
	ErrorMessage string          `json:"error_message,omitempty"`
	StartedAt    time.Time       `json:"started_at"`
	FinishedAt   time.Time       `json:"finished_at"`
}

// Shift ca làm việc (phục vụ hệ thống tính công sau này)
type Shift struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	StartTime         string    `json:"start_time"` // HH:MM
	EndTime           string    `json:"end_time"`   // HH:MM
	BreakMinutes      int       `json:"break_minutes"`
	LateGraceMinutes  int       `json:"late_grace_minutes"`
	EarlyGraceMinutes int       `json:"early_grace_minutes"`
	MaxWorkingMinutes int       `json:"max_working_minutes"`
	Timezone          string    `json:"timezone"`
	ColorCode         string    `json:"color_code,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// User tài khoản đăng nhập hệ thống quản trị
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	RoleID       string    `json:"role_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// Role vai trò
type Role struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Permission quyền hạn chi tiết
type Permission struct {
	ID     string `json:"id"`
	RoleID string `json:"role_id"`
	Action string `json:"action"`
	Object string `json:"object"`
}

// EmployeeShift gán ca làm việc cho nhân viên
type EmployeeShift struct {
	ID         string     `json:"id"`
	EmployeeID string     `json:"employee_id"`
	ShiftID    string     `json:"shift_id"`
	StartDate  time.Time  `json:"start_date"`
	EndDate    *time.Time `json:"end_date,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// LeaveRequest đăng ký nghỉ phép
type LeaveRequest struct {
	ID         string    `json:"id"`
	EmployeeID string    `json:"employee_id"`
	LeaveType  string    `json:"leave_type"` // annual, sick, unpaid, business_trip
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	Reason     string    `json:"reason"`
	Status     string    `json:"status"` // pending, approved, rejected
	ApprovedBy *string   `json:"approved_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// AttendanceCorrection đơn xin sửa giờ chấm công
type AttendanceCorrection struct {
	ID            string    `json:"id"`
	EmployeeID    string    `json:"employee_id"`
	Date          time.Time `json:"date"`
	CorrectedTime time.Time `json:"corrected_time"`
	CheckType     string    `json:"check_type"` // in, out
	Reason        string    `json:"reason"`
	Status        string    `json:"status"` // pending, approved, rejected
	ApprovedBy    *string   `json:"approved_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// DailyAttendance kết quả chấm công tổng hợp ngày
type DailyAttendance struct {
	ID               int64      `json:"id"`
	EmployeeID       string     `json:"employee_id"`
	Date             time.Time  `json:"date"`
	ShiftID          *string    `json:"shift_id,omitempty"`
	FirstIn          *time.Time `json:"first_in,omitempty"`
	LastOut          *time.Time `json:"last_out,omitempty"`
	LateMinutes      int        `json:"late_minutes"`
	EarlyMinutes     int        `json:"early_minutes"`
	WorkingHours     float64    `json:"working_hours"`
	AttendanceStatus string     `json:"attendance_status"` // present, late, early, absent, leave
	OvertimeMinutes  int        `json:"overtime_minutes"`
	LeaveID          *string    `json:"leave_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// OvertimeRequest đăng ký làm thêm giờ
type OvertimeRequest struct {
	ID         string    `json:"id"`
	EmployeeID string    `json:"employee_id"`
	Date       time.Time `json:"date"`
	StartTime  string    `json:"start_time"` // HH:MM
	EndTime    string    `json:"end_time"`   // HH:MM
	Status     string    `json:"status"`     // pending, approved, rejected
	ApprovedBy *string   `json:"approved_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// AuditLog nhật ký hoạt động hệ thống
type AuditLog struct {
	ID          int64     `json:"id"`
	UserID      *string   `json:"user_id,omitempty"`
	Action      string    `json:"action"`
	ObjectType  string    `json:"object_type"`
	ObjectID    string    `json:"object_id"`
	Description string    `json:"description"`
	IPAddress   string    `json:"ip_address"`
	CreatedAt   time.Time `json:"created_at"`
}
