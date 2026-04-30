package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type dailyFileWriter struct {
	mu          sync.Mutex
	dir         string
	prefix      string
	stdout      io.Writer
	currentDate string
	file        *os.File
}

func newDailyFileWriter(dir string, prefix string) (*dailyFileWriter, error) {
	writer := &dailyFileWriter{
		dir:    normalizeLogDir(dir),
		prefix: strings.TrimSpace(prefix),
		stdout: os.Stdout,
	}

	if writer.prefix == "" {
		writer.prefix = "app"
	}

	if err := writer.rotateIfNeeded(time.Now()); err != nil {
		return nil, err
	}

	return writer, nil
}

func (w *dailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeeded(time.Now()); err != nil {
		return 0, err
	}

	return io.MultiWriter(w.stdout, w.file).Write(p)
}

func (w *dailyFileWriter) rotateIfNeeded(now time.Time) error {
	date := now.Format("2006-01-02")
	if w.file != nil && w.currentDate == date {
		return nil
	}

	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}

	path := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, date))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	if w.file != nil {
		_ = w.file.Close()
	}

	w.file = file
	w.currentDate = date
	return nil
}

func normalizeLogDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = "logs"
	}

	cleaned := filepath.Clean(dir)
	if absPath, err := filepath.Abs(cleaned); err == nil {
		return absPath
	}

	return cleaned
}
