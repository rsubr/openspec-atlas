package internal

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// maxSafeReadSize caps how large a file we will load into memory for the
// regex-based analysers. Files bigger than this are skipped rather than
// allocating hundreds of megabytes for a single source file.
const maxSafeReadSize = 2 * 1024 * 1024

// errFileSkipped is returned by readFileSafe for files that are too large to
// load. Using a sentinel keeps callers from having to distinguish between the
// (nil, nil) "skipped" case and a real error.
var errFileSkipped = errors.New("file skipped")

// readFileSafe reads a file and returns its content. Files larger than
// maxSafeReadSize are skipped with errFileSkipped so callers can branch on
// errors.Is instead of checking for a nil slice.
func readFileSafe(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxSafeReadSize {
		return nil, errFileSkipped
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
// When display is already absolute it is returned unchanged; otherwise it is
// joined with root using the platform's native separator.
func absolutePath(display, root string) string {
	if filepath.IsAbs(display) {
		return display
	}
	if root == "" {
		return display
	}
	return filepath.Join(root, display)
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

// stripQueryString removes any trailing query string so route matching compares
// only the path portion.
func stripQueryString(raw string) string {
	if idx := strings.IndexByte(raw, '?'); idx >= 0 {
		return raw[:idx]
	}
	return raw
}

// stripCommonAPIPrefix removes broad API prefixes that commonly differ between
// frontend client URLs and backend route declarations.
func stripCommonAPIPrefix(raw string) string {
	for _, pfx := range []string{"/api/v1", "/api/v2", "/api/v3", "/api"} {
		if strings.HasPrefix(raw, pfx+"/") || raw == pfx {
			return raw[len(pfx):]
		}
	}
	return raw
}

// canonicalizePathParams rewrites dynamic-looking path segments into a single
// placeholder so equivalent routes normalize to the same shape.
func canonicalizePathParams(raw string) string {
	segments := strings.Split(raw, "/")
	for i, seg := range segments {
		if isPathParamSegment(seg) {
			segments[i] = "{param}"
		}
	}
	return strings.Join(segments, "/")
}

// isPathParamSegment recognizes route segments that look like variable IDs
// instead of literal path text.
func isPathParamSegment(seg string) bool {
	return len(seg) > 1 && (seg[0] == ':' ||
		(seg[0] == '{' && seg[len(seg)-1] == '}') ||
		(seg[0] == '<' && seg[len(seg)-1] == '>') ||
		isNumericSegment(seg) ||
		isUUIDLikeSegment(seg))
}

// isNumericSegment reports whether the segment is made entirely of digits.
func isNumericSegment(seg string) bool {
	for _, r := range seg {
		if r < '0' || r > '9' {
			return false
		}
	}
	return seg != ""
}

// isUUIDLikeSegment treats hyphenated hexadecimal identifiers as path
// parameters so UUID URLs match named parameter routes.
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

// splitLines splits src into lines without the 64KB limit imposed by the
// default bufio.Scanner buffer. Large SQL dumps and minified JS routinely
// contain lines bigger than that, so we slice the source directly instead of
// running it through a scanner.
func splitLines(src []byte) []string {
	if len(src) == 0 {
		return nil
	}
	n := bytes.Count(src, []byte{'\n'})
	lines := make([]string, 0, n+1)
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] != '\n' {
			continue
		}
		end := i
		if end > start && src[end-1] == '\r' {
			end--
		}
		lines = append(lines, string(src[start:end]))
		start = i + 1
	}
	if start < len(src) {
		end := len(src)
		if end > start && src[end-1] == '\r' {
			end--
		}
		lines = append(lines, string(src[start:end]))
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
