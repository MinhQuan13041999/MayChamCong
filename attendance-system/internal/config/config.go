package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Env           string             `mapstructure:"env"`
	HTTPPort      int                `mapstructure:"http_port"`
	PostgresDSN   string             `mapstructure:"postgres_dsn"`
	CronSpec      string             `mapstructure:"attendance_sync_cron"` // ví dụ "*/15 * * * *"
	JWTSecret     string             `mapstructure:"jwt_secret"`
	LDAPEnabled   bool               `mapstructure:"ldap_enabled"`
	LDAPURL       string             `mapstructure:"ldap_url"`
	LDAPDomain    string             `mapstructure:"ldap_domain"`
	SDK           SDKConfig          `mapstructure:"sdk"`
	Attendance    AttendanceConfig   `mapstructure:"attendance"`
	Notifications NotificationConfig `mapstructure:"notifications"`
}

// AttendanceConfig điều khiển cửa sổ nhận diện log và chính sách OT.
// Các giá trị mặc định giữ nguyên cửa sổ ca ±1 giờ đang sử dụng.
type AttendanceConfig struct {
	ShiftWindowBeforeMinutes  int  `mapstructure:"shift_window_before_minutes"`
	ShiftWindowAfterMinutes   int  `mapstructure:"shift_window_after_minutes"`
	OvertimeRequiresApproval  bool `mapstructure:"overtime_requires_approval"`
	OvertimeRequiresDeviceLog bool `mapstructure:"overtime_requires_device_log"`
	OvertimeLogGraceMinutes   int  `mapstructure:"overtime_log_grace_minutes"`
}

// SDKConfig controls the background attendance pull used when a device is
// configured without ADMS. The worker is deliberately independent from the
// existing cron job so changing this interval does not alter manual payroll
// calculation or the ADMS push endpoints.
type SDKConfig struct {
	AttendancePollIntervalSeconds int `mapstructure:"attendance_poll_interval_seconds"`
	ConnectTimeoutSeconds         int `mapstructure:"connect_timeout_seconds"`
	MaxRetries                    int `mapstructure:"max_retries"`
}

// NotificationConfig cấu hình gửi thông báo chấm công qua email và Zalo OA.
// ZaloUserID của từng nhân viên được lưu trong hồ sơ nhân viên.
type NotificationConfig struct {
	Enabled              bool   `mapstructure:"enabled"`
	EmailEnabled         bool   `mapstructure:"email_enabled"`
	SMTPHost             string `mapstructure:"smtp_host"`
	SMTPPort             int    `mapstructure:"smtp_port"`
	SMTPUsername         string `mapstructure:"smtp_username"`
	SMTPPassword         string `mapstructure:"smtp_password"`
	SMTPFrom             string `mapstructure:"smtp_from"`
	ZaloEnabled          bool   `mapstructure:"zalo_enabled"`
	ZaloAPIURL           string `mapstructure:"zalo_api_url"`
	ZaloAccessToken      string `mapstructure:"zalo_access_token"`
	InstantMaxAgeMinutes int    `mapstructure:"instant_max_age_minutes"`
	CheckoutGraceMinutes int    `mapstructure:"checkout_grace_minutes"`
}

// Load đọc cấu hình từ file config.yaml (nếu có) và biến môi trường (ưu tiên cao hơn).
// Biến môi trường dùng prefix ATTENDANCE_, ví dụ ATTENDANCE_POSTGRES_DSN.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(path)
	v.AddConfigPath(".")

	v.SetEnvPrefix("ATTENDANCE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("env", "development")
	v.SetDefault("http_port", 8080)
	v.SetDefault("attendance_sync_cron", "*/10 * * * *")
	v.SetDefault("sdk.attendance_poll_interval_seconds", 600)
	v.SetDefault("sdk.connect_timeout_seconds", 8)
	v.SetDefault("sdk.max_retries", 2)
	v.SetDefault("attendance.shift_window_before_minutes", 60)
	v.SetDefault("attendance.shift_window_after_minutes", 60)
	v.SetDefault("attendance.overtime_requires_approval", true)
	v.SetDefault("attendance.overtime_requires_device_log", true)
	v.SetDefault("attendance.overtime_log_grace_minutes", 15)
	v.SetDefault("notifications.enabled", true)
	v.SetDefault("notifications.email_enabled", false)
	v.SetDefault("notifications.smtp_host", "")
	v.SetDefault("notifications.smtp_port", 587)
	v.SetDefault("notifications.smtp_username", "")
	v.SetDefault("notifications.smtp_password", "")
	v.SetDefault("notifications.smtp_from", "")
	v.SetDefault("notifications.zalo_enabled", false)
	v.SetDefault("notifications.zalo_api_url", "https://openapi.zalo.me/v3.0/oa/message/cs")
	v.SetDefault("notifications.zalo_access_token", "")
	v.SetDefault("notifications.instant_max_age_minutes", 10)
	v.SetDefault("notifications.checkout_grace_minutes", 30)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
		// không bắt buộc phải có file config, có thể dùng toàn bộ env var
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if cfg.PostgresDSN == "" {
		return nil, fmt.Errorf("postgres_dsn (ATTENDANCE_POSTGRES_DSN) is required")
	}

	return &cfg, nil
}
