package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server    ServerConfig    `json:"server"`
	Database  DatabaseConfig  `json:"database"`
	Cache     CacheConfig     `json:"cache"`
	Logging   LoggingConfig   `json:"logging"`
	CORS      CORSConfig      `json:"cors"`
	RateLimit RateLimitConfig `json:"rate_limit"`
}

type ServerConfig struct {
	Port         int `json:"port"`
	ReadTimeout  int `json:"read_timeout"`
	WriteTimeout int `json:"write_timeout"`
	IdleTimeout  int `json:"idle_timeout"`
}

type DatabaseConfig struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSLMode  string `json:"ssl_mode"`
	MaxConns int    `json:"max_conns"`
	MaxIdle  int    `json:"max_idle"`
}

type CacheConfig struct {
	Enabled  bool   `json:"enabled"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	TTL      int    `json:"ttl"`
}

type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

type CORSConfig struct {
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"`
}

type RateLimitConfig struct {
	Enabled       bool `json:"enabled"`
	MaxRequests   int  `json:"max_requests"`
	WindowMinutes int  `json:"window_minutes"`
}

func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Port:         getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:  getEnvInt("SERVER_READ_TIMEOUT", 30),
			WriteTimeout: getEnvInt("SERVER_WRITE_TIMEOUT", 30),
			IdleTimeout:  getEnvInt("SERVER_IDLE_TIMEOUT", 60),
		},
		Database: DatabaseConfig{
			Type:     getEnvString("DB_TYPE", "postgres"),
			Host:     getEnvString("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", getDefaultDBPort(getEnvString("DB_TYPE", "postgres"))),
			Database: getEnvString("DB_NAME", "goapitemplate"),
			Username: getEnvString("DB_USER", "postgres"),
			Password: getEnvString("DB_PASSWORD", ""),
			SSLMode:  getEnvString("DB_SSLMODE", "disable"),
			MaxConns: getEnvInt("DB_MAX_CONNS", 25),
			MaxIdle:  getEnvInt("DB_MAX_IDLE", 10),
		},
		Cache: CacheConfig{
			Enabled:  getEnvBool("CACHE_ENABLED", false),
			Type:     getEnvString("CACHE_TYPE", "redis"),
			Host:     getEnvString("CACHE_HOST", "localhost"),
			Port:     getEnvInt("CACHE_PORT", getDefaultCachePort(getEnvString("CACHE_TYPE", "redis"))),
			Password: getEnvString("CACHE_PASSWORD", ""),
			DB:       getEnvInt("CACHE_DB", 0),
			TTL:      getEnvInt("CACHE_TTL", 3600),
		},
		Logging: LoggingConfig{
			Level:  getEnvString("LOG_LEVEL", "info"),
			Format: getEnvString("LOG_FORMAT", "json"),
		},
		CORS: CORSConfig{
			AllowedOrigins:   strings.Split(getEnvString("CORS_ALLOWED_ORIGINS", "*"), ","),
			AllowedMethods:   strings.Split(getEnvString("CORS_ALLOWED_METHODS", "GET,POST,PUT,DELETE,OPTIONS"), ","),
			AllowedHeaders:   strings.Split(getEnvString("CORS_ALLOWED_HEADERS", "Content-Type,Authorization,X-Requested-With"), ","),
			AllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", true),
			MaxAge:           getEnvInt("CORS_MAX_AGE", 86400),
		},
		RateLimit: RateLimitConfig{
			Enabled:       getEnvBool("RATE_LIMIT_ENABLED", false),
			MaxRequests:   getEnvInt("RATE_LIMIT_MAX_REQUESTS", 100),
			WindowMinutes: getEnvInt("RATE_LIMIT_WINDOW_MINUTES", 1),
		},
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

func validateConfig(cfg *Config) error {
	supportedDBTypes := []string{"postgres", "mysql", "sqlite"}
	if !contains(supportedDBTypes, cfg.Database.Type) {
		return fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	if cfg.Cache.Enabled {
		supportedCacheTypes := []string{"redis", "memcache"}
		if !contains(supportedCacheTypes, cfg.Cache.Type) {
			return fmt.Errorf("unsupported cache type: %s", cfg.Cache.Type)
		}
	}

	supportedLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(supportedLogLevels, cfg.Logging.Level) {
		return fmt.Errorf("unsupported log level: %s", cfg.Logging.Level)
	}

	return nil
}

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDefaultDBPort(dbType string) int {
	switch dbType {
	case "postgres":
		return 5432
	case "mysql":
		return 3306
	case "sqlite":
		return 0
	default:
		return 5432
	}
}

func getDefaultCachePort(cacheType string) int {
	switch cacheType {
	case "redis":
		return 6379
	case "memcache":
		return 11211
	default:
		return 6379
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
