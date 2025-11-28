package router

import (
	"strings"

	"finance-backend/internal/config"
	"finance-backend/internal/handler/health"

	"github.com/gin-gonic/gin"
)

func New(cfg config.Config) *gin.Engine {
	setGinMode(cfg.AppEnv)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	api := r.Group("/api")
	{
		api.GET("/health", health.Ping)
	}

	return r
}

func setGinMode(env string) {
	switch strings.ToLower(env) {
	case "prod", "production":
		gin.SetMode(gin.ReleaseMode)
	case "test", "testing":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}
}
