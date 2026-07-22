package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xuri/excelize/v2"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/interface/dto"
	"attendance-system/internal/usecase"
)

type EmployeeHandler struct {
	service          *usecase.EmployeeService
	processor        *usecase.AttendanceProcessorService
	biometricService *usecase.BiometricService
}

func NewEmployeeHandler(service *usecase.EmployeeService, processor *usecase.AttendanceProcessorService, biometricService *usecase.BiometricService) *EmployeeHandler {
	return &EmployeeHandler{service: service, processor: processor, biometricService: biometricService}
}

func (h *EmployeeHandler) Routes(r chi.Router) {
	r.Get("/employees", h.List)
	r.Get("/employees/{id}/device-mappings", h.ListDeviceMappings)

	// Các thao tác quản trị yêu cầu quyền admin
	r.Group(func(adminOnly chi.Router) {
		adminOnly.Use(RequireRole("admin"))
		adminOnly.Post("/employees", h.Create)
		adminOnly.Put("/employees/{id}", h.Update)
		adminOnly.Delete("/employees/{id}", h.Delete)
		adminOnly.Post("/employees/import", h.Import)
		adminOnly.Post("/employees/batch-enroll", h.BatchEnroll)
		adminOnly.Post("/devices/{id}/sync-employees", h.SyncToDevice)
		adminOnly.Post("/devices/{id}/pull-employees", h.PullFromDevice)
		adminOnly.Post("/employees/{id}/devices/{deviceID}/sync", h.SyncEmployeeToDevice)
		adminOnly.Post("/employees/{id}/devices/{deviceID}/fingerprint-confirm", h.ConfirmFingerprint)
		adminOnly.Post("/employees/{id}/push-to-all-devices", h.PushToAllDevices)
		adminOnly.Post("/employees/{id}/clear-enroll-data", h.ClearEnrollData)
	})
}

func (h *EmployeeHandler) ListDeviceMappings(w http.ResponseWriter, r *http.Request) {
	mappings, err := h.service.ListDeviceMappings(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, mappings)
}

func (h *EmployeeHandler) SyncEmployeeToDevice(w http.ResponseWriter, r *http.Request) {
	var req dto.SyncEmployeeToDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	employeeID, deviceID := chi.URLParam(r, "id"), chi.URLParam(r, "deviceID")
	mapping, err := h.service.SyncEmployeeToDevice(r.Context(), employeeID, deviceID, req.DeviceUserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "SYNC_EMPLOYEE_TO_DEVICE", "employee", employeeID, fmt.Sprintf("Synced employee to device %s as user %s", deviceID, req.DeviceUserID), r.RemoteAddr)
	writeJSON(w, http.StatusOK, mapping)
}

func (h *EmployeeHandler) ConfirmFingerprint(w http.ResponseWriter, r *http.Request) {
	employeeID, deviceID := chi.URLParam(r, "id"), chi.URLParam(r, "deviceID")
	if err := h.service.ConfirmFingerprintEnrolled(r.Context(), employeeID, deviceID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "CONFIRM_FINGERPRINT_ENROLLMENT", "employee", employeeID, fmt.Sprintf("Confirmed fingerprint enrollment on device %s", deviceID), r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Fingerprint enrollment confirmed"})
}

func (h *EmployeeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateEmployeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	e := &entity.Employee{
		EmployeeCode: req.EmployeeCode,
		FullName:     req.FullName,
		DepartmentID: req.DepartmentID,
		CardNo:       req.CardNo,
		Email:        req.Email,
		Phone:        req.Phone,
		Gender:       sanitizeGender(req.Gender),
		Dob:          parseDateString(req.Dob),
		JoinDate:     parseJoinDateString(req.JoinDate),
		JobTitle:     req.JobTitle,
		AvatarURL:    req.AvatarURL,
	}
	if req.EnrollFingerprint {
		if err := h.service.CreateEmployeeWithEnrollment(r.Context(), e, true, req.DeviceID, req.DeviceUserID); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	} else if err := h.service.CreateEmployee(r.Context(), e); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "CREATE_EMPLOYEE", "employee", e.ID,
		fmt.Sprintf("Created employee '%s' (Code: %s)", e.FullName, e.EmployeeCode), r.RemoteAddr)

	writeJSON(w, http.StatusCreated, e)
}

