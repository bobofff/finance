package router

import (
	"strings"

	"finance-backend/internal/config"
	"finance-backend/internal/handler/account"
	"finance-backend/internal/handler/accountsnapshot"
	"finance-backend/internal/handler/categories"
	"finance-backend/internal/handler/health"
	"finance-backend/internal/handler/investment"
	"finance-backend/internal/handler/report"
	"finance-backend/internal/handler/transfer"
	"finance-backend/internal/handler/transaction"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB) *gin.Engine {
	setGinMode(cfg.AppEnv)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	api := r.Group("/api")
	{
		api.GET("/health", health.Ping)
		account.RegisterRoutes(api.Group("/accounts"), db)
		accountsnapshot.RegisterRoutes(api.Group("/account-snapshots"), db)
		categories.RegisterRoutes(api.Group("/categories"), db)
		investment.RegisterRoutes(api.Group("/investments"), db)
		transfer.RegisterRoutes(api.Group("/transfers"), db)
		transaction.RegisterRoutes(api.Group("/transactions"), db)
		report.RegisterRoutes(api.Group("/reports"), db)
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
