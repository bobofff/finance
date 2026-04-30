package main

import (
	"log"

	"finance-backend/internal/config"
	"finance-backend/internal/db"
	"finance-backend/internal/logging"
	"finance-backend/internal/model"
	"finance-backend/internal/router"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	logger, logWriter, err := logging.New(logging.Config{Dir: cfg.Log.Dir})
	if err != nil {
		log.Fatalf("init logger failed: %v", err)
	}
	log.SetOutput(logWriter)
	gin.DefaultWriter = logWriter
	gin.DefaultErrorWriter = logWriter
	logger.Info("logging initialized", "log_dir", cfg.Log.Dir)

	database := db.MustConnect(cfg)

	if cfg.SkipAutoMigrate {
		logger.Info("auto migrate skipped", "skip_auto_migrate", true)
	} else {
		if err := model.AutoMigrate(database); err != nil {
			log.Fatalf("auto migrate failed: %v", err)
		}
	}
	if err := model.SeedStrategyTemplates(database); err != nil {
		log.Printf("seed strategy templates failed: %v", err)
	}

	engine := router.New(cfg, database, logger)
	if err := engine.Run(cfg.ServerAddr()); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
