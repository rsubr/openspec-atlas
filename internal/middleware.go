package internal

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// middlewareRule matches a line of source and maps it to a (name, type, framework).
type middlewareRule struct {
	re        *regexp.Regexp
	name      string // static name; if empty, use capture group 1
	mwType    string
	framework string
}

// expressRules detects Express / Fastify / Hono middleware registrations.
var expressRules = []middlewareRule{
	// app.use(helmet())
	{regexp.MustCompile(`\.use\(\s*helmet\s*\(`), "helmet", "auth", "express"},
	// app.use(cors())
	{regexp.MustCompile(`\.use\(\s*cors\s*\(`), "cors", "cors", "express"},
	// app.use(rateLimit(...))
	{regexp.MustCompile(`\.use\(\s*(?:rateLimit|rateLimiter|expressRateLimit)\s*\(`), "rate-limit", "rate-limit", "express"},
	// app.use(express.json()) or bodyParser.json()
	{regexp.MustCompile(`\.use\(\s*(?:express\.json|bodyParser\.json|bodyParser\.urlencoded)\s*\(`), "body-parser", "validation", "express"},
	// app.use(morgan(...))
	{regexp.MustCompile(`\.use\(\s*morgan\s*\(`), "morgan", "logging", "express"},
	// app.use(passport.initialize())
	{regexp.MustCompile(`\.use\(\s*passport\b`), "passport", "auth", "express"},
	// app.use(jwtMiddleware) or jwt({...})
	{regexp.MustCompile(`\.use\(\s*jwt\s*[({]`), "jwt", "auth", "express"},
	// app.use(session(...))
	{regexp.MustCompile(`\.use\(\s*session\s*\(`), "session", "auth", "express"},
	// app.use(errorHandler)
	{regexp.MustCompile(`\.use\(\s*errorHandler\b`), "errorHandler", "error-handler", "express"},
	// Generic: app.use(someMiddleware) — capture name
	{regexp.MustCompile(`\.use\(\s*(\w+)\s*\)`), "", "custom", "express"},
	// router.use(...)
	{regexp.MustCompile(`router\.use\(\s*(\w+)`), "", "custom", "express"},
}

// nestjsMiddlewareRules extracts NestJS guard/interceptor/pipe decorators
// that are already captured as symbol annotations.
// We process them from FileInfo directly (see collectMiddlewareFromSymbols).

// fastapiRules detects FastAPI Depends() usage.
var fastapiRules = []middlewareRule{
	{regexp.MustCompile(`Depends\(\s*(\w+)\s*\)`), "", "custom", "fastapi"},
	{regexp.MustCompile(`Depends\(\s*(\w+(?:\.\w+)?)\s*\)`), "", "custom", "fastapi"},
}

// nestjsAnnotationMiddleware maps NestJS decorator names to middleware types.
var nestjsAnnotationMiddleware = map[string]struct {
	mwType string
	name   string
}{
	"UseGuards":       {"auth", "UseGuards"},
	"UseInterceptors": {"custom", "UseInterceptors"},
	"UsePipes":        {"validation", "UsePipes"},
	"UseFilters":      {"error-handler", "UseFilters"},
}

// collectMiddlewareFromSymbols extracts NestJS middleware from already-parsed
// symbol annotations.
func collectMiddlewareFromSymbols(files []FileInfo) []MiddlewareItem {
	var items []MiddlewareItem
	var walk func(syms []Symbol, file string)
	walk = func(syms []Symbol, file string) {
		for _, sym := range syms {
			for annName, meta := range nestjsAnnotationMiddleware {
				if val := annotationValue(sym.Annotations, annName); val != "" {
					items = append(items, MiddlewareItem{
						Name:      val,
						Type:      meta.mwType,
						Framework: "nestjs",
						File:      file,
						Line:      int(sym.Line),
					})
				} else if hasAnnotation(sym.Annotations, annName) {
					items = append(items, MiddlewareItem{
						Name:      meta.name,
						Type:      meta.mwType,
						Framework: "nestjs",
						File:      file,
						Line:      int(sym.Line),
					})
				}
			}
			walk(sym.Children, file)
		}
	}
	for _, fi := range files {
		walk(fi.Symbols, fi.Path)
	}
	return items
}

// scanMiddlewareLine applies the given rules to a single source line.
func scanMiddlewareLine(line string, lineNum int, rules []middlewareRule, file string, items *[]MiddlewareItem) {
	for _, rule := range rules {
		m := rule.re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := rule.name
		if name == "" && len(m) > 1 {
			name = m[1]
		}
		if name == "" {
			continue
		}
		*items = append(*items, MiddlewareItem{
			Name:      name,
			Type:      rule.mwType,
			Framework: rule.framework,
			File:      file,
			Line:      lineNum,
		})
		return // one match per line per rule set
	}
}

// collectMiddlewareFromSource scans raw source files for middleware patterns.
func collectMiddlewareFromSource(allPaths []string, displayRoot string) []MiddlewareItem {
	var items []MiddlewareItem

	for _, path := range allPaths {
		ext := strings.ToLower(fileExt(path))
		var rules []middlewareRule
		switch ext {
		case ".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx":
			rules = expressRules
		case ".py":
			rules = fastapiRules
		default:
			continue
		}

		src, err := readFileSafe(path)
		if err != nil || src == nil {
			continue
		}

		displayPath := relativePath(path, displayRoot)
		scanner := bufio.NewScanner(bytes.NewReader(src))
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			scanMiddlewareLine(scanner.Text(), lineNum, rules, displayPath, &items)
		}
	}
	return items
}

// collectMiddleware returns all detected middleware from symbol annotations
// and raw source scans.
func collectMiddleware(allPaths []string, files []FileInfo, displayRoot string) []MiddlewareItem {
	var items []MiddlewareItem
	items = append(items, collectMiddlewareFromSymbols(files)...)
	items = append(items, collectMiddlewareFromSource(allPaths, displayRoot)...)
	return items
}
