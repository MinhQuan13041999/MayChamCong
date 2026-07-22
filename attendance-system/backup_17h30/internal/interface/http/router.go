package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"attendance-system/internal/config"
	"attendance-system/internal/interface/dto"
)

// NewRouter khởi tạo router chính, gắn middleware chung và các nhóm route theo domain.
// Route /api/v1/auth/login là public (không cần JWT).
// Tất cả các route còn lại trong /api/v1 yêu cầu JWT hợp lệ.
func NewRouter(
	deviceHandler *DeviceHandler,
	employeeHandler *EmployeeHandler,
	syncHandler *SyncHandler,
	authHandler *AuthHandler,
	reportHandler *ReportHandler,
	attendanceHandler *AttendanceHandler,
	admsHandler *ADMSHandler,
	biometricHandler *BiometricHandler,
	jwtSecret []byte,
	cfg *config.Config,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(RequestLogger)

	// ADMS Push protocol endpoints - public (không dùng JWT vì thiết bị không hỗ trợ)
	r.Route("/iclock", func(iclock chi.Router) {
		iclock.HandleFunc("/cdata", admsHandler.Cdata)
		iclock.Get("/getrequest", admsHandler.GetRequest)
		iclock.Post("/devicecmd", admsHandler.DeviceCmd)
	})

	r.Route("/api/v1", func(api chi.Router) {
		// Public: đăng nhập không cần token
		authHandler.Routes(api)

		// Protected: tất cả các route khác yêu cầu JWT hợp lệ
		api.Group(func(protected chi.Router) {
			protected.Use(JWTAuth(jwtSecret))
			deviceHandler.Routes(protected)
			employeeHandler.Routes(protected)
			syncHandler.Routes(protected)
			biometricHandler.Routes(protected)
			protected.Get("/stream", NewSSEHandler(jwtSecret))
			
			// System configurations
			protected.Get("/system/config", func(w http.ResponseWriter, r *http.Request) {
				dbHost, dbName := parseDSN(cfg.PostgresDSN)
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"ldap_enabled": cfg.LDAPEnabled,
					"ldap_url":     cfg.LDAPURL,
					"ldap_domain":  cfg.LDAPDomain,
					"cron_spec":    cfg.CronSpec,
					"http_port":    cfg.HTTPPort,
					"db_host":      dbHost,
					"db_name":      dbName,
				})
			})

			reportHandler.Routes(protected)
			attendanceHandler.Routes(protected)
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	webRoot := filepath.Join(".", "web")
	fileServer := http.FileServer(http.Dir(webRoot))
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") || strings.HasPrefix(r.URL.Path, "/css/") || strings.HasPrefix(r.URL.Path, "/js/") || strings.HasPrefix(r.URL.Path, "/styles.css") || strings.HasPrefix(r.URL.Path, "/app.js") {
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := os.Stat(filepath.Join(webRoot, strings.TrimPrefix(r.URL.Path, "/"))); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(webRoot, "index.html"))
	})

	return r
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, dto.ErrorResponse{Error: err.Error()})
}

func errNotImplemented(feature string) error {
	return fmt.Errorf("%s: not implemented yet", feature)
}

func parseDSN(dsn string) (string, string) {
	host := "localhost"
	dbname := "attendance"
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parts := strings.SplitN(dsn, "@", 2)
		if len(parts) > 1 {
			remain := parts[1]
			slashIdx := strings.Index(remain, "/")
			if slashIdx != -1 {
				host = remain[:slashIdx]
				dbAndParams := remain[slashIdx+1:]
				qIdx := strings.Index(dbAndParams, "?")
				if qIdx != -1 {
					dbname = dbAndParams[:qIdx]
				} else {
					dbname = dbAndParams
				}
			} else {
				host = remain
			}
		}
	} else {
		fields := strings.Fields(dsn)
		for _, f := range fields {
			kv := strings.SplitN(f, "=", 2)
			if len(kv) == 2 {
				if kv[0] == "host" {
					host = kv[1]
				} else if kv[0] == "dbname" {
					dbname = kv[1]
				}
			}
		}
	}
	return host, dbname
}
