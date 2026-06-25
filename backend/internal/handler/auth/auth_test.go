package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddlewareSkipsWorldCupWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	api := router.Group("/api")
	api.Use(Middleware())
	api.GET("/world-cup", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	api.GET("/world-cup/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for _, path := range []string{"/api/world-cup?refresh=1", "/api/world-cup/?refresh=1"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusNoContent {
			t.Fatalf("expected world cup route to skip auth for %s, got status %d with body %q", path, response.Code, response.Body.String())
		}
	}
}
