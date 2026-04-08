package internals

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

// DriftKind classifies how an item changed between baseline and current.
type DriftKind string

const (
	DriftAdded   DriftKind = "added"
	DriftRemoved DriftKind = "removed"
	DriftChanged DriftKind = "changed"
)

// DriftIssue describes a single detected change.
type DriftIssue struct {
	Kind     DriftKind `json:"kind"`
	Category string    `json:"category"` // symbol, endpoint, env_var, schema_model, middleware, ui_component
	Name     string    `json:"name"`
	File     string    `json:"file,omitempty"`
	Detail   string    `json:"detail,omitempty"`
}

// DriftSummary aggregates counts by kind.
type DriftSummary struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
	Total   int `json:"total"`
}

// DriftReport is the top-level output of the drift command.
type DriftReport struct {
	BaselineFile string       `json:"baseline_file"`
	CurrentFile  string       `json:"current_file,omitempty"`
	Issues       []DriftIssue `json:"issues"`
	Summary      DriftSummary `json:"summary"`
}

// --------------------------------------------------------------------------
// Diff helpers
// --------------------------------------------------------------------------

// symEntry holds a symbol and the file it came from for diff purposes.
type symEntry struct {
	sym  Symbol
	file string
}

// flattenSymbols recursively walks all symbols in all files, returning a map
// of "file::name" → symEntry for efficient lookup.
func flattenSymbols(files []FileInfo) map[string]symEntry {
	m := map[string]symEntry{}
	var walk func(syms []Symbol, file string)
	walk = func(syms []Symbol, file string) {
		for _, s := range syms {
			key := file + "::" + s.Name
			m[key] = symEntry{sym: s, file: file}
			walk(s.Children, file)
		}
	}
	for _, fi := range files {
		walk(fi.Symbols, fi.Path)
	}
	return m
}

// flattenEndpoints collects all endpoint-bearing symbols across all files.
func flattenEndpoints(files []FileInfo) map[string]string {
	m := map[string]string{} // "METHOD path" → file
	var walk func(syms []Symbol, file string)
	walk = func(syms []Symbol, file string) {
		for _, s := range syms {
			if s.Endpoint != nil {
				key := s.Endpoint.Method + " " + s.Endpoint.Path
				m[key] = file
			}
			walk(s.Children, file)
		}
	}
	for _, fi := range files {
		walk(fi.Symbols, fi.Path)
	}
	return m
}

// --------------------------------------------------------------------------
// Individual diff functions
// --------------------------------------------------------------------------

func diffSymbols(baseline, current []FileInfo) []DriftIssue {
	bMap := flattenSymbols(baseline)
	cMap := flattenSymbols(current)
	var issues []DriftIssue

	for key, be := range bMap {
		if ce, ok := cMap[key]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftRemoved,
				Category: "symbol",
				Name:     be.sym.Name,
				File:     be.file,
				Detail:   be.sym.Kind,
			})
		} else if ce.sym.Kind != be.sym.Kind {
			issues = append(issues, DriftIssue{
				Kind:     DriftChanged,
				Category: "symbol",
				Name:     be.sym.Name,
				File:     be.file,
				Detail:   fmt.Sprintf("kind changed: %s → %s", be.sym.Kind, ce.sym.Kind),
			})
		}
	}
	for key, ce := range cMap {
		if _, ok := bMap[key]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftAdded,
				Category: "symbol",
				Name:     ce.sym.Name,
				File:     ce.file,
				Detail:   ce.sym.Kind,
			})
		}
	}
	return issues
}

func diffEndpoints(baseline, current []FileInfo) []DriftIssue {
	bMap := flattenEndpoints(baseline)
	cMap := flattenEndpoints(current)
	var issues []DriftIssue

	for key, file := range bMap {
		if _, ok := cMap[key]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftRemoved,
				Category: "endpoint",
				Name:     key,
				File:     file,
			})
		}
	}
	for key, file := range cMap {
		if _, ok := bMap[key]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftAdded,
				Category: "endpoint",
				Name:     key,
				File:     file,
			})
		}
	}
	return issues
}

