package router

import (
	"log/slog"
	"strings"

	"finance-backend/internal/config"
	"finance-backend/internal/handler/account"
	"finance-backend/internal/handler/accountsnapshot"
	"finance-backend/internal/handler/ai"
	"finance-backend/internal/handler/auth"
	"finance-backend/internal/handler/categories"
	"finance-backend/internal/handler/goal"
	"finance-backend/internal/handler/health"
	"finance-backend/internal/handler/investment"
	"finance-backend/internal/handler/report"
	"finance-backend/internal/handler/transaction"
	"finance-backend/internal/handler/transfer"
	"finance-backend/internal/logging"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, logger *slog.Logger) *gin.Engine {
	setGinMode(cfg.AppEnv)

	r := gin.New()
	r.Use(gin.Logger(), logging.Recovery(logger), logging.RequestLogger(logger))

	api := r.Group("/api")
	{
		api.GET("/health", health.Ping)
		auth.RegisterRoutes(api.Group("/auth"))
		api.Use(auth.Middleware())
		account.RegisterRoutes(api.Group("/accounts"), db)
		accountsnapshot.RegisterRoutes(api.Group("/account-snapshots"), db)
		ai.RegisterRoutes(api.Group("/ai"), db, ai.Config{
			OpenAIAPIKey:          cfg.AI.OpenAIAPIKey,
			OpenAIBaseURL:         cfg.AI.OpenAIBaseURL,
			OpenAIModel:           cfg.AI.OpenAIModel,
			RequestTimeoutSeconds: cfg.AI.RequestTimeoutSeconds,
			UseEnvProxy:           cfg.AI.UseEnvProxy,
			Timezone:              cfg.DB.Timezone,
		})
		categories.RegisterRoutes(api.Group("/categories"), db)
		goal.RegisterRoutes(api.Group("/goals"), db)
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
