package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Env         string `mapstructure:"env"`
	HTTPPort    int    `mapstructure:"http_port"`
	PostgresDSN string `mapstructure:"postgres_dsn"`
	CronSpec    string `mapstructure:"attendance_sync_cron"` // ví dụ "*/15 * * * *"
	JWTSecret   string `mapstructure:"jwt_secret"`
	LDAPEnabled bool   `mapstructure:"ldap_enabled"`
	LDAPURL     string `mapstructure:"ldap_url"`
	LDAPDomain  string `mapstructure:"ldap_domain"`
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
	v.AutomaticEnv()

	v.SetDefault("env", "development")
	v.SetDefault("http_port", 8080)
	v.SetDefault("attendance_sync_cron", "*/15 * * * *")

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
