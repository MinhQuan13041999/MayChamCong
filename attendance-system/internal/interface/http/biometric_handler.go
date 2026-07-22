package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"attendance-system/internal/usecase"
)

type BiometricHandler struct {
	biometricService *usecase.BiometricService
}

func NewBiometricHandler(biometricService *usecase.BiometricService) *BiometricHandler {
	return &BiometricHandler{biometricService: biometricService}
}

func (h *BiometricHandler) Routes(r chi.Router) {
	r.Get("/employees/{id}/fingerprints", h.ListFingerprints)
	r.Delete("/employees/{id}/fingerprints/{finger_index}", h.DeleteFingerprint)
	r.Post("/employees/{id}/fingerprints/push", h.PushFingerprints)
	r.Post("/employees/{id}/fingerprints/enroll", h.EnrollFingerprint)
	r.Post("/employees/{id}/fingerprints/re-enroll", h.ReEnrollFingerprint)
	r.Post("/devices/{id}/backup", h.BackupDevice)
	r.Post("/devices/{id}/cancel-pending-commands", h.CancelPendingCommands)
}

func (h *BiometricHandler) ListFingerprints(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")
	fps, err := h.biometricService.ListFingerprints(r.Context(), employeeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, fps)
}

func (h *BiometricHandler) DeleteFingerprint(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")
	fingerIndexStr := chi.URLParam(r, "finger_index")

	fingerIndex, err := strconv.Atoi(fingerIndexStr)
	if err != nil || fingerIndex < 0 || fingerIndex > 9 {
		http.Error(w, "invalid finger index", http.StatusBadRequest)
		return
	}

	err = h.biometricService.DeleteFingerprint(r.Context(), employeeID, fingerIndex)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Fingerprint deleted successfully"})
}

func (h *BiometricHandler) PushFingerprints(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")
	err := h.biometricService.PropagateToAllDevices(r.Context(), employeeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Fingerprint push queued successfully to all online devices"})
}

func (h *BiometricHandler) EnrollFingerprint(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	var body struct {
		DeviceID    string `json:"device_id"`
		FingerIndex *int   `json:"finger_index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DeviceID == "" {
		http.Error(w, "invalid request body, missing device_id", http.StatusBadRequest)
		return
	}

	fingerIndex := 0
	if body.FingerIndex != nil {
		fingerIndex = *body.FingerIndex
		if fingerIndex < 0 || fingerIndex > 9 {
			http.Error(w, "invalid finger_index (must be 0-9)", http.StatusBadRequest)
			return
		}
	}

	err := h.biometricService.EnrollFingerprint(r.Context(), employeeID, body.DeviceID, fingerIndex)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Fingerprint enrollment command sent to device successfully"})
}

func (h *BiometricHandler) ReEnrollFingerprint(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	var body struct {
		DeviceID    string `json:"device_id"`
		FingerIndex *int   `json:"finger_index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DeviceID == "" {
		http.Error(w, "invalid request body, missing device_id", http.StatusBadRequest)
		return
	}

	fingerIndex := 0
	if body.FingerIndex != nil {
		fingerIndex = *body.FingerIndex
		if fingerIndex < 0 || fingerIndex > 9 {
			http.Error(w, "invalid finger_index (must be 0-9)", http.StatusBadRequest)
			return
		}
	}

	err := h.biometricService.ReEnrollFingerprint(r.Context(), employeeID, body.DeviceID, fingerIndex)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Fingerprint re-enroll command sent to device successfully"})
}

func (h *BiometricHandler) CancelPendingCommands(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	cancelled, err := h.biometricService.CancelPendingBatchEnroll(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"cancelled": cancelled,
		"message":   fmt.Sprintf("Đã hủy %d lệnh đang chờ trên thiết bị", cancelled),
	})
}

func (h *BiometricHandler) BackupDevice(w http.ResponseWriter, r *http.Request) {
	srcDeviceID := chi.URLParam(r, "id")

	var body struct {
		TargetDeviceIDs []string `json:"target_device_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.TargetDeviceIDs) == 0 {
		http.Error(w, "invalid request body, missing target_device_ids", http.StatusBadRequest)
		return
	}

	err := h.biometricService.BackupDeviceTemplates(r.Context(), srcDeviceID, body.TargetDeviceIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Device backup completed successfully"})
}
