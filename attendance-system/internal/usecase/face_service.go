package usecase

import (
	"context"
	"fmt"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/infrastructure/broadcast"
)

type FaceService struct {
	faceRepo            port.FaceRepository
	employeeRepo        port.EmployeeRepository
	attendanceRepo      port.AttendanceLogRepository
	attendanceProcessor *AttendanceProcessorService
}

func NewFaceService(
	faceRepo port.FaceRepository,
	employeeRepo port.EmployeeRepository,
	attendanceRepo port.AttendanceLogRepository,
	attendanceProcessor *AttendanceProcessorService,
) *FaceService {
	return &FaceService{
		faceRepo:            faceRepo,
		employeeRepo:        employeeRepo,
		attendanceRepo:      attendanceRepo,
		attendanceProcessor: attendanceProcessor,
	}
}

func (s *FaceService) RegisterFace(ctx context.Context, employeeID string, descriptor string) error {
	// Kiểm tra xem nhân viên có tồn tại hay không
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp == nil {
		return fmt.Errorf("employee not found")
	}

	// Lưu hoặc cập nhật khuôn mặt
	if err := s.faceRepo.Upsert(ctx, employeeID, descriptor); err != nil {
		return err
	}

	// Cập nhật trạng thái FaceEnrolled của nhân viên
	emp.FaceEnrolled = true
	if err := s.employeeRepo.Update(ctx, emp); err != nil {
		return err
	}

	// Gửi broadcast sự kiện cập nhật
	broadcast.Global.Broadcast("face_updated", map[string]any{
		"employee_id": employeeID,
		"action":      "enrolled",
	})

	return nil
}

func (s *FaceService) DeleteFace(ctx context.Context, employeeID string) error {
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp == nil {
		return fmt.Errorf("employee not found")
	}

	if err := s.faceRepo.Delete(ctx, employeeID); err != nil {
		return err
	}

	emp.FaceEnrolled = false
	if err := s.employeeRepo.Update(ctx, emp); err != nil {
		return err
	}

	broadcast.Global.Broadcast("face_updated", map[string]any{
		"employee_id": employeeID,
		"action":      "deleted",
	})

	return nil
}

func (s *FaceService) GetAllFaces(ctx context.Context) ([]entity.EmployeeFace, error) {
	return s.faceRepo.ListAll(ctx)
}

func (s *FaceService) SubmitFaceAttendance(ctx context.Context, employeeID string) (*entity.AttendanceLog, error) {
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if emp == nil {
		return nil, fmt.Errorf("employee not found")
	}

	now := time.Now()
	// Tạo bản ghi chấm công từ Camera
	log := entity.AttendanceLog{
		EmployeeCode: emp.EmployeeCode,
		CheckTime:    now,
		CheckType:    entity.CheckTypeIn, // Mặc định là check-in
		VerifyMode:   entity.VerifyModeFace,
		DeviceID:     "",                                    // Để trống để PostgreSQL nhận NULL (UUID)
		RawPayload:   []byte(`{"source":"CAMERA_LAPTOP"}`),  // JSON hợp lệ cho PostgreSQL column json/jsonb
		IsValid:      true,
		SyncedAt:     now,
	}

	inserted, err := s.attendanceRepo.BulkInsert(ctx, []entity.AttendanceLog{log})
	if err != nil {
		return nil, err
	}
	if inserted == 0 {
		return nil, fmt.Errorf("failed to insert attendance log")
	}

	// Query lại log vừa insert để lấy ID
	logs, err := s.attendanceRepo.Query(ctx, now.Add(-5*time.Second), now.Add(5*time.Second), emp.EmployeeCode, "")
	var savedLog entity.AttendanceLog
	if err == nil && len(logs) > 0 {
		savedLog = logs[len(logs)-1]
	} else {
		savedLog = log
	}

	// Tính toán lại công ngày cho nhân viên
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	go func() {
		// Chạy trong goroutine để tránh chặn HTTP response chính
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.attendanceProcessor.ProcessDailyAttendanceForEmployee(bgCtx, emp.ID, today)
	}()

	// Đẩy sự kiện SSE thông báo chấm công thành công real-time
	broadcast.Global.Broadcast("attendance_synced", map[string]any{
		"device_id":   "CAMERA_LAPTOP",
		"new_records": 1,
	})

	// Gửi broadcast đồng bộ với giao diện quẹt vân tay (Cập nhật LastScanCard & Dashboard)
	broadcast.Global.Broadcast("attendance_logged", map[string]any{
		"employee_id":     emp.ID,
		"employee_code":   emp.EmployeeCode,
		"full_name":       emp.FullName,
		"avatar_url":      emp.AvatarURL,
		"department_name": emp.JobTitle,
		"role":            emp.JobTitle,
		"check_time":      now.Format("15:04:05"),
		"check_type":      "IN",
		"verify_mode":     "face",
		"device_name":     "Camera Laptop",
	})

	// Gửi broadcast riêng cho popup chào mừng của Camera
	broadcast.Global.Broadcast("face_attendance_detected", map[string]any{
		"employee_id":     emp.ID,
		"employee_code":   emp.EmployeeCode,
		"full_name":       emp.FullName,
		"avatar_url":      emp.AvatarURL,
		"department_name": emp.JobTitle,
		"check_time":      now.Format("15:04:05"),
	})

	return &savedLog, nil
}