func (h *EmployeeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dto.UpdateEmployeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	e := &entity.Employee{
		ID:           id,
		FullName:     req.FullName,
		DepartmentID: req.DepartmentID,
		CardNo:       req.CardNo,
		Status:       req.Status,
		Email:        req.Email,
		Phone:        req.Phone,
		Gender:       sanitizeGender(req.Gender),
		Dob:          parseDateString(req.Dob),
		JoinDate:     parseJoinDateString(req.JoinDate),
		JobTitle:     req.JobTitle,
		AvatarURL:    req.AvatarURL,
	}
	if err := h.service.UpdateEmployee(r.Context(), e); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "UPDATE_EMPLOYEE", "employee", id,
		fmt.Sprintf("Updated employee '%s' profile details", e.FullName), r.RemoteAddr)

	writeJSON(w, http.StatusOK, e)
}

func (h *EmployeeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteEmployee(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "DELETE_EMPLOYEE", "employee", id,
		"Deleted employee profile", r.RemoteAddr)

	w.WriteHeader(http.StatusNoContent)
}

func (h *EmployeeHandler) List(w http.ResponseWriter, r *http.Request) {
	employees, err := h.service.ListEmployees(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, employees)
}

func (h *EmployeeHandler) Import(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("request too large (max 10MB)"))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("field 'file' is required"))
		return
	}
	defer file.Close()

	name := header.Filename
	if len(name) < 5 || name[len(name)-5:] != ".xlsx" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("only .xlsx files are supported"))
		return
	}

	xlsx, err := excelize.OpenReader(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid excel file: %w", err))
		return
	}
	defer xlsx.Close()

	sheetName := xlsx.GetSheetName(0)
	rows, err := xlsx.GetRows(sheetName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("cannot read sheet: %w", err))
		return
	}

	type importResult struct {
		Imported int      `json:"imported"`
		Failed   int      `json:"failed"`
		Errors   []string `json:"errors,omitempty"`
	}
	result := importResult{}

	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) == 0 || row[0] == "" {
			continue
		}

		// Định dạng file nhập Excel nâng cấp
		// Cột A: employee_code, Cột B: full_name, Cột C: card_no, Cột D: department_id
		// Cột E: email, Cột F: phone, Cột G: gender, Cột H: dob (YYYY-MM-DD), Cột I: job_title
		dobStr := cellValue(row, 7)
		var dob *time.Time
		if dobStr != "" {
			parsedDob := parseDateString(&dobStr)
			if parsedDob != nil {
				dob = parsedDob
			}
		}

		emp := &entity.Employee{
			EmployeeCode: cellValue(row, 0),
			FullName:     cellValue(row, 1),
			CardNo:       cellValue(row, 2),
			DepartmentID: cellValue(row, 3),
			Email:        cellValue(row, 4),
			Phone:        cellValue(row, 5),
			Gender:       cellValue(row, 6),
			Dob:          dob,
			JoinDate:     time.Now(),
			JobTitle:     cellValue(row, 8),
			Status:       "active",
		}
		if emp.EmployeeCode == "" || emp.FullName == "" {
			result.Failed++
			result.Errors = append(result.Errors,
				fmt.Sprintf("row %d: employee_code and full_name are required", i+1))
			continue
		}

		if err := h.service.CreateEmployee(r.Context(), emp); err != nil {
			result.Failed++
			result.Errors = append(result.Errors,
				fmt.Sprintf("row %d (%s): %s", i+1, emp.EmployeeCode, err.Error()))
		} else {
			result.Imported++
		}
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "IMPORT_EMPLOYEES", "employee", "bulk",
		fmt.Sprintf("Bulk imported employees: %d successful, %d failed", result.Imported, result.Failed), r.RemoteAddr)

	writeJSON(w, http.StatusOK, result)
}

