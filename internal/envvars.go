package internal

import (
	"bufio"
	"bytes"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// envVarExtensions lists file extensions whose content is scanned for env var
// references. .env* files are handled separately by envVarFromDotEnv.
var envVarExtensions = map[string]bool{
	".js": true, ".mjs": true, ".cjs": true,
	".ts": true, ".tsx": true, ".jsx": true,
	".py": true,
	".go": true,
	".rb": true,
}

// patterns that read an env var with no fallback (required)
// and patterns that read with a fallback (has_default).
var envPatterns = []struct {
	re         *regexp.Regexp
	hasDefault bool
}{
	// JS/TS: process.env.VAR_NAME
	{regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)\b`), false},
	// JS/TS: process.env.VAR_NAME ?? / || 'default'  (has default)
	{regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)\s*(?:\?\?|(?:\|\|))`), true},
	// Python: os.environ["VAR"] or os.environ['VAR']
	{regexp.MustCompile(`os\.environ\[["']([A-Z_][A-Z0-9_]*)["']\]`), false},
	// Python: os.environ.get("VAR") or os.environ.get("VAR", default)
	{regexp.MustCompile(`os\.environ\.get\(["']([A-Z_][A-Z0-9_]*)["']`), true},
	// Python: os.getenv("VAR") — may or may not have default; treat as has_default
	{regexp.MustCompile(`os\.getenv\(["']([A-Z_][A-Z0-9_]*)["']`), true},
	// Go: os.Getenv("VAR")
	{regexp.MustCompile(`os\.Getenv\(["` + "`" + `]([A-Z_][A-Z0-9_]*)["` + "`" + `]\)`), true},
	// Go: os.LookupEnv("VAR")
	{regexp.MustCompile(`os\.LookupEnv\(["` + "`" + `]([A-Z_][A-Z0-9_]*)["` + "`" + `]\)`), false},
	// Ruby: ENV["VAR"] or ENV['VAR']
	{regexp.MustCompile(`ENV\[["']([A-Z_][A-Z0-9_]*)["']\]`), false},
	// Ruby: ENV.fetch("VAR", default)
	{regexp.MustCompile(`ENV\.fetch\(["']([A-Z_][A-Z0-9_]*)["']`), true},
}

// envFileNameRe matches .env, .env.local, .env.production, etc.
var envFileNameRe = regexp.MustCompile(`(?i)^\.env`)

// extractEnvVars scans the given file content for environment variable
// references, returning a slice of (name, hasDefault) pairs.
func extractEnvVars(src []byte, ext string) []struct {
	name       string
	hasDefault bool
} {
	if !envVarExtensions[ext] {
		return nil
	}
	var results []struct {
		name       string
		hasDefault bool
	}
	seen := map[string]bool{}
	for _, p := range envPatterns {
		matches := p.re.FindAllSubmatch(src, -1)
		for _, m := range matches {
			name := string(m[1])
			if seen[name] {
				continue
			}
			seen[name] = true
			results = append(results, struct {
				name       string
				hasDefault bool
			}{name, p.hasDefault})
		}
	}
	return results
}

// parseDotEnv reads key=value lines from a .env file, returning all defined
// variable names. Comments (#) and blank lines are skipped.
func parseDotEnv(src []byte) []string {
	var names []string
	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineRe := regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := lineRe.FindStringSubmatch(line); m != nil {
			names = append(names, m[1])
		}
	}
	return names
}

// mergeEnvVars takes a map[name]→{files, hasDefault, required} and converts
// it to a sorted []EnvVar slice.
func mergeEnvVars(byName map[string]*EnvVar) []EnvVar {
	out := make([]EnvVar, 0, len(byName))
	for _, ev := range byName {
		ev.Required = !ev.HasDefault
		out = append(out, *ev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// collectEnvVars walks allPaths (absolute), reads each file, and returns
// an aggregated env var inventory.  displayRoot is used to produce relative
// display paths.
func collectEnvVars(allPaths []string, displayRoot string) []EnvVar {
	byName := map[string]*EnvVar{}

	ensure := func(name string) *EnvVar {
		if byName[name] == nil {
			byName[name] = &EnvVar{Name: name}
		}
		return byName[name]
	}

	addFile := func(ev *EnvVar, path string) {
		rel := relativePath(path, displayRoot)
		for _, f := range ev.Files {
			if f == rel {
				return
			}
		}
		ev.Files = append(ev.Files, rel)
	}

	for _, path := range allPaths {
		base := filepath.Base(path)
		ext := strings.ToLower(filepath.Ext(path))

		src, err := readFileSafe(path)
		if err != nil {
			continue
		}

		// Handle .env* files
		if envFileNameRe.MatchString(base) {
			for _, name := range parseDotEnv(src) {
				ev := ensure(name)
				ev.HasDefault = true // defined in .env = has a value
				addFile(ev, path)
			}
			continue
		}

		// Source code files
		for _, hit := range extractEnvVars(src, ext) {
			ev := ensure(hit.name)
			if hit.hasDefault {
				ev.HasDefault = true
			}
			addFile(ev, path)
		}
	}

	return mergeEnvVars(byName)
}
