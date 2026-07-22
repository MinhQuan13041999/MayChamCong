// Package hikvision implements port.DeviceAdapter for Hikvision devices via ISAPI.
//
// Hikvision ISAPI là HTTP/JSON REST API chạy trực tiếp trên thiết bị.
// Xác thực bằng HTTP Digest Auth. Không cần SDK/DLL chính hãng.
package hikvision

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

// Hikvision ISAPI response types — chỉ dùng trong package này.
type hikSystemStatus struct {
	DeviceInfo struct {
		FirmwareVersion string `json:"firmwareVersion"`
	} `json:"DeviceInfo"`
}

type hikUserInfoList struct {
	UserInfo []struct {
		EmployeeNo string `json:"employeeNo"`
		Name       string `json:"name"`
		CardNo     string `json:"cardNo,omitempty"`
	} `json:"UserInfo"`
}

type hikAcsEvent struct {
	AcsEventCond struct {
		SearchResultPosition int    `json:"searchResultPosition"`
		MaxResults           int    `json:"maxResults"`
		Major                int    `json:"major"`
		Minor                int    `json:"minor"`
		StartTime            string `json:"startTime"`
		EndTime              string `json:"endTime"`
	} `json:"AcsEventCond"`
}

type hikAcsEventResp struct {
	AcsEvent struct {
		InfoList []struct {
			EmployeeNoString string `json:"employeeNoString"`
			Time             string `json:"time"` // "2006-01-02T15:04:05+07:00"
			MajorEventType   int    `json:"majorEventType"`
			SubEventType     int    `json:"subEventType"`
		} `json:"InfoList"`
	} `json:"AcsEvent"`
}

// Adapter implement port.DeviceAdapter cho Hikvision qua ISAPI (HTTP Digest Auth).
type Adapter struct {
	cfg        port.DeviceConfig
	baseURL    string
	httpClient *http.Client
}

// New tạo instance mới cho Hikvision adapter.
func New() *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (a *Adapter) Connect(ctx context.Context, cfg port.DeviceConfig) error {
	a.cfg = cfg
	if cfg.IPAddress == "" {
		return fmt.Errorf("hikvision: ip_address is required")
	}
	port := cfg.Port
	if port == 0 {
		port = 80
	}
	a.baseURL = fmt.Sprintf("http://%s:%d", cfg.IPAddress, port)

	// Ping thiết bị qua ISAPI /System/deviceInfo
	_, err := a.doRequest(ctx, http.MethodGet, "/ISAPI/System/deviceInfo", nil)
	if err != nil {
		return fmt.Errorf("hikvision: cannot connect to %s: %w", a.baseURL, err)
	}
	return nil
}

func (a *Adapter) Disconnect(ctx context.Context) error {
	// ISAPI là stateless — không cần teardown
	return nil
}

func (a *Adapter) CheckStatus(ctx context.Context) (port.DeviceStatus, error) {
	body, err := a.doRequest(ctx, http.MethodGet, "/ISAPI/System/deviceInfo", nil)
	if err != nil {
		return port.DeviceStatus{}, fmt.Errorf("hikvision check status: %w", err)
	}
	var status hikSystemStatus
	_ = json.Unmarshal(body, &status)
	return port.DeviceStatus{
		Online:       true,
		FirmwareInfo: status.DeviceInfo.FirmwareVersion,
	}, nil
}

func (a *Adapter) SyncTime(ctx context.Context) error {
	// PUT /ISAPI/System/time với time format ISO 8601
	payload := fmt.Sprintf(`{"Time":{"localTime":"%s","timeMode":"manual"}}`,
		time.Now().Format("2006-01-02T15:04:05"))
	_, err := a.doRequest(ctx, http.MethodPut, "/ISAPI/System/time", []byte(payload))
	if err != nil {
		return fmt.Errorf("hikvision sync time: %w", err)
	}
	return nil
}

func (a *Adapter) GetEmployees(ctx context.Context) ([]entity.Employee, error) {
	body, err := a.doRequest(ctx, http.MethodGet, "/ISAPI/AccessControl/UserInfo/Search?format=json", nil)
	if err != nil {
		return nil, fmt.Errorf("hikvision get employees: %w", err)
	}

	var result hikUserInfoList
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("hikvision parse employees: %w", err)
	}

	employees := make([]entity.Employee, 0, len(result.UserInfo))
	for _, u := range result.UserInfo {
		employees = append(employees, entity.Employee{
			EmployeeCode: u.EmployeeNo,
			FullName:     u.Name,
			CardNo:       u.CardNo,
		})
	}
	return employees, nil
}

func (a *Adapter) PushEmployee(ctx context.Context, emp entity.Employee) error {
	payload := map[string]interface{}{
		"UserInfo": map[string]interface{}{
			"employeeNo":        emp.EmployeeCode,
			"name":              emp.FullName,
			"userType":          "normal",
			"belVeficationMode": "idle",
			"password":          "",
		},
	}
	body, _ := json.Marshal(payload)
	_, err := a.doRequest(ctx, http.MethodPost, "/ISAPI/AccessControl/UserInfo/Record?format=json", body)
	if err != nil {
		return fmt.Errorf("hikvision push employee %s: %w", emp.EmployeeCode, err)
	}
	return nil
}

func (a *Adapter) PushFingerprint(ctx context.Context, employeeCode string, fp entity.EmployeeFingerprint) error {
	// Hikvision ISAPI không được cấu hình cho việc đẩy vân tay thô trực tiếp trong adapter này
	return fmt.Errorf("hikvision: PushFingerprint is not supported via ISAPI in this adapter")
}

func (a *Adapter) GetFingerprint(ctx context.Context, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	return nil, fmt.Errorf("hikvision: GetFingerprint is not supported via ISAPI")
}

