// Package config - отвечает за загрузку и хранение конфигурации приложения из YAML файла и переменных окружения
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config - структура для хранения всей конфигурации приложения
type Config struct {
	Telegram Telegram `yaml:"telegram"`
	Salute   Salute   `yaml:"salute"`
	GigaChat GigaChat `yaml:"gigachat"`
	Database Database `yaml:"database"`
	Vault    Vault    `yaml:"vault"`
}

// Telegram - конфигурация для Telegram бота
type Telegram struct {
	Token string `yaml:"token"`
}

// Salute - конфигурация для SaluteSpeech API
type Salute struct {
	ClientID     string        `yaml:"client_id"`
	ClientSecret string        `yaml:"client_secret"`
	Scope        string        `yaml:"scope"`
	AuthURL      string        `yaml:"auth_url"`
	GRPCEndpoint string        `yaml:"grpc_endpoint"`
	GRPCInsecure bool          `yaml:"grpc_insecure"`
	CertPath     string        `yaml:"cert_path"`
	PollInterval time.Duration `yaml:"poll_interval"`
	ModelURI     string        `yaml:"model_uri"`
}

// GigaChat - конфигурация для GigaChat API
type GigaChat struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	Scope        string `yaml:"scope"`
	AuthURL      string `yaml:"auth_url"`
	APIURL       string `yaml:"api_url"`
	Model        string `yaml:"model"`
	CertPath     string `yaml:"cert_path"`
}

// Database - конфигурация для подключения к базе данных
type Database struct {
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
	ConnMaxLife  string `yaml:"conn_max_life"`
	// Optional connection parts when using Vault to supply credentials
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Name    string `yaml:"name"`
	SSLMode string `yaml:"ssl_mode"`
}

// Vault - конфигурация для HashiCorp Vault
type Vault struct {
	Addr                 string        `yaml:"addr"`
	Token                string        `yaml:"token"`
	AppRoleRoleID        string        `yaml:"approle_role_id"`
	AppRoleSecretID      string        `yaml:"approle_secret_id"`
	AppRoleSecretWrapped bool          `yaml:"approle_secret_wrapped"`
	DatabaseRole         string        `yaml:"database_role"`
	WrapTTL              string        `yaml:"wrap_ttl"`
	RotateInterval       time.Duration `yaml:"rotate_interval"`
	// AppRoleRoleName - имя роли в Vault (используется при создании secret-id)
	AppRoleRoleName string `yaml:"approle_role_name"`
	// WrappedTokenFile - путь для записи wrapping token (wrap_info.token) при ротации
	WrappedTokenFile string `yaml:"wrapped_token_file"`
}

// Load загружает конфигурацию из YAML файла и переопределяет её значениями из переменных окружения
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.overrideFromEnv()
	return &cfg, nil
}

// overrideFromEnv переопределяет поля конфигурации значениями из переменных окружения, если они установлены
func (c *Config) overrideFromEnv() {
	if v := os.Getenv("TELEGRAM_TOKEN"); v != "" {
		c.Telegram.Token = v
	}
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		c.Database.DSN = v
	}
	if v := os.Getenv("SALUTE_CLIENT_ID"); v != "" {
		c.Salute.ClientID = v
	}
	if v := os.Getenv("SALUTE_CLIENT_SECRET"); v != "" {
		c.Salute.ClientSecret = v
	}
	if v := os.Getenv("GIGACHAT_CLIENT_ID"); v != "" {
		c.GigaChat.ClientID = v
	}
	if v := os.Getenv("GIGACHAT_CLIENT_SECRET"); v != "" {
		c.GigaChat.ClientSecret = v
	}
	if v := os.Getenv("VAULT_ADDR"); v != "" {
		c.Vault.Addr = v
	}
	if v := os.Getenv("VAULT_TOKEN"); v != "" {
		c.Vault.Token = v
	}
	if v := os.Getenv("VAULT_APPROLE_ROLE_ID"); v != "" {
		c.Vault.AppRoleRoleID = v
	}
	if v := os.Getenv("VAULT_APPROLE_ROLE_NAME"); v != "" {
		c.Vault.AppRoleRoleName = v
	}
	if v := os.Getenv("VAULT_WRAPPED_TOKEN_FILE"); v != "" {
		c.Vault.WrappedTokenFile = v
	}
	if v := os.Getenv("VAULT_APPROLE_SECRET_ID"); v != "" {
		c.Vault.AppRoleSecretID = v
	}
	if v := os.Getenv("VAULT_DATABASE_ROLE"); v != "" {
		c.Vault.DatabaseRole = v
	}
}
