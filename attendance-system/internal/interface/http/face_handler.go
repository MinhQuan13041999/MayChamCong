package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"attendance-system/internal/usecase"
)

type FaceHandler struct {
	faceService *usecase.FaceService
}

func NewFaceHandler(faceService *usecase.FaceService) *FaceHandler {
	return &FaceHandler{faceService: faceService}
}

func (h *FaceHandler) Routes(r chi.Router) {
	r.Post("/employees/{id}/face", h.RegisterFace)
	r.Delete("/employees/{id}/face", h.DeleteFace)
	r.Get("/faces", h.ListAll)
	r.Post("/attendance/face-check", h.SubmitFaceAttendance)
}

func (h *FaceHandler) RegisterFace(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")

	var body struct {
		FaceDescriptor string `json:"face_descriptor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.FaceDescriptor == "" {
		http.Error(w, "invalid request body, missing face_descriptor", http.StatusBadRequest)
		return
	}

	err := h.faceService.RegisterFace(r.Context(), employeeID, body.FaceDescriptor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Face registered successfully"})
}

func (h *FaceHandler) DeleteFace(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")
	err := h.faceService.DeleteFace(r.Context(), employeeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Face deleted successfully"})
}

func (h *FaceHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	faces, err := h.faceService.GetAllFaces(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, faces)
}

func (h *FaceHandler) SubmitFaceAttendance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EmployeeID string `json:"employee_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EmployeeID == "" {
		http.Error(w, "invalid request body, missing employee_id", http.StatusBadRequest)
		return
	}

	log, err := h.faceService.SubmitFaceAttendance(r.Context(), body.EmployeeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Face attendance submitted successfully",
		"log":     log,
	})
}
