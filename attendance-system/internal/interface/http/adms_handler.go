package http

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"attendance-system/internal/usecase"
)

type ADMSHandler struct {
	admsService *usecase.ADMSService
}

func NewADMSHandler(admsService *usecase.ADMSService) *ADMSHandler {
	return &ADMSHandler{admsService: admsService}
}

func extractADMSSerial(r *http.Request) string {
	query := r.URL.Query()
	for key, values := range query {
		if len(values) == 0 {
			continue
		}
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		if lowerKey == "sn" || lowerKey == "serial" || lowerKey == "devicesn" || lowerKey == "device_sn" || lowerKey == "deviceid" {
			if value := strings.TrimSpace(values[0]); value != "" {
				return value
			}
		}
	}
	return ""
}

// Cdata xử lý cả GET và POST tới /iclock/cdata
func (h *ADMSHandler) Cdata(w http.ResponseWriter, r *http.Request) {
	sn := extractADMSSerial(r)
	if sn == "" {
		http.Error(w, "missing SN parameter", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		fmt.Printf("[ADMS] REGISTER SN=%s query=%s\n", sn, r.URL.RawQuery)
		resp, err := h.admsService.RegisterOrGetConfig(r.Context(), sn)
		if err != nil {
			fmt.Printf("[ADMS] RegisterOrGetConfig error: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Printf("[ADMS] Config response: %q\n", resp)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
		return
	}

	if r.Method == http.MethodPost {
		table := r.URL.Query().Get("table")
		if table == "" {
			table = "ATTLOG"
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		// Log chi tiết dữ liệu nhận được từ máy — quan trọng để debug
		fmt.Printf("[ADMS] PUSH SN=%s table=%s bodyLen=%d\n>>> BODY BEGIN <<<\n%s\n>>> BODY END <<<\n",
			sn, table, len(bodyBytes), string(bodyBytes))

		resp, err := h.admsService.ProcessIncomingData(r.Context(), sn, table, string(bodyBytes))
		if err != nil {
			fmt.Printf("[ADMS] ERROR processing SN=%s table=%s: %v\n", sn, table, err)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK\n"))
			return
		}

		fmt.Printf("[ADMS] OK SN=%s table=%s result=%q\n", sn, table, resp)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// GetRequest xử lý GET tới /iclock/getrequest (thiết bị kéo lệnh)
func (h *ADMSHandler) GetRequest(w http.ResponseWriter, r *http.Request) {
	sn := extractADMSSerial(r)
	if sn == "" {
		http.Error(w, "missing SN parameter", http.StatusBadRequest)
		return
	}

	resp, err := h.admsService.GetPendingCommand(r.Context(), sn)
	if err != nil {
		fmt.Printf("[ADMS] GETREQUEST SN=%s error=%v\n", sn, err)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
		return
	}

	fmt.Printf("[ADMS] GETREQUEST SN=%s response=%q\n", sn, resp)
	if resp != "OK\n" {
		fmt.Printf("[ADMS] COMMAND sent to SN=%s: %q\n", sn, resp)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(resp))
}

// DeviceCmd xử lý POST tới /iclock/devicecmd (xác nhận lệnh hoàn tất)
func (h *ADMSHandler) DeviceCmd(w http.ResponseWriter, r *http.Request) {
	sn := extractADMSSerial(r)
	if sn == "" {
		http.Error(w, "missing SN parameter", http.StatusBadRequest)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	fmt.Printf("[ADMS] DEVICECMD SN=%s body=%q\n", sn, string(bodyBytes))

	resp, err := h.admsService.ConfirmCommand(r.Context(), sn, string(bodyBytes))
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(resp))
}
