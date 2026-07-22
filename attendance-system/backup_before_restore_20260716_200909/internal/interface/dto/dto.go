package dto

import "attendance-system/internal/domain/entity"

// CreateDeviceRequest payload tạo thiết bị mới
type CreateDeviceRequest struct {
	Name             string `json:"name"`
	DeviceType       string `json:"device_type"`
	IPAddress        string `json:"ip_address"`
	Port             int    `json:"port"`
	SerialNumber     string `json:"serial_number"`
	SerialNumberADMS string `json:"serial_number_adms"`
	ADMSEnabled      bool   `json:"adms_enabled"`
	Location         string `json:"location"`
	FirmwareVersion  string `json:"firmware_version"`
	MacAddress       string `json:"mac_address"`
}

// UpdateDeviceRequest payload sửa thiết bị
type UpdateDeviceRequest struct {
	Name             string `json:"name"`
	DeviceType       string `json:"device_type"`
	IPAddress        string `json:"ip_address"`
	Port             int    `json:"port"`
	SerialNumber     string `json:"serial_number"`
	SerialNumberADMS string `json:"serial_number_adms"`
	ADMSEnabled      bool   `json:"adms_enabled"`
	Location         string `json:"location"`
	FirmwareVersion  string `json:"firmware_version"`
	MacAddress       string `json:"mac_address"`
}

// CreateEmployeeRequest payload tạo nhân viên mới
type CreateEmployeeRequest struct {
	EmployeeCode      string  `json:"employee_code"`
	FullName          string  `json:"full_name"`
	DepartmentID      string  `json:"department_id"`
	CardNo            string  `json:"card_no"`
	Email             string  `json:"email"`
	Phone             string  `json:"phone"`
	Gender            string  `json:"gender"`
	Dob               *string `json:"dob"`       // YYYY-MM-DD
	JoinDate          *string `json:"join_date"` // YYYY-MM-DD
	JobTitle          string  `json:"job_title"`
	AvatarURL         string  `json:"avatar_url"`
	EnrollFingerprint bool    `json:"enroll_fingerprint"`
	DeviceID          string  `json:"device_id"`
	DeviceUserID      string  `json:"device_user_id"`
}

// UpdateEmployeeRequest payload sửa nhân viên
type UpdateEmployeeRequest struct {
	FullName     string  `json:"full_name"`
	DepartmentID string  `json:"department_id"`
	CardNo       string  `json:"card_no"`
	Status       string  `json:"status"`
	Email        string  `json:"email"`
	Phone        string  `json:"phone"`
	Gender       string  `json:"gender"`
	Dob          *string `json:"dob"`       // YYYY-MM-DD
	JoinDate     *string `json:"join_date"` // YYYY-MM-DD
	JobTitle     string  `json:"job_title"`
	AvatarURL    string  `json:"avatar_url"`
}

type SyncEmployeeToDeviceRequest struct {
	DeviceUserID string `json:"device_user_id"`
}

type BatchEnrollRequest struct {
	EmployeeIDs []string `json:"employee_ids"`
	DeviceID    string   `json:"device_id"`
}

// ErrorResponse response chuẩn khi có lỗi
type ErrorResponse struct {
	Error string `json:"error"`
}

// SyncAttendanceRequest payload trigger đồng bộ chấm công thủ công
type SyncAttendanceRequest struct {
	From string `json:"from"` // RFC3339
	To   string `json:"to"`   // RFC3339
}

// SyncHistoryResponse trả về thông tin 1 lần đồng bộ
type SyncHistoryResponse struct {
	ID           int64  `json:"id"`
	DeviceID     string `json:"device_id"`
	SyncType     string `json:"sync_type"`
	TriggerType  string `json:"trigger_type"`
	Status       string `json:"status"`
	RecordCount  int    `json:"record_count"`
	ErrorMessage string `json:"error_message,omitempty"`
}

func FromSyncHistory(h *entity.SyncHistory) SyncHistoryResponse {
	return SyncHistoryResponse{
		ID:           h.ID,
		DeviceID:     h.DeviceID,
		SyncType:     string(h.SyncType),
		TriggerType:  string(h.TriggerType),
		Status:       string(h.Status),
		RecordCount:  h.RecordCount,
		ErrorMessage: h.ErrorMessage,
	}
}
