package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	retentionDays = 7
	logFileName   = "agent-hive.log"
)

// RotatingWriter is an io.Writer that rotates log files daily and cleans up old files.
type RotatingWriter struct {
	mu      sync.Mutex
	dir     string
	file    *os.File
	curDate string
	nowFunc func() time.Time // for testing
}

// NewRotatingWriter creates a new rotating log writer in the given directory.
func NewRotatingWriter(dir string) (*RotatingWriter, error) {
	w := &RotatingWriter{
		dir:     dir,
		nowFunc: time.Now,
	}
	if err := w.openOrRotate(); err != nil {
		return nil, err
	}
	return w, nil
}

// Write implements io.Writer. It checks for date change before each write.
func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := w.nowFunc().Format("2006-01-02")
	if today != w.curDate {
		if err := w.rotate(today); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

// Close closes the current log file.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *RotatingWriter) openOrRotate() error {
	today := w.nowFunc().Format("2006-01-02")
	logPath := filepath.Join(w.dir, logFileName)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	w.file = f
	w.curDate = today
	return nil
}

func (w *RotatingWriter) rotate(newDate string) error {
	if w.file != nil {
		w.file.Close()
	}

	logPath := filepath.Join(w.dir, logFileName)
	archiveName := fmt.Sprintf("agent-hive.%s.log", w.curDate)
	archivePath := filepath.Join(w.dir, archiveName)

	if _, err := os.Stat(logPath); err == nil {
		os.Rename(logPath, archivePath)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("create new log file: %w", err)
	}
	w.file = f
	w.curDate = newDate

	w.cleanOldLogs()
	return nil
}

func (w *RotatingWriter) cleanOldLogs() {
	cutoff := w.nowFunc().AddDate(0, 0, -retentionDays)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Match pattern: agent-hive.YYYY-MM-DD.log
		if len(name) != len("agent-hive.2006-01-02.log") {
			continue
		}
		if name[:12] != "agent-hive." || name[22:] != ".log" {
			continue
		}
		dateStr := name[12:22]
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			os.Remove(filepath.Join(w.dir, name))
		}
	}
}

// Setup configures the global logger to write to both stdout and a rotating file.
// dir is the directory for the log file (typically the binary's directory).
func Setup(dir string) (*RotatingWriter, error) {
	rw, err := NewRotatingWriter(dir)
	if err != nil {
		return nil, err
	}
	multi := io.MultiWriter(os.Stdout, rw)
	log.SetOutput(multi)
	log.SetFlags(log.Ldate | log.Ltime)
	return rw, nil
}