func diffEnvVars(baseline, current []EnvVar) []DriftIssue {
	bMap := map[string]EnvVar{}
	for _, v := range baseline {
		bMap[v.Name] = v
	}
	cMap := map[string]EnvVar{}
	for _, v := range current {
		cMap[v.Name] = v
	}

	var issues []DriftIssue
	for name, bv := range bMap {
		if cv, ok := cMap[name]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftRemoved,
				Category: "env_var",
				Name:     name,
			})
		} else if cv.HasDefault != bv.HasDefault || cv.Required != bv.Required {
			detail := ""
			if cv.Required != bv.Required {
				detail = fmt.Sprintf("required: %v → %v", bv.Required, cv.Required)
			} else {
				detail = fmt.Sprintf("has_default: %v → %v", bv.HasDefault, cv.HasDefault)
			}
			issues = append(issues, DriftIssue{
				Kind:     DriftChanged,
				Category: "env_var",
				Name:     name,
				Detail:   detail,
			})
		}
	}
	for name := range cMap {
		if _, ok := bMap[name]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftAdded,
				Category: "env_var",
				Name:     name,
			})
		}
	}
	return issues
}

func diffSchemaModels(baseline, current []SchemaModel) []DriftIssue {
	key := func(m SchemaModel) string { return m.ORM + "::" + m.Name }

	bMap := map[string]SchemaModel{}
	for _, m := range baseline {
		bMap[key(m)] = m
	}
	cMap := map[string]SchemaModel{}
	for _, m := range current {
		cMap[key(m)] = m
	}

	var issues []DriftIssue
	for k, bm := range bMap {
		if cm, ok := cMap[k]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftRemoved,
				Category: "schema_model",
				Name:     bm.Name,
				File:     bm.File,
				Detail:   bm.ORM,
			})
		} else if len(cm.Fields) != len(bm.Fields) {
			issues = append(issues, DriftIssue{
				Kind:     DriftChanged,
				Category: "schema_model",
				Name:     bm.Name,
				File:     cm.File,
				Detail:   fmt.Sprintf("fields: %d → %d", len(bm.Fields), len(cm.Fields)),
			})
		}
	}
	for k, cm := range cMap {
		if _, ok := bMap[k]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftAdded,
				Category: "schema_model",
				Name:     cm.Name,
				File:     cm.File,
				Detail:   cm.ORM,
			})
		}
	}
	return issues
}

func diffMiddleware(baseline, current []MiddlewareItem) []DriftIssue {
	key := func(m MiddlewareItem) string { return m.Framework + "::" + m.File + "::" + m.Name }

	bSet := map[string]MiddlewareItem{}
	for _, m := range baseline {
		bSet[key(m)] = m
	}
	cSet := map[string]MiddlewareItem{}
	for _, m := range current {
		cSet[key(m)] = m
	}

	var issues []DriftIssue
	for k, bm := range bSet {
		if _, ok := cSet[k]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftRemoved,
				Category: "middleware",
				Name:     bm.Name,
				File:     bm.File,
				Detail:   bm.Framework + "/" + bm.Type,
			})
		}
	}
	for k, cm := range cSet {
		if _, ok := bSet[k]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftAdded,
				Category: "middleware",
				Name:     cm.Name,
				File:     cm.File,
				Detail:   cm.Framework + "/" + cm.Type,
			})
		}
	}
	return issues
}

func diffUIComponents(baseline, current []UIComponent) []DriftIssue {
	key := func(c UIComponent) string { return c.Framework + "::" + c.File + "::" + c.Name }

	bSet := map[string]UIComponent{}
	for _, c := range baseline {
		bSet[key(c)] = c
	}
	cSet := map[string]UIComponent{}
	for _, c := range current {
		cSet[key(c)] = c
	}

	var issues []DriftIssue
	for k, bc := range bSet {
		if _, ok := cSet[k]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftRemoved,
				Category: "ui_component",
				Name:     bc.Name,
				File:     bc.File,
				Detail:   bc.Framework,
			})
		}
	}
	for k, cc := range cSet {
		if _, ok := bSet[k]; !ok {
			issues = append(issues, DriftIssue{
				Kind:     DriftAdded,
				Category: "ui_component",
				Name:     cc.Name,
				File:     cc.File,
				Detail:   cc.Framework,
			})
		}
	}
	return issues
}

// --------------------------------------------------------------------------
// Report builder
// --------------------------------------------------------------------------

