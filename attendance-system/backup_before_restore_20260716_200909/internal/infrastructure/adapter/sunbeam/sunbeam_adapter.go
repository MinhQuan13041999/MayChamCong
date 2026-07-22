// Package sunbeam implements port.DeviceAdapter for Sunbeam (Timmy) devices.
//
// Sunbeam/Timmy thiết bị cung cấp REST API chạy trực tiếp trên thiết bị qua HTTP.
// Adapter này dùng net/http thuần + Digest Auth để giao tiếp, không cần SDK riêng.
package sunbeam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

// Sunbeam API response types — chỉ dùng trong package này, không lộ ra ngoài.
type sbEmployee struct {
	Pin      string `json:"pin"`
	Name     string `json:"name"`
	CardNo   string `json:"card_no"`
	Passport string `json:"passport"`
}

type sbAttLog struct {
	Pin        string `json:"pin"`
	Time       string `json:"time"`        // "2006-01-02 15:04:05"
	Status     int    `json:"status"`      // 0=check-in, 1=check-out
	VerifyType int    `json:"verify_type"` // 1=fingerprint,4=face,15=card
}

// Adapter implement port.DeviceAdapter cho thiết bị Sunbeam/Timmy qua HTTP REST API.
type Adapter struct {
	cfg        port.DeviceConfig
	baseURL    string
	httpClient *http.Client
}

// New tạo instance mới cho Sunbeam adapter.
func New() *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (a *Adapter) Connect(ctx context.Context, cfg port.DeviceConfig) error {
	a.cfg = cfg
	if cfg.IPAddress == "" {
		return fmt.Errorf("sunbeam: ip_address is required")
	}
	port := cfg.Port
	if port == 0 {
		port = 80
	}
	a.baseURL = fmt.Sprintf("http://%s:%d", cfg.IPAddress, port)

	// Thử kết nối bằng cách gọi endpoint thông tin thiết bị
	_, err := a.doGET(ctx, "/iclock/info")
	if err != nil {
		return fmt.Errorf("sunbeam: cannot connect to %s: %w", a.baseURL, err)
	}
	return nil
}

func (a *Adapter) Disconnect(ctx context.Context) error {
	// HTTP stateless — không cần teardown
	return nil
}

func (a *Adapter) CheckStatus(ctx context.Context) (port.DeviceStatus, error) {
	body, err := a.doGET(ctx, "/iclock/info")
	if err != nil {
		return port.DeviceStatus{}, fmt.Errorf("sunbeam check status: %w", err)
	}
	var info map[string]interface{}
	_ = json.Unmarshal(body, &info)
	fw, _ := info["firmware"].(string)
	return port.DeviceStatus{
		Online:       true,
		FirmwareInfo: fw,
	}, nil
}

func (a *Adapter) SyncTime(ctx context.Context) error {
	payload := map[string]string{
		"time": time.Now().Format("2006-01-02 15:04:05"),
	}
	body, _ := json.Marshal(payload)
	_, err := a.doPOST(ctx, "/iclock/time", body)
	if err != nil {
		return fmt.Errorf("sunbeam sync time: %w", err)
	}
	return nil
}

func (a *Adapter) GetEmployees(ctx context.Context) ([]entity.Employee, error) {
	body, err := a.doGET(ctx, "/iclock/employee/all")
	if err != nil {
		return nil, fmt.Errorf("sunbeam get employees: %w", err)
	}
	var sbEmps []sbEmployee
	if err := json.Unmarshal(body, &sbEmps); err != nil {
		return nil, fmt.Errorf("sunbeam parse employees: %w", err)
	}

	employees := make([]entity.Employee, 0, len(sbEmps))
	for _, e := range sbEmps {
		employees = append(employees, entity.Employee{
			EmployeeCode: e.Pin,
			FullName:     e.Name,
			CardNo:       e.CardNo,
		})
	}
	return employees, nil
}

