package logging

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	serviceName     = "finance-backend"
	requestIDHeader = "X-Request-Id"
	loggerKey       = "request_logger"
	requestIDKey    = "request_id"
)

type Config struct {
	Dir string
}

func New(cfg Config) (*slog.Logger, io.Writer, error) {
	writer, err := newDailyFileWriter(cfg.Dir, serviceName)
	if err != nil {
		return nil, nil, err
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler).With("service", serviceName), writer, nil
}

type bodyCaptureWriter struct {
	gin.ResponseWriter
	body  *bytes.Buffer
	limit int
}

func (w *bodyCaptureWriter) Write(data []byte) (int, error) {
	w.capture(data)
	return w.ResponseWriter.Write(data)
}

func (w *bodyCaptureWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

func (w *bodyCaptureWriter) capture(data []byte) {
	if w == nil || w.body == nil || len(data) == 0 || w.limit <= 0 {
		return
	}

	remaining := w.limit - w.body.Len()
	if remaining <= 0 {
		return
	}

	if len(data) > remaining {
		data = data[:remaining]
	}

	_, _ = w.body.Write(data)
}

func RequestLogger(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := requestIDFromHeader(c.GetHeader(requestIDHeader))
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Writer.Header().Set(requestIDHeader, requestID)

		capture := &bodyCaptureWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			limit:          4096,
		}
		c.Writer = capture

		start := time.Now()
		logger := loggerForContext(c, base).With(
			slog.String(requestIDKey, requestID),
			slog.String("method", c.Request.Method),
			slog.String("path", requestPath(c)),
			slog.String("client_ip", c.ClientIP()),
		)
		c.Set(loggerKey, logger)
		c.Set(requestIDKey, requestID)

		c.Next()

		status := c.Writer.Status()
		if status < http.StatusBadRequest && len(c.Errors) == 0 {
			return
		}

		attrs := []any{
			slog.Int("status", status),
			slog.Duration("latency", time.Since(start)),
		}
		if errText := strings.TrimSpace(c.Errors.String()); errText != "" {
			attrs = append(attrs, slog.String("errors", errText))
		}
		if responseError := extractResponseError(capture.body.Bytes()); responseError != "" {
			attrs = append(attrs, slog.String("response_error", responseError))
		}

		switch {
		case len(c.Errors) > 0 || status >= http.StatusInternalServerError:
			logger.Error("request failed", attrs...)
		default:
			logger.Warn("request returned client error", attrs...)
		}
	}
}

func Recovery(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger := loggerForContext(c, base).With(
					slog.String(requestIDKey, requestIDFromContext(c)),
					slog.String("method", c.Request.Method),
					slog.String("path", requestPath(c)),
					slog.String("client_ip", c.ClientIP()),
				)
				logger.Error("panic recovered",
					slog.String("panic", fmt.Sprint(recovered)),
					slog.String("stack", string(debug.Stack())),
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()

		c.Next()
	}
}

func AbortWithError(c *gin.Context, status int, publicMessage string, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	c.AbortWithStatusJSON(status, gin.H{"error": publicMessage})
}

func FromContext(c *gin.Context) *slog.Logger {
	return loggerForContext(c, nil)
}

func loggerForContext(c *gin.Context, base *slog.Logger) *slog.Logger {
	if value, ok := c.Get(loggerKey); ok {
		if logger, ok := value.(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	if base != nil {
		return base
	}
	return slog.Default()
}

func requestPath(c *gin.Context) string {
	if path := strings.TrimSpace(c.FullPath()); path != "" {
		return path
	}
	return c.Request.URL.Path
}

func requestIDFromContext(c *gin.Context) string {
	if value, ok := c.Get(requestIDKey); ok {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}

func requestIDFromHeader(header string) string {
	header = strings.TrimSpace(header)
	if header != "" {
		return header
	}
	return ""
}

func newRequestID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func extractResponseError(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		if message, ok := payload["error"].(string); ok {
			message = strings.TrimSpace(message)
			if message != "" {
				return message
			}
		}
	}

	return truncateString(trimmed, 1024)
}

func truncateString(value string, limit int) string {
	if limit <= 0 {
		return value
	}

	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}

	return string(runes[:limit]) + "...(truncated)"
}