func (a *Adapter) GetAttendanceLogs(ctx context.Context, from, to time.Time) ([]entity.AttendanceLog, error) {
	reqBody := hikAcsEvent{}
	reqBody.AcsEventCond.SearchResultPosition = 0
	reqBody.AcsEventCond.MaxResults = 1000
	reqBody.AcsEventCond.Major = 5 // 5 = Access Control events
	reqBody.AcsEventCond.Minor = 0
	reqBody.AcsEventCond.StartTime = from.Format("2006-01-02T15:04:05") + "+07:00"
	reqBody.AcsEventCond.EndTime = to.Format("2006-01-02T15:04:05") + "+07:00"

	bodyBytes, _ := json.Marshal(reqBody)
	respBytes, err := a.doRequest(ctx, http.MethodPost,
		"/ISAPI/AccessControl/AcsEvent?format=json", bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("hikvision get attendance: %w", err)
	}

	var result hikAcsEventResp
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("hikvision parse attendance: %w", err)
	}

	var logs []entity.AttendanceLog
	for _, info := range result.AcsEvent.InfoList {
		t, err := time.Parse(time.RFC3339, info.Time)
		if err != nil {
			t, err = time.ParseInLocation("2006-01-02T15:04:05", info.Time, time.Local)
			if err != nil {
				continue
			}
		}
		rawBytes, _ := json.Marshal(info)
		logs = append(logs, entity.AttendanceLog{
			EmployeeCode: info.EmployeeNoString,
			CheckTime:    t,
			CheckType:    mapHikCheckType(info.SubEventType),
			VerifyMode:   entity.VerifyModeCard,
			RawPayload:   rawBytes,
		})
	}
	return logs, nil
}

func (a *Adapter) ClearAttendanceLogs(ctx context.Context) error {
	// Hikvision ISAPI không hỗ trợ xoá log chấm công
	return fmt.Errorf("hikvision: ClearAttendanceLogs not supported by ISAPI")
}

func (a *Adapter) ClearEmployees(ctx context.Context) error {
	return fmt.Errorf("hikvision: ClearEmployees not supported by ISAPI")
}

func (a *Adapter) DeleteEmployee(ctx context.Context, employeeCode string) error {
	return fmt.Errorf("hikvision: deleting one employee through the SDK is not supported by ISAPI")
}

func (a *Adapter) DeleteFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error {
	return fmt.Errorf("hikvision: deleting fingerprints through the SDK is not supported by ISAPI")
}

func (a *Adapter) Reboot(ctx context.Context) error {
	_, err := a.doRequest(ctx, http.MethodPut, "/ISAPI/System/reboot", nil)
	if err != nil {
		return fmt.Errorf("hikvision reboot: %w", err)
	}
	return nil
}

func (a *Adapter) Reset(ctx context.Context) error {
	return fmt.Errorf("hikvision: Reset not supported")
}

func (a *Adapter) EnrollFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error {
	return fmt.Errorf("hikvision: EnrollFingerprint not supported")
}

// ---- HTTP Digest Auth helper ----

// doRequest thực hiện HTTP request với Digest Authentication đơn giản.
// Lần đầu gửi không có auth → nhận 401 kèm WWW-Authenticate → tính digest → gửi lại.
func (a *Adapter) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	url := a.baseURL + path

	// Lần 1: gửi request không auth để lấy challenge
	req1, _ := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := a.httpClient.Do(req1)
	if err != nil {
		return nil, err
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusUnauthorized {
		// Thiết bị không yêu cầu auth hoặc đã ok
		if resp1.StatusCode >= 400 {
			return nil, fmt.Errorf("HTTP %d from %s", resp1.StatusCode, path)
		}
		return io.ReadAll(resp1.Body)
	}

	// Lần 2: parse WWW-Authenticate, tính digest response
	authHeader := resp1.Header.Get("WWW-Authenticate")
	authVal := buildDigestAuth(a.cfg.Username, a.cfg.Password, method, path, authHeader)

	req2, _ := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", authVal)
	resp2, err := a.httpClient.Do(req2)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()

	if resp2.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s (after digest auth)", resp2.StatusCode, path)
	}
	return io.ReadAll(resp2.Body)
}

// buildDigestAuth tính chuỗi Authorization cho HTTP Digest Auth (RFC 2617, đơn giản hoá).
func buildDigestAuth(username, password, method, uri, wwwAuth string) string {
	params := parseDigestParams(wwwAuth)
	realm := params["realm"]
	nonce := params["nonce"]
	qop := params["qop"]

	ha1 := md5hex(username + ":" + realm + ":" + password)
	ha2 := md5hex(method + ":" + uri)

	var response string
	nc := "00000001"
	cnonce := "abcdef01"
	if qop == "auth" {
		response = md5hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)
		return fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", nc=%s, cnonce="%s", qop=%s, response="%s"`,
			username, realm, nonce, uri, nc, cnonce, qop, response)
	}
	response = md5hex(ha1 + ":" + nonce + ":" + ha2)
	return fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		username, realm, nonce, uri, response)
}

func parseDigestParams(header string) map[string]string {
	params := make(map[string]string)
	header = strings.TrimPrefix(header, "Digest ")
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			params[strings.TrimSpace(kv[0])] = strings.Trim(strings.TrimSpace(kv[1]), `"`)
		}
	}
	return params
}

func md5hex(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

func mapHikCheckType(subEventType int) entity.CheckType {
	switch subEventType {
	case 75: // cardReaderAuthenticationPassedNoRecord (entry)
		return entity.CheckTypeIn
	case 76:
		return entity.CheckTypeOut
	default:
		return entity.CheckTypeUnknown
	}
}
