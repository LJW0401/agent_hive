package fileutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeJoin(t *testing.T) {
	base := t.TempDir()
	// Create subdirectory
	os.MkdirAll(filepath.Join(base, "sub"), 0755)
	os.WriteFile(filepath.Join(base, "sub", "file.go"), []byte("package main"), 0644)

	tests := []struct {
		name    string
		rel     string
		wantErr bool
	}{
		{"valid subpath", "sub/file.go", false},
		{"valid dot", ".", false},
		{"dotdot traversal", "../etc/passwd", true},
		{"deep dotdot", "sub/../../etc/passwd", true},
		{"absolute path", "/etc/passwd", true},
		{"current dir dotdot", "sub/../sub/file.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeJoin(base, tt.rel)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for rel=%q, got result=%q", tt.rel, result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for rel=%q: %v", tt.rel, err)
				}
			}
		})
	}
}

func TestSafeJoinSymlink(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0644)

	// Create a symlink inside base that points outside
	err := os.Symlink(outside, filepath.Join(base, "escape"))
	if err != nil {
		t.Skip("symlinks not supported")
	}

	_, err = SafeJoin(base, "escape/secret.txt")
	if err == nil {
		t.Error("expected error for symlink traversal outside base")
	}
}

func TestIsBinary(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		content  []byte
		wantBin  bool
	}{
		{"text file", []byte("hello world\nline 2\n"), false},
		{"binary with NUL", []byte("hello\x00world"), true},
		{"ELF header", []byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00"), true},
		{"empty file", []byte{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name)
			os.WriteFile(path, tt.content, 0644)
			got, err := IsBinary(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.wantBin {
				t.Errorf("IsBinary() = %v, want %v", got, tt.wantBin)
			}
		})
	}
}

func TestReadTailLines(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name      string
		lines     int
		maxLines  int
		wantTrunc bool
		wantCount int
	}{
		{"under limit", 5, 10, false, 5},
		{"at limit", 10, 10, false, 10},
		{"over limit", 20, 10, true, 10},
		{"single line", 1, 10, false, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name+".txt")
			var sb strings.Builder
			for i := 0; i < tt.lines; i++ {
				if i > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString("line content")
			}
			os.WriteFile(path, []byte(sb.String()), 0644)

			content, truncated, err := ReadTailLines(path, tt.maxLines)
			if err != nil {
				t.Fatal(err)
			}
			if truncated != tt.wantTrunc {
				t.Errorf("truncated = %v, want %v", truncated, tt.wantTrunc)
			}
			gotCount := len(strings.Split(content, "\n"))
			if gotCount != tt.wantCount {
				t.Errorf("line count = %d, want %d", gotCount, tt.wantCount)
			}
		})
	}
}

func TestFileType(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"main.go", "text"},
		{"app.tsx", "text"},
		{"README.md", "markdown"},
		{"doc.pdf", "pdf"},
		{"photo.png", "image"},
		{"icon.svg", "image"},
		{"archive.zip", "binary"},
		{"binary.exe", "binary"},
		{"Makefile", "text"},
		{"Dockerfile", "text"},
		{".gitignore", "text"},
		{"data.json", "text"},
		{"style.css", "text"},
		{"config.yaml", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileType(tt.name)
			if got != tt.want {
				t.Errorf("FileType(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestLanguageFromExt(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"main.go", "go"},
		{"app.tsx", "tsx"},
		{"script.py", "python"},
		{"Makefile", "makefile"},
		{"unknown.xyz", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LanguageFromExt(tt.name)
			if got != tt.want {
				t.Errorf("LanguageFromExt(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
