package db

import (
	"fmt"
	"log"
	"strings"
	"time"

	"finance-backend/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens a GORM connection based on Config.DB.
func Connect(cfg config.Config) (*gorm.DB, error) {
	dialector, err := buildDialector(cfg.DB)
	if err != nil {
		return nil, err
	}

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Sensible defaults; adjust later if needed.
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}

func buildDialector(cfg config.DBConfig) (gorm.Dialector, error) {
	switch strings.ToLower(cfg.Driver) {
	case "postgres", "postgresql":
		dsn := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
			cfg.Host,
			cfg.Port,
			cfg.User,
			cfg.Password,
			cfg.Name,
			cfg.SSLMode,
			cfg.Timezone,
		)
		return postgres.Open(dsn), nil
	case "mysql":
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.User,
			cfg.Password,
			cfg.Host,
			cfg.Port,
			cfg.Name,
		)
		return mysql.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported DB driver: %s", cfg.Driver)
	}
}

func MustConnect(cfg config.Config) *gorm.DB {
	db, err := Connect(cfg)
	if err != nil {
		log.Fatalf("database connect failed: %v", err)
	}
	return db
}
