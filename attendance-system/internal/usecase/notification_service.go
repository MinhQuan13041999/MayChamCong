package usecase

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"attendance-system/internal/config"
	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

type ShiftResolver interface {
	ResolveShiftForEmployeeOnDate(ctx context.Context, employeeID string, date time.Time) (*entity.Shift, *entity.EmployeeShift, error)
}

// NotificationService gửi thông báo chấm công và kiểm tra nhân viên thiếu check-out.
// Email sử dụng SMTP; Zalo sử dụng Zalo OA API (recipient là ZaloUserID của nhân viên).
type NotificationService struct {
	employeeRepo   port.EmployeeRepository
	shiftRepo      port.ShiftRepository
	empShiftRepo   port.EmployeeShiftRepository
	attendanceRepo port.AttendanceLogRepository
	cfg            config.NotificationConfig
	httpClient     *http.Client
	now            func() time.Time
	resolver       ShiftResolver

	mu             sync.Mutex
	attendanceSent map[string]time.Time
	checkoutSent   map[string]time.Time
}

func NewNotificationService(
	employeeRepo port.EmployeeRepository,
	shiftRepo port.ShiftRepository,
	empShiftRepo port.EmployeeShiftRepository,
	attendanceRepo port.AttendanceLogRepository,
	cfg config.NotificationConfig,
) *NotificationService {
	if cfg.CheckoutGraceMinutes <= 0 {
		cfg.CheckoutGraceMinutes = 30
	}
	if cfg.InstantMaxAgeMinutes <= 0 {
		cfg.InstantMaxAgeMinutes = 10
	}
	return &NotificationService{
		employeeRepo:   employeeRepo,
		shiftRepo:      shiftRepo,
		empShiftRepo:   empShiftRepo,
		attendanceRepo: attendanceRepo,
		cfg:            cfg,
		httpClient:     &http.Client{Timeout: 15 * time.Second},
		now:            time.Now,
		attendanceSent: make(map[string]time.Time),
		checkoutSent:   make(map[string]time.Time),
	}
}

func (s *NotificationService) SetResolver(r ShiftResolver) {
	s.resolver = r
}

// NotifyAttendanceLogs gửi thông báo cho các log mới sau khi thiết bị đồng bộ thành công.
// Hàm được gọi ở background để không làm chậm việc nhận log từ thiết bị.
func (s *NotificationService) NotifyAttendanceLogs(ctx context.Context, logs []entity.AttendanceLog) {
	if s == nil || !s.cfg.Enabled {
		return
	}
	for _, log := range logs {
		if log.EmployeeCode == "" || log.CheckTime.IsZero() {
			continue
		}
		age := s.now().Sub(log.CheckTime)
		if age > time.Duration(s.cfg.InstantMaxAgeMinutes)*time.Minute || age < -5*time.Minute {
			// Không gửi hàng loạt thông báo cho log lịch sử khi thiết bị được
			// kết nối hoặc đồng bộ lần đầu.
			continue
		}
		key := attendanceNotificationKey(log)
		if !s.markOnce(s.attendanceSent, key) {
			continue
		}
		emp, err := s.employeeRepo.GetByCode(ctx, log.EmployeeCode)
		if err != nil || emp == nil {
			if err != nil {
				zap.L().Warn("attendance notification: employee lookup failed", zap.String("employee_code", log.EmployeeCode), zap.Error(err))
			}
			continue
		}
		message := attendanceMessage(emp.FullName, log)
		if _, err := s.sendEmployee(ctx, emp, message); err != nil {
			zap.L().Warn("attendance notification failed", zap.String("employee_code", emp.EmployeeCode), zap.Error(err))
		}
	}
}

