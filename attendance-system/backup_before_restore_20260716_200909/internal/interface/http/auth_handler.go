package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-ldap/ldap/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"attendance-system/internal/domain/port"
)

// AuthHandler xử lý đăng nhập và phát hành JWT token.
type AuthHandler struct {
	userRepo    port.UserRepository
	jwtSecret   []byte
	ldapEnabled bool
	ldapURL     string
	ldapDomain  string
}

// NewAuthHandler khởi tạo AuthHandler với các cấu hình.
func NewAuthHandler(userRepo port.UserRepository, jwtSecret string, ldapEnabled bool, ldapURL string, ldapDomain string) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		jwtSecret:   []byte(jwtSecret),
		ldapEnabled: ldapEnabled,
		ldapURL:     ldapURL,
		ldapDomain:  ldapDomain,
	}
}

func (h *AuthHandler) Routes(r chi.Router) {
	r.Post("/auth/login", h.Login)
}

// loginRequest payload cho endpoint đăng nhập.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse trả về JWT access token sau khi đăng nhập thành công.
type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"` // Unix timestamp
}

// Login xác thực username/password (LDAP hoặc local bcrypt) và phát hành JWT access token (24h).
//
//	POST /api/v1/auth/login
//	{"username":"admin","password":"admin123"}
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("username and password are required"))
		return
	}

	var ldapAuthenticated bool
	if h.ldapEnabled {
		if err := h.authenticateLDAP(req.Username, req.Password); err == nil {
			ldapAuthenticated = true
		}
	}

	// Tìm user trong database để lấy thông tin phân quyền
	user, err := h.userRepo.GetByUsername(r.Context(), req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("authentication error"))
		return
	}

	if ldapAuthenticated {
		if user == nil {
			// Tài khoản tồn tại trong AD/LDAP nhưng chưa được phân quyền trong hệ thống chấm công
			writeError(w, http.StatusUnauthorized, fmt.Errorf("account authenticated via LDAP but not authorized in attendance system"))
			return
		}
	} else {
		// Fallback xác thực password bằng bcrypt (timing-safe)
		if user == nil {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid username or password"))
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid username or password"))
			return
		}
	}

	// Phát hành JWT, hết hạn sau 24 giờ
	expiresAt := time.Now().Add(24 * time.Hour)
	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"role_id":  user.RoleID,
		"exp":      expiresAt.Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(h.jwtSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("could not generate token"))
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		Token:     signed,
		ExpiresAt: expiresAt.Unix(),
	})
}

// authenticateLDAP thực hiện Bind thử vào AD/LDAP Server.
func (h *AuthHandler) authenticateLDAP(username, password string) error {
	bindUsername := fmt.Sprintf("%s@%s", username, h.ldapDomain)
	if strings.Contains(username, "@") {
		bindUsername = username
	}

	l, err := ldap.DialURL(h.ldapURL)
	if err != nil {
		return fmt.Errorf("ldap dial: %w", err)
	}
	defer l.Close()

	// Thực hiện Bind (Xác thực thông tin tài khoản mật khẩu)
	err = l.Bind(bindUsername, password)
	if err != nil {
		return fmt.Errorf("ldap bind failed: %w", err)
	}
	return nil
}