func (a *Adapter) PushEmployee(ctx context.Context, emp entity.Employee) error {
	payload := sbEmployee{
		Pin:    emp.EmployeeCode,
		Name:   emp.FullName,
		CardNo: emp.CardNo,
	}
	body, _ := json.Marshal(payload)
	_, err := a.doPOST(ctx, "/iclock/employee", body)
	if err != nil {
		return fmt.Errorf("sunbeam push employee %s: %w", emp.EmployeeCode, err)
	}
	return nil
}

func (a *Adapter) PushFingerprint(ctx context.Context, employeeCode string, fp entity.EmployeeFingerprint) error {
	// Sunbeam không cấu hình cho việc đẩy vân tay thô trực tiếp trong adapter này
	return fmt.Errorf("sunbeam: PushFingerprint is not supported in this adapter")
}

func (a *Adapter) GetFingerprint(ctx context.Context, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	return nil, fmt.Errorf("sunbeam: GetFingerprint is not supported")
}

func (a *Adapter) GetAttendanceLogs(ctx context.Context, from, to time.Time) ([]entity.AttendanceLog, error) {
	url := fmt.Sprintf("/iclock/attlogs?from=%s&to=%s",
		from.Format("2006-01-02T15:04:05"),
		to.Format("2006-01-02T15:04:05"),
	)
	body, err := a.doGET(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("sunbeam get attendance: %w", err)
	}

	var sbLogs []sbAttLog
	if err := json.Unmarshal(body, &sbLogs); err != nil {
		return nil, fmt.Errorf("sunbeam parse attendance: %w", err)
	}

	var logs []entity.AttendanceLog
	for _, l := range sbLogs {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", l.Time, time.Local)
		if err != nil {
			continue
		}
		rawBytes, _ := json.Marshal(l)
		logs = append(logs, entity.AttendanceLog{
			EmployeeCode: l.Pin,
			CheckTime:    t,
			CheckType:    mapCheckType(l.Status),
			VerifyMode:   mapVerifyMode(l.VerifyType),
			RawPayload:   rawBytes,
		})
	}
	return logs, nil
}

func (a *Adapter) ClearAttendanceLogs(ctx context.Context) error {
	// Sunbeam không hỗ trợ xoá log từ xa qua API
	return fmt.Errorf("sunbeam: ClearAttendanceLogs not supported by this device")
}

func (a *Adapter) Reboot(ctx context.Context) error {
	_, err := a.doPOST(ctx, "/iclock/reboot", nil)
	if err != nil {
		return fmt.Errorf("sunbeam reboot: %w", err)
	}
	return nil
}

func (a *Adapter) Reset(ctx context.Context) error {
	return fmt.Errorf("sunbeam: Reset not supported")
}

func (a *Adapter) EnrollFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error {
	return fmt.Errorf("sunbeam: EnrollFingerprint not supported")
}

// ---- HTTP helpers ----

func (a *Adapter) doGET(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if a.cfg.Username != "" {
		req.SetBasicAuth(a.cfg.Username, a.cfg.Password)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, path)
	}
	return io.ReadAll(resp.Body)
}

func (a *Adapter) doPOST(ctx context.Context, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.cfg.Username != "" {
		req.SetBasicAuth(a.cfg.Username, a.cfg.Password)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, path)
	}
	return io.ReadAll(resp.Body)
}

func mapCheckType(status int) entity.CheckType {
	switch status {
	case 0:
		return entity.CheckTypeIn
	case 1:
		return entity.CheckTypeOut
	default:
		return entity.CheckTypeUnknown
	}
}

func mapVerifyMode(vt int) entity.VerifyMode {
	switch vt {
	case 1:
		return entity.VerifyModeFingerprint
	case 4:
		return entity.VerifyModeFace
	case 15:
		return entity.VerifyModeCard
	default:
		return entity.VerifyModeFingerprint
	}
}