// CheckMissingCheckouts gửi một cảnh báo cho mỗi nhân viên sau giờ kết thúc ca 30 phút
// nếu trong ngày chưa có log check-out. Scheduler gọi hàm này định kỳ mỗi phút.
func (s *NotificationService) CheckMissingCheckouts(ctx context.Context) error {
	if s == nil || !s.cfg.Enabled || s.employeeRepo == nil || s.shiftRepo == nil || s.empShiftRepo == nil || s.attendanceRepo == nil {
		return nil
	}
	employees, err := s.employeeRepo.ListActive(ctx)
	if err != nil {
		return err
	}
	now := s.now()
	for _, emp := range employees {
		var shift *entity.Shift
		var assignment *entity.EmployeeShift

		if s.resolver != nil {
			shift, assignment, err = s.resolver.ResolveShiftForEmployeeOnDate(ctx, emp.ID, now)
		} else {
			assignment, err = s.empShiftRepo.GetActiveShiftForEmployee(ctx, emp.ID, now)
			if err == nil && assignment != nil && assignment.ShiftID != nil {
				shift, err = s.shiftRepo.GetByID(ctx, *assignment.ShiftID)
			}
		}

		if err != nil || shift == nil || assignment == nil {
			continue
		}
		loc := time.Local
		if strings.TrimSpace(shift.Timezone) != "" {
			if loaded, loadErr := time.LoadLocation(shift.Timezone); loadErr == nil {
				loc = loaded
			}
		}
		localNow := now.In(loc)
		shiftStart, startOK := shiftBoundary(localNow, shift.StartTime)
		shiftEnd, endOK := shiftBoundary(localNow, shift.EndTime)
		if !startOK || !endOK {
			continue
		}
		if !shiftEnd.After(shiftStart) {
			if localNow.Before(shiftStart) {
				shiftStart = shiftStart.Add(-24 * time.Hour)
			} else {
				shiftEnd = shiftEnd.Add(24 * time.Hour)
			}
		}
		deadline := shiftEnd.Add(time.Duration(s.cfg.CheckoutGraceMinutes) * time.Minute)
		if localNow.Before(deadline) {
			continue
		}

		dayKey := localNow.Format("2006-01-02")
		key := emp.ID + ":" + dayKey + ":" + shift.ID
		if s.wasSent(s.checkoutSent, key) {
			continue
		}
		logs, queryErr := s.attendanceRepo.Query(ctx, shiftStart, now, emp.EmployeeCode, "")
		if queryErr != nil {
			zap.L().Warn("checkout reminder: attendance lookup failed", zap.String("employee_code", emp.EmployeeCode), zap.Error(queryErr))
			continue
		}
		hasCheckin := false
		hasCheckout := false
		for _, log := range logs {
			if log.CheckType == entity.CheckTypeOut {
				hasCheckout = true
			} else if log.CheckType == entity.CheckTypeIn || log.CheckType == entity.CheckTypeUnknown {
				hasCheckin = true
			}
		}
		if !hasCheckin || hasCheckout {
			continue
		}
		message := fmt.Sprintf("Cảnh báo: %s chưa ghi nhận check-out sau khi ca %s kết thúc lúc %s. Vui lòng kiểm tra lại.", emp.FullName, shift.Name, shiftEnd.In(loc).Format("15:04"))
		sent, sendErr := s.sendEmployee(ctx, &emp, message)
		if sent {
			s.recordSent(s.checkoutSent, key)
		}
		if sendErr != nil {
			zap.L().Warn("checkout reminder failed", zap.String("employee_code", emp.EmployeeCode), zap.Error(sendErr))
		}
	}
	return nil
}

func (s *NotificationService) markOnce(store map[string]time.Time, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sentAt, ok := store[key]; ok && s.now().Sub(sentAt) < 24*time.Hour {
		return false
	}
	store[key] = s.now()
	return true
}

func (s *NotificationService) wasSent(store map[string]time.Time, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sentAt, ok := store[key]
	return ok && s.now().Sub(sentAt) < 24*time.Hour
}

func (s *NotificationService) recordSent(store map[string]time.Time, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	store[key] = s.now()
}

func (s *NotificationService) sendEmployee(ctx context.Context, emp *entity.Employee, message string) (bool, error) {
	sent := false
	var errs []string
	if s.cfg.EmailEnabled && strings.TrimSpace(emp.Email) != "" {
		if err := s.sendEmail(emp.Email, message); err != nil {
			errs = append(errs, "email: "+err.Error())
		} else {
			sent = true
		}
	}
	if s.cfg.ZaloEnabled && strings.TrimSpace(emp.ZaloUserID) != "" {
		if err := s.sendZalo(ctx, emp.ZaloUserID, message); err != nil {
			errs = append(errs, "zalo: "+err.Error())
		} else {
			sent = true
		}
	}
	if !sent && len(errs) == 0 {
		return false, errors.New("no notification channel configured for employee")
	}
	if len(errs) > 0 {
		return sent, errors.New(strings.Join(errs, "; "))
	}
	return sent, nil
}

func (s *NotificationService) sendEmail(to, message string) error {
	return s.SendEmailWithAttachment(to, "Thông báo chấm công", message, "", "", nil)
}

