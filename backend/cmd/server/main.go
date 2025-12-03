package main

import (
	"log"

	"finance-backend/internal/config"
	"finance-backend/internal/db"
	"finance-backend/internal/model"
	"finance-backend/internal/router"
)

func main() {
	cfg := config.Load()

	database := db.MustConnect(cfg)

	if err := model.AutoMigrate(database); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}

	engine := router.New(cfg, database)
	if err := engine.Run(cfg.ServerAddr()); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
