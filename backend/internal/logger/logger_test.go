package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRotatingWriter_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	rw, err := NewRotatingWriter(dir)
	if err != nil {
		t.Fatalf("NewRotatingWriter: %v", err)
	}
	defer rw.Close()

	msg := []byte("hello log\n")
	n, err := rw.Write(msg)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("expected %d bytes written, got %d", len(msg), n)
	}

	rw.Close()
	data, err := os.ReadFile(filepath.Join(dir, logFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != string(msg) {
		t.Fatalf("expected %q, got %q", msg, data)
	}
}

func TestRotatingWriter_RotatesOnDateChange(t *testing.T) {
	dir := t.TempDir()
	day1 := time.Date(2026, 4, 15, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, 4, 16, 0, 1, 0, 0, time.UTC)

	rw, err := NewRotatingWriter(dir)
	if err != nil {
		t.Fatalf("NewRotatingWriter: %v", err)
	}
	defer rw.Close()

	// Override nowFunc for day1
	rw.mu.Lock()
	rw.nowFunc = func() time.Time { return day1 }
	rw.curDate = day1.Format("2006-01-02")
	rw.mu.Unlock()

	rw.Write([]byte("day1 log\n"))

	// Switch to day2
	rw.mu.Lock()
	rw.nowFunc = func() time.Time { return day2 }
	rw.mu.Unlock()

	rw.Write([]byte("day2 log\n"))
	rw.Close()

	// Check archived file exists
	archivePath := filepath.Join(dir, "agent-hive.2026-04-15.log")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatal("expected archived log file for 2026-04-15")
	}

	// Check current file has day2 content
	data, _ := os.ReadFile(filepath.Join(dir, logFileName))
	if string(data) != "day2 log\n" {
		t.Fatalf("expected day2 content, got %q", data)
	}
}

func TestRotatingWriter_CleansOldLogs(t *testing.T) {
	dir := t.TempDir()

	// Create old log files
	for i := 0; i < 10; i++ {
		d := time.Date(2026, 4, 1+i, 0, 0, 0, 0, time.UTC)
		name := "agent-hive." + d.Format("2006-01-02") + ".log"
		os.WriteFile(filepath.Join(dir, name), []byte("old"), 0644)
	}

	rw, err := NewRotatingWriter(dir)
	if err != nil {
		t.Fatalf("NewRotatingWriter: %v", err)
	}

	// Set now to April 16 and trigger rotation
	rw.mu.Lock()
	rw.nowFunc = func() time.Time { return time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC) }
	rw.curDate = "2026-04-15" // Force date mismatch
	rw.mu.Unlock()

	rw.Write([]byte("trigger rotation\n"))
	rw.Close()

	// Files older than 7 days from April 16 (cutoff: April 9) should be deleted
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		if len(name) == len("agent-hive.2006-01-02.log") && name[:12] == "agent-hive." && name[22:] == ".log" {
			dateStr := name[12:22]
			d, _ := time.Parse("2006-01-02", dateStr)
			cutoff := time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC)
			if d.Before(cutoff) {
				t.Errorf("old log file %s should have been cleaned up", name)
			}
		}
	}
}
