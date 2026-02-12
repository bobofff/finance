package auth

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultTTLHours    = 24
	rememberTTLHours   = 24 * 30
	authHeaderPrefix   = "Bearer "
	claimsIssuer       = "finance-backend"
	contextUsernameKey = "auth_username"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Remember bool   `json:"remember"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type meResponse struct {
	Username string `json:"username"`
}

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/login", login)
	rg.GET("/me", me)
}

func login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	configUsername := strings.TrimSpace(os.Getenv("AUTH_USERNAME"))
	configHash := strings.TrimSpace(os.Getenv("AUTH_PASSWORD_HASH"))
	secret := strings.TrimSpace(os.Getenv("AUTH_JWT_SECRET"))

	if configUsername == "" || configHash == "" || secret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth is not configured"})
		return
	}

	if username != configUsername {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(configHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	ttl := time.Duration(defaultTTLHours) * time.Hour
	if req.Remember {
		ttl = time.Duration(rememberTTLHours) * time.Hour
	}

	expiresAt := time.Now().Add(ttl)
	claims := jwt.RegisteredClaims{
		Issuer:    claimsIssuer,
		Subject:   username,
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
		return
	}

	c.JSON(http.StatusOK, loginResponse{
		Token:     signed,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}

func me(c *gin.Context) {
	username, _ := c.Get(contextUsernameKey)
	name, _ := username.(string)
	if name == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, meResponse{Username: name})
}

func Middleware() gin.HandlerFunc {
	skipPaths := map[string]struct{}{
		"/api/health":     {},
		"/api/auth/login": {},
	}

	secret := strings.TrimSpace(os.Getenv("AUTH_JWT_SECRET"))
	return func(c *gin.Context) {
		if _, ok := skipPaths[c.FullPath()]; ok {
			c.Next()
			return
		}
		if c.FullPath() == "" {
			c.Next()
			return
		}
		if secret == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "auth is not configured"})
			c.Abort()
			return
		}

		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, authHeaderPrefix) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			c.Abort()
			return
		}

		raw := strings.TrimSpace(strings.TrimPrefix(header, authHeaderPrefix))
		if raw == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			c.Abort()
			return
		}

		parsed, err := jwt.ParseWithClaims(raw, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(secret), nil
		})
		if err != nil || !parsed.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
		if !ok || claims.Subject == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set(contextUsernameKey, claims.Subject)
		c.Next()
	}
}
