package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	AppEnv          string
	AppPort         string
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	DBSSLMode       string
	JWTSecret       string
	JWTTTL          time.Duration
	HMACSecretInst1 string
	HMACSecretInst2 string
	HMACSecretInst3 string
}

// Load reads environment variables (and optional .env files) and validates required settings.
func Load() (*Config, error) {
	// Try loading .env from current directory or parent root directory
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	ttlStr := getEnv("JWT_TTL", "2h")
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		ttl = 2 * time.Hour
	}

	cfg := &Config{
		AppEnv:          getEnv("APP_ENV", "development"),
		AppPort:         getEnv("APP_PORT", "8080"),
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnv("DB_PORT", "5432"),
		DBUser:          getEnv("DB_USER", "ukt_user"),
		DBPassword:      getEnv("DB_PASSWORD", "ukt_secret_password_2026"),
		DBName:          getEnv("DB_NAME", "ukt_payment_db"),
		DBSSLMode:       getEnv("DB_SSLMODE", "disable"),
		JWTSecret:       getEnv("JWT_SECRET", "change-this-jwt-secret-key-min-32-chars-2026"),
		JWTTTL:          ttl,
		HMACSecretInst1: getEnv("HMAC_SECRET_INST_1", ""),
		HMACSecretInst2: getEnv("HMAC_SECRET_INST_2", ""),
		HMACSecretInst3: getEnv("HMAC_SECRET_INST_3", ""),
	}

	if cfg.DBHost == "" || cfg.DBUser == "" || cfg.DBName == "" {
		return nil, fmt.Errorf("konfigurasi database (DB_HOST, DB_USER, DB_NAME) wajib diisi")
	}

	return cfg, nil
}

// DSN returns PostgreSQL connection string for pgxpool.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser,
		c.DBPassword,
		c.DBHost,
		c.DBPort,
		c.DBName,
		c.DBSSLMode,
	)
}

// AdminDSN returns PostgreSQL connection string connected to the default 'postgres' database (for initial DB creation).
func (c *Config) AdminDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/postgres?sslmode=%s",
		c.DBUser,
		c.DBPassword,
		c.DBHost,
		c.DBPort,
		c.DBSSLMode,
	)
}

// ResolveHMACSecret resolves the HMAC secret string by environment variable key name.
func (c *Config) ResolveHMACSecret(envKey string) string {
	switch envKey {
	case "HMAC_SECRET_INST_1":
		return c.HMACSecretInst1
	case "HMAC_SECRET_INST_2":
		return c.HMACSecretInst2
	case "HMAC_SECRET_INST_3":
		return c.HMACSecretInst3
	default:
		return os.Getenv(envKey)
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