func (h *EmployeeHandler) SyncToDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	hist, err := h.service.PushEmployeesToDevice(r.Context(), deviceID, entity.SyncTriggerManual)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "SYNC_EMPLOYEES_TO_DEVICE", "device", deviceID,
		"Synchronized employee records to device", r.RemoteAddr)

	writeJSON(w, http.StatusOK, dto.FromSyncHistory(hist))
}

// PushToAllDevices đẩy 1 nhân viên xuống TẤT CẢ thiết bị ADMS đang online.
// POST /employees/{id}/push-to-all-devices
func (h *EmployeeHandler) PushToAllDevices(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")
	successCount, errList, err := h.service.PushEmployeeToAllDevices(r.Context(), employeeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "PUSH_EMPLOYEE_TO_ALL_DEVICES", "employee", employeeID,
		fmt.Sprintf("Pushed employee to %d devices", successCount), r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{
		"success_count": successCount,
		"errors":        errList,
		"message":       fmt.Sprintf("Đã đưa vào queue %d thiết bị", successCount),
	})
}

// BatchEnroll gửi lệnh quét vân tay hàng loạt cho danh sách nhân viên xuống 1 thiết bị ADMS.
// POST /employees/batch-enroll
// Body: { "employee_ids": ["id1","id2",...], "device_id": "devID" }
func (h *EmployeeHandler) BatchEnroll(w http.ResponseWriter, r *http.Request) {
	var req dto.BatchEnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.EmployeeIDs) == 0 || req.DeviceID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("employee_ids and device_id are required"))
		return
	}
	if h.biometricService == nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("biometric service not configured"))
		return
	}
	enqueued, errList, err := h.biometricService.BatchEnroll(r.Context(), req.EmployeeIDs, req.DeviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "BATCH_ENROLL_FINGERPRINT", "device", req.DeviceID,
		fmt.Sprintf("Batch enroll %d employees fingerprint on device", enqueued), r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{
		"enqueued":        enqueued,
		"total_requested": len(req.EmployeeIDs),
		"errors":          errList,
		"message":         fmt.Sprintf("Đã đưa %d lệnh quét vân tay vào queue máy chấm công", enqueued),
	})
}

// PullFromDevice kéo danh sách nhân viên từ máy chấm công về và merge vào web.
// POST /devices/{id}/pull-employees
func (h *EmployeeHandler) PullFromDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	imported, existing, errList, err := h.service.PullEmployeesFromDevice(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "PULL_EMPLOYEES_FROM_DEVICE", "device", deviceID,
		fmt.Sprintf("Pulled employees from device: %d new, %d existing", imported, existing), r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{
		"imported": imported,
		"existing": existing,
		"errors":   errList,
		"message":  fmt.Sprintf("Kéo thành công: %d nhân viên mới, %d đã có", imported, existing),
	})
}

func (h *EmployeeHandler) ClearEnrollData(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")
	if h.biometricService == nil {
		http.Error(w, "biometric service not initialized", http.StatusInternalServerError)
		return
	}

	err := h.biometricService.ClearEnrollDataForEmployee(r.Context(), employeeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "CLEAR_EMPLOYEE_ENROLL_DATA", "employee", employeeID,
		"Cleared enroll data for employee on all ADMS devices", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Clear enroll data command sent to all devices successfully"})
}

func parseDateString(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil
	}
	return &t
}

func parseJoinDateString(s *string) time.Time {
	if s == nil || *s == "" {
		return time.Now()
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return time.Now()
	}
	return t
}

func cellValue(row []string, idx int) string {
	if idx < len(row) {
		return row[idx]
	}
	return ""
}

// sanitizeGender trả về giá trị gender hợp lệ theo CHECK constraint của DB.
// Giá trị trống hoặc không hợp lệ sẽ trả về "" để DB lưu NULL.
func sanitizeGender(g string) string {
	switch g {
	case "male", "female", "other":
		return g
	default:
		return ""
	}
}
