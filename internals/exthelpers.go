package internals

import (
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

// normalizeHTTPPath strips common API prefixes and replaces path parameter
// patterns ({id}, :id, <id>) with a canonical {param} placeholder.
func normalizeHTTPPath(raw string) string {
	// Strip query string
	if idx := strings.IndexByte(raw, '?'); idx >= 0 {
		raw = raw[:idx]
	}
	// Strip common API version prefixes
	for _, pfx := range []string{"/api/v1", "/api/v2", "/api/v3", "/api"} {
		if strings.HasPrefix(raw, pfx+"/") || raw == pfx {
			raw = raw[len(pfx):]
			break
		}
	}
	if raw == "" {
		raw = "/"
	}
	// Normalise path param styles: :id, {id}, <id> → {param}
	paramRe := strings.NewReplacer()
	_ = paramRe
	segments := strings.Split(raw, "/")
	for i, seg := range segments {
		if len(seg) > 1 && (seg[0] == ':' || (seg[0] == '{' && seg[len(seg)-1] == '}') || (seg[0] == '<' && seg[len(seg)-1] == '>')) {
			segments[i] = "{param}"
		}
	}
	return strings.Join(segments, "/")
}
