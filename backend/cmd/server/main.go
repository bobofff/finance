package main

import (
	"log"

	"finance-backend/internal/config"
	"finance-backend/internal/db"
	"finance-backend/internal/router"
)

func main() {
	cfg := config.Load()

	if _, err := db.Connect(cfg); err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	engine := router.New(cfg)
	if err := engine.Run(cfg.ServerAddr()); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
