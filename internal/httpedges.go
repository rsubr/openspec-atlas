package internal

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// httpClientExtensions is the set of file extensions we scan for HTTP calls.
var httpClientExtensions = map[string]bool{
	".js": true, ".mjs": true, ".cjs": true,
	".ts": true, ".tsx": true, ".jsx": true,
}

// httpCallPattern matches common HTTP client calls and captures (method, url).
// Groups:  [full, method-or-empty, url]
var httpCallPatterns = []*regexp.Regexp{
	// fetch("url") or fetch("url", { method: "POST" })
	regexp.MustCompile(`\bfetch\(\s*["` + "`" + `]([^"` + "`" + `]+)["` + "`" + `]`),
	// axios.get("url"), axios.post("url") …
	regexp.MustCompile(`\baxios\.(get|post|put|delete|patch|head|options)\(\s*["` + "`" + `]([^"` + "`" + `]+)["` + "`" + `]`),
	// ky.get("url"), got.get("url")
	regexp.MustCompile(`\b(?:ky|got)\.(get|post|put|delete|patch)\(\s*["` + "`" + `]([^"` + "`" + `]+)["` + "`" + `]`),
	// superagent.get("url")
	regexp.MustCompile(`\bsuperagent\.(get|post|put|delete|patch)\(\s*["` + "`" + `]([^"` + "`" + `]+)["` + "`" + `]`),
}

// fetchMethodPattern detects an explicit method inside a fetch options object
// on the same or nearby line.
var fetchMethodPattern = regexp.MustCompile(`(?i)method:\s*["'` + "`" + `](\w+)["'` + "`" + `]`)

// httpCallInfo holds a parsed HTTP call site.
type httpCallInfo struct {
	method string
	path   string
	line   int
}

// isInternalURL reports whether url should be collected as an internal API call.
// It accepts path-relative URLs (/...) and localhost/127.0.0.1 absolute URLs.
func isInternalURL(url string) bool {
	if strings.HasPrefix(url, "/") {
		return true
	}
	if strings.HasPrefix(url, "http") {
		return strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1")
	}
	return false
}

// extractURLPath strips the scheme and host from url and returns only the path.
// If url is already a path (starts with /), it is returned unchanged.
func extractURLPath(url string) string {
	if !strings.HasPrefix(url, "http") {
		return url
	}
	if idx := strings.Index(url, "://"); idx >= 0 {
		rest := url[idx+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			return rest[slash:]
		}
	}
	return "/"
}

// extractHTTPCalls scans source lines for outgoing HTTP calls.
func extractHTTPCalls(src []byte) []httpCallInfo {
	var results []httpCallInfo
	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNum := 0
	var prevLines [3]string // rolling window to detect fetch method

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Slide window
		prevLines[0], prevLines[1], prevLines[2] = prevLines[1], prevLines[2], line

		for _, pat := range httpCallPatterns {
			m := pat.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			var method, url string
			switch len(m) {
			case 2: // fetch pattern — url is m[1], method unknown yet
				url = m[1]
				method = "GET"
				// look for method: "POST" in the next or same line context (window)
				window := strings.Join(prevLines[:], " ") + " " + line
				if mm := fetchMethodPattern.FindStringSubmatch(window); mm != nil {
					method = strings.ToUpper(mm[1])
				}
			case 3: // axios/ky/got/superagent — m[1]=method, m[2]=url
				method = strings.ToUpper(m[1])
				url = m[2]
			}

			if !isInternalURL(url) {
				continue
			}
			url = extractURLPath(url)

			results = append(results, httpCallInfo{
				method: method,
				path:   url,
				line:   lineNum,
			})
		}
	}
	return results
}

// backendRoute is an endpoint extracted from already-parsed symbol annotations.
type backendRoute struct {
	method  string
	path    string
	file    string
	normPath string
}

// buildBackendRouteIndex builds a lookup of all resolved HTTP endpoints from
// the already-scanned FileInfo slice.
func buildBackendRouteIndex(files []FileInfo) []backendRoute {
	var routes []backendRoute
	var walk func(symbols []Symbol, file string)
	walk = func(symbols []Symbol, file string) {
		for _, sym := range symbols {
			if sym.Endpoint != nil {
				routes = append(routes, backendRoute{
					method:   sym.Endpoint.Method,
					path:     sym.Endpoint.Path,
					file:     file,
					normPath: normalizeHTTPPath(sym.Endpoint.Path),
				})
			}
			walk(sym.Children, file)
		}
	}
	for _, fi := range files {
		walk(fi.Symbols, fi.Path)
	}
	return routes
}

// matchRoute attempts to find a backend route for a call, returning confidence.
func matchRoute(call httpCallInfo, routes []backendRoute) (file, confidence string) {
	norm := normalizeHTTPPath(call.path)

	for _, r := range routes {
		if strings.EqualFold(r.method, call.method) && r.normPath == norm {
			return r.file, "exact"
		}
	}
	// Path match regardless of method
	for _, r := range routes {
		if r.normPath == norm {
			return r.file, "path"
		}
	}
	// Fuzzy: check if either normalized path is a suffix/prefix of the other
	for _, r := range routes {
		if strings.HasSuffix(r.normPath, norm) || strings.HasSuffix(norm, r.normPath) {
			return r.file, "fuzzy"
		}
	}
	return "", ""
}

// collectHTTPEdges scans all JS/TS source paths for outgoing HTTP calls and
// attempts to match them against the backend routes already discovered in
// files.
func collectHTTPEdges(allPaths []string, files []FileInfo, displayRoot string) []HttpEdge {
	routes := buildBackendRouteIndex(files)
	var edges []HttpEdge

	for _, path := range allPaths {
		ext := strings.ToLower(fileExt(path))
		if !httpClientExtensions[ext] {
			continue
		}
		src, err := readFileSafe(path)
		if err != nil || src == nil {
			continue
		}

		calls := extractHTTPCalls(src)
		displayPath := relativePath(path, displayRoot)
		for _, call := range calls {
			edge := HttpEdge{
				CallerFile: displayPath,
				CallerLine: call.line,
				Method:     call.method,
				Path:       call.path,
			}
			if len(routes) > 0 {
				edge.HandlerFile, edge.Confidence = matchRoute(call, routes)
			}
			edges = append(edges, edge)
		}
	}
	return edges
}