func buildDriftReport(baseline, current Output, baselineFile, currentFile string) DriftReport {
	var issues []DriftIssue
	issues = append(issues, diffSymbols(baseline.Files, current.Files)...)
	issues = append(issues, diffEndpoints(baseline.Files, current.Files)...)
	issues = append(issues, diffEnvVars(baseline.EnvVars, current.EnvVars)...)
	issues = append(issues, diffSchemaModels(baseline.SchemaModels, current.SchemaModels)...)
	issues = append(issues, diffMiddleware(baseline.Middleware, current.Middleware)...)
	issues = append(issues, diffUIComponents(baseline.UIComponents, current.UIComponents)...)

	// Sort: category → kind (removed first, then changed, then added) → name
	kindOrder := map[DriftKind]int{DriftRemoved: 0, DriftChanged: 1, DriftAdded: 2}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Category != issues[j].Category {
			return issues[i].Category < issues[j].Category
		}
		if issues[i].Kind != issues[j].Kind {
			return kindOrder[issues[i].Kind] < kindOrder[issues[j].Kind]
		}
		return issues[i].Name < issues[j].Name
	})

	summary := DriftSummary{}
	for _, iss := range issues {
		switch iss.Kind {
		case DriftAdded:
			summary.Added++
		case DriftRemoved:
			summary.Removed++
		case DriftChanged:
			summary.Changed++
		}
	}
	summary.Total = summary.Added + summary.Removed + summary.Changed

	return DriftReport{
		BaselineFile: baselineFile,
		CurrentFile:  currentFile,
		Issues:       issues,
		Summary:      summary,
	}
}

// --------------------------------------------------------------------------
// CLI handler
// --------------------------------------------------------------------------

func runDrift(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("openspec-atlas drift", flag.ContinueOnError)
	fs.SetOutput(stderr)

	baselinePath := fs.String("baseline", "", "path to baseline structure.json (required)")
	currentPath := fs.String("current", "", "path to current structure.json (optional; re-scans if absent)")
	jsonOut := fs.Bool("json", false, "output machine-readable JSON")
	failOn := fs.String("fail-on", "removed", "exit 1 if any issue of this kind exists: added, removed, changed, none")
	allFiles := fs.Bool("all", false, "ignore .gitignore files when re-scanning")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: openspec-atlas drift --baseline <path> [--current <path>] [--json] [--fail-on removed] [dir ...]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *baselinePath == "" {
		fs.Usage()
		return fmt.Errorf("--baseline is required")
	}

	// Load baseline
	baselineData, err := os.ReadFile(*baselinePath)
	if err != nil {
		return fmt.Errorf("read baseline %s: %w", *baselinePath, err)
	}
	var baseline Output
	if err := json.Unmarshal(baselineData, &baseline); err != nil {
		return fmt.Errorf("parse baseline %s: %w", *baselinePath, err)
	}

	// Obtain current output
	var current Output
	usedCurrentFile := *currentPath
	if *currentPath != "" {
		data, err := os.ReadFile(*currentPath)
		if err != nil {
			return fmt.Errorf("read current %s: %w", *currentPath, err)
		}
		if err := json.Unmarshal(data, &current); err != nil {
			return fmt.Errorf("parse current %s: %w", *currentPath, err)
		}
	} else {
		dirs := fs.Args()
		if len(dirs) == 0 {
			fs.Usage()
			return fmt.Errorf("provide at least one directory to scan, or --current <path>")
		}
		current = scanProjects(dirs, *allFiles, io.Discard, stderr)
	}

	report := buildDriftReport(baseline, current, *baselinePath, usedCurrentFile)

	if *jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal report: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
	} else {
		printDriftReport(report, stdout)
	}

	// Exit behaviour
	if *failOn != "none" {
		for _, iss := range report.Issues {
			if string(iss.Kind) == *failOn {
				return fmt.Errorf("drift detected: %d %s issue(s)", countKind(report.Issues, DriftKind(*failOn)), *failOn)
			}
		}
	}
	return nil
}

func countKind(issues []DriftIssue, kind DriftKind) int {
	n := 0
	for _, iss := range issues {
		if iss.Kind == kind {
			n++
		}
	}
	return n
}

// --------------------------------------------------------------------------
// Human-readable output
// --------------------------------------------------------------------------

func printDriftReport(r DriftReport, w io.Writer) {
	if r.Summary.Total == 0 {
		fmt.Fprintln(w, "no drift detected")
		return
	}

	kindLabel := map[DriftKind]string{
		DriftAdded:   "+ added  ",
		DriftRemoved: "- removed",
		DriftChanged: "~ changed",
	}

	prevCat := ""
	for _, iss := range r.Issues {
		if iss.Category != prevCat {
			fmt.Fprintf(w, "\n[%s]\n", iss.Category)
			prevCat = iss.Category
		}
		label := kindLabel[iss.Kind]
		loc := ""
		if iss.File != "" {
			loc = "  (" + iss.File + ")"
		}
		detail := ""
		if iss.Detail != "" {
			detail = "  — " + iss.Detail
		}
		fmt.Fprintf(w, "  %s  %s%s%s\n", label, iss.Name, detail, loc)
	}

	fmt.Fprintf(w, "\nsummary: +%d added  -%d removed  ~%d changed  (%d total)\n",
		r.Summary.Added, r.Summary.Removed, r.Summary.Changed, r.Summary.Total)
}