// SendEmailWithAttachment gửi email SMTP với nội dung và một file đính kèm tùy chọn.
// Hàm được dùng chung cho thông báo tức thời và báo cáo tháng.
func (s *NotificationService) SendEmailWithAttachment(to, subject, message, filename, contentType string, attachment []byte) error {
	if !s.cfg.EmailEnabled {
		return errors.New("email notifications are disabled")
	}
	host := strings.TrimSpace(s.cfg.SMTPHost)
	from := strings.TrimSpace(s.cfg.SMTPFrom)
	if host == "" || from == "" {
		return errors.New("SMTP host/from is not configured")
	}
	parsedTo, err := mail.ParseAddress(strings.TrimSpace(to))
	if err != nil || parsedTo.Address != strings.TrimSpace(to) {
		return errors.New("invalid recipient email address")
	}
	parsedFrom, err := mail.ParseAddress(from)
	if err != nil || parsedFrom.Address != from {
		return errors.New("invalid sender email address")
	}
	port := s.cfg.SMTPPort
	if port <= 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	var auth smtp.Auth
	if s.cfg.SMTPUsername != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, host)
	}

	var body bytes.Buffer
	body.WriteString("From: " + parsedFrom.Address + "\r\n")
	body.WriteString("To: " + parsedTo.Address + "\r\n")
	body.WriteString("Subject: " + mime.BEncoding.Encode("UTF-8", sanitizeHeaderValue(subject)) + "\r\n")
	if len(attachment) == 0 {
		body.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
		body.WriteString(message)
	} else {
		boundary := fmt.Sprintf("=_attendance_%d", time.Now().UnixNano())
		body.WriteString("MIME-Version: 1.0\r\n")
		body.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n\r\n")
		writer := multipart.NewWriter(&body)
		if err := writer.SetBoundary(boundary); err != nil {
			return err
		}
		textPart, err := writer.CreatePart(map[string][]string{
			"Content-Type":              {"text/plain; charset=UTF-8"},
			"Content-Transfer-Encoding": {"8bit"},
		})
		if err != nil {
			return err
		}
		if _, err := textPart.Write([]byte(message + "\r\n")); err != nil {
			return err
		}
		if contentType == "" {
			contentType = "text/csv"
		}
		attachmentPart, err := writer.CreatePart(map[string][]string{
			"Content-Type":              {contentType + "; name=\"" + filename + "\""},
			"Content-Disposition":       {"attachment; filename=\"" + filename + "\""},
			"Content-Transfer-Encoding": {"base64"},
		})
		if err != nil {
			return err
		}
		encoder := base64.NewEncoder(base64.StdEncoding, attachmentPart)
		if _, err := encoder.Write(attachment); err != nil {
			return err
		}
		if err := encoder.Close(); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
	}
	return smtp.SendMail(addr, auth, parsedFrom.Address, []string{parsedTo.Address}, body.Bytes())
}

func sanitizeHeaderValue(value string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(value)
}

func (s *NotificationService) sendZalo(ctx context.Context, recipientID, message string) error {
	endpoint := strings.TrimSpace(s.cfg.ZaloAPIURL)
	if endpoint == "" {
		endpoint = "https://openapi.zalo.me/v3.0/oa/message/cs"
	}
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return fmt.Errorf("invalid Zalo API URL: %w", err)
	}
	token := strings.TrimSpace(s.cfg.ZaloAccessToken)
	if token == "" {
		return errors.New("Zalo OA access token is not configured")
	}
	payload := map[string]any{
		"recipient": map[string]string{"user_id": recipientID},
		"message":   map[string]string{"text": message},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("access_token", token)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Zalo API returned HTTP %d", resp.StatusCode)
	}
	var zaloResponse struct {
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&zaloResponse); err == nil && zaloResponse.Error != 0 {
		return fmt.Errorf("Zalo API error %d: %s", zaloResponse.Error, zaloResponse.Message)
	}
	return nil
}

func attendanceMessage(name string, log entity.AttendanceLog) string {
	action := "chấm công"
	if log.CheckType == entity.CheckTypeIn {
		action = "check-in"
	} else if log.CheckType == entity.CheckTypeOut {
		action = "check-out"
	}
	return fmt.Sprintf("Chào %s, bạn đã %s thành công lúc %s.", name, action, log.CheckTime.Local().Format("15:04"))
}

func shiftBoundary(day time.Time, value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"15:04", "15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return time.Date(day.Year(), day.Month(), day.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, day.Location()), true
		}
	}
	return time.Time{}, false
}

func attendanceNotificationKey(log entity.AttendanceLog) string {
	t := log.CheckTime.UTC().Truncate(time.Microsecond)
	return log.DeviceID + ":" + log.EmployeeCode + ":" + t.Format(time.RFC3339Nano)
}

// filterNewAttendanceLogs removes records that were already persisted. It keeps
// overlapping 10-second SDK polls from sending duplicate notifications.
func filterNewAttendanceLogs(ctx context.Context, repo port.AttendanceLogRepository, logs []entity.AttendanceLog) []entity.AttendanceLog {
	if repo == nil || len(logs) == 0 {
		return logs
	}
	from, to := logs[0].CheckTime, logs[0].CheckTime
	for _, log := range logs[1:] {
		if log.CheckTime.Before(from) {
			from = log.CheckTime
		}
		if log.CheckTime.After(to) {
			to = log.CheckTime
		}
	}
	existing, err := repo.Query(ctx, from.Add(-time.Microsecond), to.Add(time.Microsecond), "", "")
	if err != nil {
		return logs
	}
	existingKeys := make(map[string]struct{}, len(existing))
	for _, log := range existing {
		existingKeys[attendanceNotificationKey(log)] = struct{}{}
	}
	seen := make(map[string]struct{}, len(logs))
	result := make([]entity.AttendanceLog, 0, len(logs))
	for _, log := range logs {
		key := attendanceNotificationKey(log)
		if _, ok := existingKeys[key]; ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, log)
	}
	return result
}
