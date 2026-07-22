package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// contextKey kiểu riêng để tránh collision với các key khác trong context.
type contextKey string

const ctxKeyUserID contextKey = "user_id"
const ctxKeyUsername contextKey = "username"
const ctxKeyRoleID contextKey = "role_id"

// RequestLogger ghi log structured cho mỗi request (request_id, path, status, duration).
// Trong thực tế nên inject *zap.Logger qua constructor thay vì dùng global logger;
// để đơn giản hoá skeleton này dùng zap.L() (global logger, cần zap.ReplaceGlobals ở main).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		zap.L().Info("http_request",
			zap.String("request_id", middleware.GetReqID(r.Context())),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", ww.Status()),
			zap.Duration("duration", time.Since(start)),
		)
	})
}

// JWTAuth là middleware bảo vệ các route yêu cầu đăng nhập.
// Đọc JWT từ header: Authorization: Bearer <token>
// Nếu hợp lệ → inject user_id, username, role_id vào context và cho phép tiếp tục.
// Nếu không hợp lệ hoặc thiếu → trả 401 Unauthorized.
func JWTAuth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			var tokenStr string
			if authHeader == "" {
				// Fallback to query parameter for SSE/EventSource
				tokenStr = r.URL.Query().Get("token")
				if tokenStr == "" {
					writeError(w, http.StatusUnauthorized, fmt.Errorf("missing Authorization header or token query parameter"))
					return
				}
			} else {
				if !strings.HasPrefix(authHeader, "Bearer ") {
					writeError(w, http.StatusUnauthorized, fmt.Errorf("authorization scheme must be Bearer"))
					return
				}
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return secret, nil
			})
			if err != nil || !token.Valid {
				writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid or expired token"))
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid token claims"))
				return
			}

			// Inject thông tin user vào context để handler downstream có thể dùng
			ctx := context.WithValue(r.Context(), ctxKeyUserID, claims["sub"])
			ctx = context.WithValue(ctx, ctxKeyUsername, claims["username"])
			ctx = context.WithValue(ctx, ctxKeyRoleID, claims["role_id"])

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole checks if the role in token is allowed
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleVal := r.Context().Value(ctxKeyRoleID)
			if roleVal == nil {
				writeError(w, http.StatusForbidden, fmt.Errorf("forbidden: role not identified"))
				return
			}
			role, ok := roleVal.(string)
			if !ok {
				writeError(w, http.StatusForbidden, fmt.Errorf("forbidden: role is not valid"))
				return
			}

			// admin always has full rights
			if strings.ToLower(role) == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			found := false
			for _, ar := range allowedRoles {
				if strings.ToLower(role) == strings.ToLower(ar) {
					found = true
					break
				}
			}

			if !found {
				writeError(w, http.StatusForbidden, fmt.Errorf("forbidden: access denied for role %s", role))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
