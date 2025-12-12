package config

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
	loadDotEnv()

	return Config{
		AppEnv:   getenv("APP_ENV", "development"),
		HTTPPort: getenv("HTTP_PORT", "8888"),
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

var envOnce sync.Once

// loadDotEnv loads values from a local .env file (if present) without
// overriding already-set environment variables.
func loadDotEnv() {
	envOnce.Do(func() {
		for _, path := range candidateEnvPaths() {
			if err := applyEnvFile(path); err == nil {
				log.Printf("config: loaded env file %s", path)
				return
			} else if !os.IsNotExist(err) {
				log.Printf("config: could not load env file %s: %v", path, err)
			}
		}
	})
}

func candidateEnvPaths() []string {
	paths := []string{".env"}

	_, filename, _, ok := runtime.Caller(0)
	if ok {
		// config.go lives in internal/config; climb to module root.
		moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
		if moduleRoot != "." {
			paths = append(paths, filepath.Join(moduleRoot, ".env"))
		}
	}

	return paths
}

func applyEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		if key == "" || os.Getenv(key) != "" {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
