package config

import (
	"os"
)

type Config struct {
	AppEnv   string
	HTTPPort string
	DB       DBConfig
}

type DBConfig struct {
	Driver   string
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	Timezone string
}

func Load() Config {
	return Config{
		AppEnv:   getenv("APP_ENV", "development"),
		HTTPPort: getenv("HTTP_PORT", "8080"),
		DB: DBConfig{
			Driver:   getenv("DB_DRIVER", "postgres"), // postgres | mysql
			Host:     getenv("DB_HOST", "127.0.0.1"),
			Port:     getenv("DB_PORT", "5432"),
			User:     getenv("DB_USER", "finance"),
			Password: getenv("DB_PASSWORD", "finance"),
			Name:     getenv("DB_NAME", "finance"),
			SSLMode:  getenv("DB_SSLMODE", "disable"),
			Timezone: getenv("DB_TIMEZONE", "Asia/Shanghai"),
		},
	}
}

func (c Config) ServerAddr() string {
	return ":" + c.HTTPPort
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
