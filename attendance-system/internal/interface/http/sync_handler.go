package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/interface/dto"
	"attendance-system/internal/usecase"
)

type SyncHandler struct {
	syncService    *usecase.SyncService
	attendanceRepo port.AttendanceLogRepository
	historyRepo    port.SyncHistoryRepository
}

func NewSyncHandler(syncService *usecase.SyncService, attendanceRepo port.AttendanceLogRepository, historyRepo port.SyncHistoryRepository) *SyncHandler {
	return &SyncHandler{syncService: syncService, attendanceRepo: attendanceRepo, historyRepo: historyRepo}
}

func (h *SyncHandler) Routes(r chi.Router) {
	r.Post("/devices/{id}/sync-attendance", h.SyncAttendance)
	r.Get("/attendance-logs", h.QueryLogs)
	r.Post("/attendance-logs", h.CreateLogs)
	r.Get("/sync-history", h.ListHistory)
	r.Post("/sync-history/{id}/retry", h.Retry)
}

func (h *SyncHandler) SyncAttendance(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")

	var req dto.SyncAttendanceRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // body optional, mặc định lấy 24h gần nhất

	to := time.Now()
	from := to.Add(-24 * time.Hour)
	if req.From != "" {
		if t, err := time.Parse(time.RFC3339, req.From); err == nil {
			from = t
		}
	}
	if req.To != "" {
		if t, err := time.Parse(time.RFC3339, req.To); err == nil {
			to = t
		}
	}

	// Đọc ATTLOG qua COM SDK có thể cần vài giây để Connect_Net và nạp bộ
	// đệm log. Cho phép tối đa 30 giây để tránh trình duyệt hủy một phiên SDK
	// hợp lệ khi thiết bị đồng thời đang bật ADMS.
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	hist, err := h.syncService.SyncAttendance(ctx, deviceID, from, to, entity.SyncTriggerManual)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromSyncHistory(hist))
}

func (h *SyncHandler) QueryLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from := parseTimeOrDefault(q.Get("from"), time.Now().AddDate(0, 0, -7))
	to := parseTimeOrDefault(q.Get("to"), time.Now().Add(24*time.Hour))

	logs, err := h.attendanceRepo.Query(r.Context(), from, to, q.Get("employee_code"), q.Get("device_id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (h *SyncHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := h.historyRepo.List(r.Context(), q.Get("device_id"), q.Get("status"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *SyncHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	hist, err := h.syncService.RetrySync(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromSyncHistory(hist))
}

func (h *SyncHandler) CreateLogs(w http.ResponseWriter, r *http.Request) {
	var logs []entity.AttendanceLog
	if err := json.NewDecoder(r.Body).Decode(&logs); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	for i := range logs {
		if logs[i].SyncedAt.IsZero() {
			logs[i].SyncedAt = time.Now()
		}
		if logs[i].CheckType == "" {
			logs[i].CheckType = entity.CheckTypeIn
		}
		if logs[i].VerifyMode == "" {
			logs[i].VerifyMode = entity.VerifyModeFingerprint
		}
	}

	inserted, err := h.attendanceRepo.BulkInsert(r.Context(), logs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"inserted": inserted,
		"message":  "Logs inserted successfully",
	})
}

func parseTimeOrDefault(v string, def time.Time) time.Time {
	if v == "" {
		return def
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t
	}
	return def
}
