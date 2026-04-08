package internal

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// readFileSafe reads a file and returns its content, or nil on error.
// Files larger than 2MB are skipped to avoid excessive memory use.
func readFileSafe(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 2*1024*1024 {
		return nil, nil
	}
	return os.ReadFile(path)
}

// relativePath returns path relative to root, falling back to path itself.
func relativePath(path, root string) string {
	if root == "" {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

// absolutePath reconstructs an absolute path from a display path + root.
func absolutePath(display, root string) string {
	if strings.HasPrefix(display, "/") {
		return display
	}
	return root + "/" + display
}

// normalizeHTTPPath strips common API prefixes and replaces path parameter
// patterns ({id}, :id, <id>) with a canonical {param} placeholder.
func normalizeHTTPPath(raw string) string {
	raw = stripQueryString(raw)
	raw = stripCommonAPIPrefix(raw)
	if raw == "" {
		raw = "/"
	}
	return canonicalizePathParams(raw)
}

func stripQueryString(raw string) string {
	if idx := strings.IndexByte(raw, '?'); idx >= 0 {
		return raw[:idx]
	}
	return raw
}

func stripCommonAPIPrefix(raw string) string {
	for _, pfx := range []string{"/api/v1", "/api/v2", "/api/v3", "/api"} {
		if strings.HasPrefix(raw, pfx+"/") || raw == pfx {
			return raw[len(pfx):]
		}
	}
	return raw
}

func canonicalizePathParams(raw string) string {
	segments := strings.Split(raw, "/")
	for i, seg := range segments {
		if isPathParamSegment(seg) {
			segments[i] = "{param}"
		}
	}
	return strings.Join(segments, "/")
}

func isPathParamSegment(seg string) bool {
	return len(seg) > 1 && (seg[0] == ':' ||
		(seg[0] == '{' && seg[len(seg)-1] == '}') ||
		(seg[0] == '<' && seg[len(seg)-1] == '>') ||
		isNumericSegment(seg) ||
		isUUIDLikeSegment(seg))
}

func isNumericSegment(seg string) bool {
	for _, r := range seg {
		if r < '0' || r > '9' {
			return false
		}
	}
	return seg != ""
}

func isUUIDLikeSegment(seg string) bool {
	if len(seg) < 8 || !strings.Contains(seg, "-") {
		return false
	}
	for _, r := range seg {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') || r == '-' {
			continue
		}
		return false
	}
	return true
}

// splitLines splits src into lines using a buffered scanner.
func splitLines(src []byte) []string {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(src))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// leadingSpaces returns the leading whitespace prefix of s.
func leadingSpaces(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}

// buildLineIndex builds a slice of byte offsets where each line starts,
// enabling O(log n) line-number lookup for a byte offset.
func buildLineIndex(src []byte) []int {
	idx := []int{0}
	for i, b := range src {
		if b == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

// lineForOffset returns the 1-based line number for a byte offset.
func lineForOffset(lineIndex []int, offset int) int {
	lo, hi := 0, len(lineIndex)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if lineIndex[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo + 1
}

// fileExt returns the lowercase extension of a path (e.g. ".ts").
func fileExt(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != '/' && path[i] != '\\'; i-- {
		if path[i] == '.' {
			return strings.ToLower(path[i:])
		}
	}
	return ""
}
