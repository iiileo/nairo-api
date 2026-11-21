package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	MySQL     MySQLConfig     `mapstructure:"mysql"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Web       WebConfig       `mapstructure:"web"`
	Admin     AdminConfig     `mapstructure:"admin"`
	Console   ConsoleConfig   `mapstructure:"console"`
	SMTP      SMTPConfig      `mapstructure:"smtp"`
	Snowflake SnowflakeConfig `mapstructure:"snowflake"`
	Pay       PayConfig       `mapstructure:"pay"`
	Security  SecurityConfig  `mapstructure:"security"`
	Geetest   GeetestConfig   `mapstructure:"geetest"`
}

type GeetestConfig struct {
	CaptchaID  string `mapstructure:"captcha_id"`
	CaptchaKey string `mapstructure:"captcha_key"`
}

type ServerConfig struct {
	Address        string `mapstructure:"address"`
	Mode           string `mapstructure:"mode"`
	AllowedOrigins string `mapstructure:"allowed_origins"` // 逗号分隔；空=放宽（local）
}

type SecurityConfig struct {
	// MasterKey 用于加密落库的 App/API Secret；prod 必填
	MasterKey string `mapstructure:"master_key"`
}

type MySQLConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	Params   string `mapstructure:"params"`
}

func (m MySQLConfig) DSN() string {
	params := m.Params
	if strings.TrimSpace(params) == "" {
		// DATETIME 按 Asia/Shanghai 解释；勿用 Local（Docker 默认 UTC 会偏 8 小时）
		params = "parseTime=true&loc=Asia%2FShanghai&charset=utf8mb4"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		m.User, m.Password, m.Host, m.Port, m.DBName, params)
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

type WebConfig struct {
	// Dev=true：静态资源读磁盘，不依赖 embed（本地开发）。
	Dev           bool   `mapstructure:"dev"`
	IndexDir      string `mapstructure:"index_dir"`
	ConsoleDir    string `mapstructure:"console_dir"`
	AdminDir      string `mapstructure:"admin_dir"`
	AdminBasePath string `mapstructure:"admin_base_path"`
}

type AdminConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	JWTSecret         string `mapstructure:"jwt_secret"`
	AccessTTLSeconds  int64  `mapstructure:"access_ttl_seconds"`
	BootstrapUsername string `mapstructure:"bootstrap_username"`
	BootstrapPassword string `mapstructure:"bootstrap_password"`
}

type ConsoleConfig struct {
	JWTSecret          string `mapstructure:"jwt_secret"`
	AccessTTLSeconds   int64  `mapstructure:"access_ttl_seconds"`
	RefreshTTLSeconds  int64  `mapstructure:"refresh_ttl_seconds"`
	DefaultFeeRateBps  int    `mapstructure:"default_fee_rate_bps"`
	CodeTTLSeconds     int64  `mapstructure:"code_ttl_seconds"`
	// RegisterEnabled：是否开放商户自助注册（可用后台热更新覆盖）
	RegisterEnabled bool `mapstructure:"register_enabled"`
	// BootstrapDemo：是否自动种子 user01（prod 默认关闭）
	BootstrapDemo bool `mapstructure:"bootstrap_demo"`
}

type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	// DevPrint=true：未配置 SMTP 时把验证码打日志（仅 local）
	DevPrint bool `mapstructure:"dev_print"`
}

type SnowflakeConfig struct {
	NodeID int64 `mapstructure:"node_id"`
}

type PayConfig struct {
	LockTTLSeconds int64  `mapstructure:"lock_ttl_seconds"`
	PublicBaseURL  string `mapstructure:"public_base_url"` // 拼回调 URL，如 http://127.0.0.1:8088
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load()

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./internal/config")
	v.AddConfigPath("/etc/nairo")

	// 无前缀：SERVER_ADDRESS / MYSQL_HOST / WEB_DEV ...
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	_ = v.ReadInConfig() // 允许仅用环境变量 / .env

	bindEnvs(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if cfg.Web.AdminBasePath == "" {
		cfg.Web.AdminBasePath = "/admin_nairo"
	}
	if !strings.HasPrefix(cfg.Web.AdminBasePath, "/") {
		cfg.Web.AdminBasePath = "/" + cfg.Web.AdminBasePath
	}
	if isProdMode(cfg.Server.Mode) {
		if _, set := os.LookupEnv("CONSOLE_BOOTSTRAP_DEMO"); !set {
			cfg.Console.BootstrapDemo = false
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func isProdMode(mode string) bool {
	m := strings.ToLower(strings.TrimSpace(mode))
	return m == "prod" || m == "production"
}

// Validate 生产模式强制安全基线。
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("nil config")
	}
	if !isProdMode(c.Server.Mode) {
		return nil
	}
	weak := []struct {
		name string
		val  string
		bad  []string
	}{
		{"ADMIN_JWT_SECRET", c.Admin.JWTSecret, []string{"", "nairo-dev-secret-change-me"}},
		{"CONSOLE_JWT_SECRET", c.Console.JWTSecret, []string{"", "nairo-console-dev-secret-change-me"}},
		{"ADMIN_BOOTSTRAP_PASSWORD", c.Admin.BootstrapPassword, []string{"", "admin123"}},
		{"SECURITY_MASTER_KEY", c.Security.MasterKey, []string{""}},
	}
	for _, w := range weak {
		v := strings.TrimSpace(w.val)
		for _, b := range w.bad {
			if v == b {
				return fmt.Errorf("prod config: set a strong %s", w.name)
			}
		}
		if len(v) < 16 {
			return fmt.Errorf("prod config: %s too short (min 16)", w.name)
		}
	}
	if c.SMTP.DevPrint {
		return fmt.Errorf("prod config: set SMTP_DEV_PRINT=false and configure SMTP")
	}
	if strings.Contains(c.Pay.PublicBaseURL, "127.0.0.1") || strings.Contains(c.Pay.PublicBaseURL, "localhost") {
		return fmt.Errorf("prod config: PAY_PUBLIC_BASE_URL must be public HTTPS URL")
	}
	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.address", ":8088")
	v.SetDefault("server.mode", "local")
	v.SetDefault("server.allowed_origins", "")
	v.SetDefault("security.master_key", "")
	v.SetDefault("mysql.host", "127.0.0.1")
	v.SetDefault("mysql.port", 3306)
	v.SetDefault("mysql.user", "root")
	v.SetDefault("mysql.password", "root")
	v.SetDefault("mysql.dbname", "nairo_app")
	v.SetDefault("logging.level", "debug")
	v.SetDefault("web.dev", true)
	v.SetDefault("web.index_dir", "index_web")
	v.SetDefault("web.console_dir", "console_web/dist")
	v.SetDefault("web.admin_dir", "admin_web/dist")
	v.SetDefault("web.admin_base_path", "/admin_nairo")
	v.SetDefault("admin.enabled", true)
	v.SetDefault("admin.jwt_secret", "nairo-dev-secret-change-me")
	v.SetDefault("admin.access_ttl_seconds", 86400)
	v.SetDefault("admin.bootstrap_username", "admin")
	v.SetDefault("admin.bootstrap_password", "admin123")
	v.SetDefault("console.jwt_secret", "nairo-console-dev-secret-change-me")
	v.SetDefault("console.access_ttl_seconds", 900)
	v.SetDefault("console.refresh_ttl_seconds", 604800)
	v.SetDefault("console.default_fee_rate_bps", 100)
	v.SetDefault("console.code_ttl_seconds", 600)
	v.SetDefault("console.register_enabled", true)
	v.SetDefault("console.bootstrap_demo", true)
	v.SetDefault("smtp.host", "")
	v.SetDefault("smtp.port", 587)
	v.SetDefault("smtp.user", "")
	v.SetDefault("smtp.password", "")
	v.SetDefault("smtp.from", "noreply@nairo.local")
	v.SetDefault("smtp.dev_print", true)
	v.SetDefault("snowflake.node_id", 1)
	v.SetDefault("pay.lock_ttl_seconds", 300)
	v.SetDefault("pay.public_base_url", "http://127.0.0.1:8088")
	v.SetDefault("geetest.captcha_id", "")
	v.SetDefault("geetest.captcha_key", "")
}

func bindEnvs(v *viper.Viper) {
	_ = v.BindEnv("server.address", "SERVER_ADDRESS")
	_ = v.BindEnv("server.mode", "SERVER_MODE")
	_ = v.BindEnv("server.allowed_origins", "SERVER_ALLOWED_ORIGINS")
	_ = v.BindEnv("security.master_key", "SECURITY_MASTER_KEY")
	_ = v.BindEnv("mysql.host", "MYSQL_HOST")
	_ = v.BindEnv("mysql.port", "MYSQL_PORT")
	_ = v.BindEnv("mysql.user", "MYSQL_USER")
	_ = v.BindEnv("mysql.password", "MYSQL_PASSWORD")
	_ = v.BindEnv("mysql.dbname", "MYSQL_DBNAME")
	_ = v.BindEnv("logging.level", "LOGGING_LEVEL")
	_ = v.BindEnv("web.dev", "WEB_DEV")
	_ = v.BindEnv("web.index_dir", "WEB_INDEX_DIR")
	_ = v.BindEnv("web.console_dir", "WEB_CONSOLE_DIR")
	_ = v.BindEnv("web.admin_dir", "WEB_ADMIN_DIR")
	_ = v.BindEnv("web.admin_base_path", "WEB_ADMIN_BASE_PATH")
	_ = v.BindEnv("snowflake.node_id", "SNOWFLAKE_NODE_ID")
	_ = v.BindEnv("admin.enabled", "ADMIN_ENABLED")
	_ = v.BindEnv("admin.jwt_secret", "ADMIN_JWT_SECRET")
	_ = v.BindEnv("admin.access_ttl_seconds", "ADMIN_ACCESS_TTL_SECONDS")
	_ = v.BindEnv("admin.bootstrap_username", "ADMIN_BOOTSTRAP_USERNAME")
	_ = v.BindEnv("admin.bootstrap_password", "ADMIN_BOOTSTRAP_PASSWORD")
	_ = v.BindEnv("console.jwt_secret", "CONSOLE_JWT_SECRET")
	_ = v.BindEnv("console.access_ttl_seconds", "CONSOLE_ACCESS_TTL_SECONDS")
	_ = v.BindEnv("console.refresh_ttl_seconds", "CONSOLE_REFRESH_TTL_SECONDS")
	_ = v.BindEnv("console.default_fee_rate_bps", "CONSOLE_DEFAULT_FEE_RATE_BPS")
	_ = v.BindEnv("console.code_ttl_seconds", "CONSOLE_CODE_TTL_SECONDS")
	_ = v.BindEnv("console.register_enabled", "CONSOLE_REGISTER_ENABLED")
	_ = v.BindEnv("console.bootstrap_demo", "CONSOLE_BOOTSTRAP_DEMO")
	_ = v.BindEnv("smtp.host", "SMTP_HOST")
	_ = v.BindEnv("smtp.port", "SMTP_PORT")
	_ = v.BindEnv("smtp.user", "SMTP_USER")
	_ = v.BindEnv("smtp.password", "SMTP_PASSWORD")
	_ = v.BindEnv("smtp.from", "SMTP_FROM")
	_ = v.BindEnv("smtp.dev_print", "SMTP_DEV_PRINT")
	_ = v.BindEnv("pay.lock_ttl_seconds", "PAY_LOCK_TTL_SECONDS")
	_ = v.BindEnv("pay.public_base_url", "PAY_PUBLIC_BASE_URL")
	_ = v.BindEnv("geetest.captcha_id", "GEETEST_CAPTCHA_ID")
	_ = v.BindEnv("geetest.captcha_key", "GEETEST_CAPTCHA_KEY")
}
