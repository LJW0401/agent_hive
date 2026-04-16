package fileutil

import (
	"bufio"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// textExts maps file extensions to whether they are text (true) or binary (false).
var textExts = map[string]bool{
	".go": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".py": true, ".rb": true, ".rs": true, ".java": true, ".c": true,
	".cpp": true, ".h": true, ".hpp": true, ".cs": true, ".swift": true,
	".kt": true, ".scala": true, ".php": true, ".lua": true, ".sh": true,
	".bash": true, ".zsh": true, ".fish": true, ".ps1": true, ".bat": true,
	".cmd": true, ".sql": true, ".html": true, ".htm": true, ".css": true,
	".scss": true, ".sass": true, ".less": true, ".xml": true, ".json": true,
	".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".cfg": true,
	".conf": true, ".env": true, ".txt": true, ".log": true, ".csv": true,
	".tsv": true, ".graphql": true, ".gql": true, ".proto": true,
	".dockerfile": true, ".gitignore": true, ".editorconfig": true,
	".makefile": true, ".cmake": true, ".gradle": true, ".tf": true,
	".hcl": true, ".r": true, ".m": true, ".mm": true, ".vue": true,
	".svelte": true, ".astro": true, ".zig": true, ".nim": true,
	".dart": true, ".ex": true, ".exs": true, ".erl": true, ".hs": true,
	".ml": true, ".clj": true, ".lisp": true, ".el": true, ".v": true,
	".wasm": false, // binary
}

// langMap maps file extensions to Shiki language identifiers.
var langMap = map[string]string{
	".go": "go", ".js": "javascript", ".jsx": "jsx", ".ts": "typescript",
	".tsx": "tsx", ".py": "python", ".rb": "ruby", ".rs": "rust",
	".java": "java", ".c": "c", ".cpp": "cpp", ".h": "c", ".hpp": "cpp",
	".cs": "csharp", ".swift": "swift", ".kt": "kotlin", ".scala": "scala",
	".php": "php", ".lua": "lua", ".sh": "bash", ".bash": "bash",
	".zsh": "bash", ".fish": "fish", ".ps1": "powershell",
	".sql": "sql", ".html": "html", ".htm": "html", ".css": "css",
	".scss": "scss", ".sass": "sass", ".less": "less",
	".xml": "xml", ".json": "json", ".yaml": "yaml", ".yml": "yaml",
	".toml": "toml", ".ini": "ini", ".graphql": "graphql",
	".proto": "protobuf", ".dockerfile": "dockerfile",
	".vue": "vue", ".svelte": "svelte", ".md": "markdown",
	".r": "r", ".dart": "dart", ".ex": "elixir", ".exs": "elixir",
	".erl": "erlang", ".hs": "haskell", ".ml": "ocaml",
	".clj": "clojure", ".zig": "zig", ".nim": "nim",
	".tf": "hcl", ".hcl": "hcl",
}

// SafeJoin joins base and rel, ensuring the result stays within base.
// It prevents path traversal via "..", absolute paths, and symlinks.
func SafeJoin(base, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed: %s", rel)
	}

	joined := filepath.Join(base, rel)
	joined = filepath.Clean(joined)

	// Check the cleaned path is still under base
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) && absJoined != absBase {
		return "", fmt.Errorf("path traversal detected: %s", rel)
	}

	// Evaluate symlinks and re-check
	realBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		return "", err
	}
	realJoined, err := filepath.EvalSymlinks(absJoined)
	if err != nil {
		// Target might not exist yet for EvalSymlinks; check parent
		if os.IsNotExist(err) {
			parentDir := filepath.Dir(absJoined)
			realParent, err2 := filepath.EvalSymlinks(parentDir)
			if err2 != nil {
				return "", fmt.Errorf("path not accessible: %s", rel)
			}
			if !strings.HasPrefix(realParent, realBase+string(filepath.Separator)) && realParent != realBase {
				return "", fmt.Errorf("symlink traversal detected: %s", rel)
			}
			return absJoined, nil
		}
		return "", err
	}
	if !strings.HasPrefix(realJoined, realBase+string(filepath.Separator)) && realJoined != realBase {
		return "", fmt.Errorf("symlink traversal detected: %s", rel)
	}

	return absJoined, nil
}

// IsBinary reads the first 512 bytes of a file and checks for NUL bytes.
func IsBinary(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}
	if n == 0 {
		return false, nil // empty file is not binary
	}

	for _, b := range buf[:n] {
		if b == 0 {
			return true, nil
		}
	}
	return false, nil
}

// maxReadBytes is the maximum bytes to read from large files.
// Files larger than this are read from the tail only.
const maxReadBytes int64 = 10 * 1024 * 1024

// ReadTailLines reads the last maxLines lines from a file.
// Returns the content, whether the file was truncated, and any error.
func ReadTailLines(path string, maxLines int) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", false, err
	}

	// For large files, seek to tail to avoid reading entire file into memory
	truncatedBySize := false
	if info.Size() > maxReadBytes {
		if _, err := f.Seek(-maxReadBytes, io.SeekEnd); err == nil {
			// Skip partial first line after seek
			r := bufio.NewReader(f)
			r.ReadLine() //nolint: discard partial line
			truncatedBySize = true
			// Re-wrap f with the buffered reader for scanning
			return scanTailLines(r, maxLines, truncatedBySize)
		}
	}

	return scanTailLines(bufio.NewReader(f), maxLines, truncatedBySize)
}

func scanTailLines(r io.Reader, maxLines int, alreadyTruncated bool) (string, bool, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", false, err
	}

	if len(lines) <= maxLines {
		return strings.Join(lines, "\n"), alreadyTruncated, nil
	}

	tail := lines[len(lines)-maxLines:]
	return strings.Join(tail, "\n"), true, nil
}

// FileType returns the content type category based on file extension.
// Returns one of: "text", "markdown", "image", "pdf", "binary"
func FileType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))

	switch ext {
	case ".md", ".markdown", ".mdx":
		return "markdown"
	case ".pdf":
		return "pdf"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico":
		return "image"
	}

	if isText, ok := textExts[ext]; ok {
		if isText {
			return "text"
		}
		return "binary"
	}

	// Check by MIME type
	mimeType := mime.TypeByExtension(ext)
	if strings.HasPrefix(mimeType, "text/") {
		return "text"
	}

	// Files without extension: check common names
	baseName := strings.ToLower(filepath.Base(name))
	switch baseName {
	case "makefile", "dockerfile", "vagrantfile", "gemfile", "rakefile",
		"procfile", "brewfile", "justfile", "taskfile",
		".gitignore", ".gitattributes", ".dockerignore", ".editorconfig",
		".eslintrc", ".prettierrc", ".babelrc",
		".bashrc", ".bash_profile", ".bash_logout", ".bash_aliases",
		".zshrc", ".zshenv", ".zprofile", ".zlogin", ".zlogout",
		".profile", ".inputrc", ".vimrc", ".nanorc", ".tmux.conf",
		".wgetrc", ".curlrc", ".npmrc", ".yarnrc",
		"license", "licence", "authors", "contributors", "changelog",
		"todo", "readme", "news", "history", "install":
		return "text"
	}

	return "binary"
}

// LanguageFromExt returns the Shiki-compatible language identifier for a file extension.
func LanguageFromExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))

	if lang, ok := langMap[ext]; ok {
		return lang
	}

	baseName := strings.ToLower(filepath.Base(name))
	switch baseName {
	case "makefile":
		return "makefile"
	case "dockerfile":
		return "dockerfile"
	}

	return "text"
}

// MimeTypeFromExt returns the MIME type for image files.
func MimeTypeFromExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}
