package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/interface/dto"
	"attendance-system/internal/usecase"
)

// DeviceHandler chỉ đảm nhiệm decode request / gọi Service / encode response.
// Không chứa business logic.
type DeviceHandler struct {
	service          *usecase.DeviceService
	processor        *usecase.AttendanceProcessorService
	biometricService *usecase.BiometricService
}

func NewDeviceHandler(service *usecase.DeviceService, processor *usecase.AttendanceProcessorService) *DeviceHandler {
	return &DeviceHandler{service: service, processor: processor}
}

func (h *DeviceHandler) SetBiometricService(biometricService *usecase.BiometricService) {
	h.biometricService = biometricService
}

func (h *DeviceHandler) Routes(r chi.Router) {
	r.Get("/devices", h.List)
	r.Get("/devices/{id}/status", h.Status)
	r.Get("/devices/{id}/debug-queue", h.DebugQueue)

	// Các thao tác quản trị yêu cầu quyền admin
	r.Group(func(adminOnly chi.Router) {
		adminOnly.Use(RequireRole("admin"))
		adminOnly.Post("/devices", h.Create)
		adminOnly.Put("/devices/{id}", h.Update)
		adminOnly.Delete("/devices/{id}", h.Delete)
		adminOnly.Post("/devices/{id}/test-connection", h.TestConnection)
		adminOnly.Post("/devices/{id}/reboot", h.Reboot)
		adminOnly.Post("/devices/{id}/cancel-pending", h.CancelPendingCommands)
		adminOnly.Post("/devices/{id}/clear-logs", h.ClearLogs)
		adminOnly.Post("/devices/{id}/reset", h.ResetDevice)
	})
}

func (h *DeviceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	d := &entity.Device{
		Name:             req.Name,
		DeviceType:       entity.DeviceType(req.DeviceType),
		IPAddress:        req.IPAddress,
		Port:             req.Port,
		SerialNumber:     req.SerialNumber,
		SerialNumberADMS: req.SerialNumberADMS,
		ADMSEnabled:      req.ADMSEnabled,
		Location:         req.Location,
		FirmwareVersion:  req.FirmwareVersion,
		MacAddress:       req.MacAddress,
		Username:         req.Username,
		Password:         req.Password,
		Status:           "offline",
	}
	if err := h.service.CreateDevice(r.Context(), d); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "CREATE_DEVICE", "device", d.ID,
		fmt.Sprintf("Created device '%s' (IP: %s, SN: %s)", d.Name, d.IPAddress, d.SerialNumber), r.RemoteAddr)

	writeJSON(w, http.StatusCreated, d)
}

func (h *DeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dto.UpdateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	d := &entity.Device{
		ID:               id,
		Name:             req.Name,
		DeviceType:       entity.DeviceType(req.DeviceType),
		IPAddress:        req.IPAddress,
		Port:             req.Port,
		SerialNumber:     req.SerialNumber,
		SerialNumberADMS: req.SerialNumberADMS,
		ADMSEnabled:      req.ADMSEnabled,
		Location:         req.Location,
		FirmwareVersion:  req.FirmwareVersion,
		MacAddress:       req.MacAddress,
		Username:         req.Username,
		Password:         req.Password,
	}
	if err := h.service.UpdateDevice(r.Context(), d); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "UPDATE_DEVICE", "device", id,
		fmt.Sprintf("Updated device '%s' details", d.Name), r.RemoteAddr)

	writeJSON(w, http.StatusOK, d)
}

func (h *DeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteDevice(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "DELETE_DEVICE", "device", id,
		"Deleted device", r.RemoteAddr)

	w.WriteHeader(http.StatusNoContent)
}

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	devices, err := h.service.ListDevices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (h *DeviceHandler) Status(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	status, err := h.service.TestConnection(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *DeviceHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	status, err := h.service.TestConnection(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "TEST_CONNECTION_DEVICE", "device", id,
		fmt.Sprintf("Tested connection to device: online=%t, users=%d, logs=%d, firmware=%s",
			status.Online, status.UserCount, status.LogCount, status.FirmwareInfo), r.RemoteAddr)

	writeJSON(w, http.StatusOK, status)
}

func (h *DeviceHandler) Reboot(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.RebootDevice(r.Context(), id); err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "REBOOT_DEVICE", "device", id,
		"Rebooted device remotely", r.RemoteAddr)

	w.WriteHeader(http.StatusAccepted)
}

func (h *DeviceHandler) ClearLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.biometricService == nil {
		http.Error(w, "biometric service not initialized", http.StatusInternalServerError)
		return
	}

	if err := h.biometricService.ClearDeviceLog(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "CLEAR_DEVICE_LOGS", "device", id,
		"Sent clear logs command to device", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Clear logs command sent to device successfully"})
}

func (h *DeviceHandler) ResetDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.biometricService == nil {
		http.Error(w, "biometric service not initialized", http.StatusInternalServerError)
		return
	}

	if err := h.biometricService.ResetDevice(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Ghi audit log
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "RESET_DEVICE", "device", id,
		"Sent reset command to device", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Device reset command sent successfully"})
}

func (h *DeviceHandler) CancelPendingCommands(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.service == nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("service not configured"))
		return
	}
	count, err := h.service.CancelPendingCommands(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.processor.CreateAuditLog(r.Context(), getUserID(r), "CANCEL_PENDING_COMMANDS", "device", id,
		fmt.Sprintf("Cancelled %d pending commands for device", count), r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": count})
}

// DebugQueue trả về các lệnh pending cho thiết bị (dành cho debug)
func (h *DeviceHandler) DebugQueue(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// Sử dụng service để lấy pending commands nếu có
	if h.service == nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("service not configured"))
		return
	}
	cmds, err := h.service.ListPendingCommands(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cmds)
}
