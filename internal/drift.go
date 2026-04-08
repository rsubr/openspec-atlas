package internal

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

// parseDriftKind validates a --fail-on flag value and returns the corresponding
// DriftKind. "none" is accepted and returned as an empty DriftKind.
func parseDriftKind(s string) (DriftKind, error) {
	switch s {
	case "added", "removed", "changed", "none":
		return DriftKind(s), nil
	default:
		return "", fmt.Errorf("invalid --fail-on value %q: must be added, removed, changed, or none", s)
	}
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

// diffByKey is a generic helper that computes added/removed/changed DriftIssues
// from two slices. key extracts a string map key from an item; removed and added
// build the DriftIssue for each case; changed, if non-nil, compares a baseline
// item against its current counterpart and returns (issue, true) when they differ.
func diffByKey[T any](
	baseline, current []T,
	key     func(T) string,
	removed func(b T) DriftIssue,
	added   func(c T) DriftIssue,
	changed func(b, c T) (DriftIssue, bool),
) []DriftIssue {
	bMap := make(map[string]T, len(baseline))
	for _, v := range baseline {
		bMap[key(v)] = v
	}
	cMap := make(map[string]T, len(current))
	for _, v := range current {
		cMap[key(v)] = v
	}

	var issues []DriftIssue
	for k, bv := range bMap {
		if cv, ok := cMap[k]; !ok {
			issues = append(issues, removed(bv))
		} else if changed != nil {
			if iss, ok := changed(bv, cv); ok {
				issues = append(issues, iss)
			}
		}
	}
	for k, cv := range cMap {
		if _, ok := bMap[k]; !ok {
			issues = append(issues, added(cv))
		}
	}
	return issues
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
			issues = append(issues, DriftIssue{Kind: DriftRemoved, Category: "endpoint", Name: key, File: file})
		}
	}
	for key, file := range cMap {
		if _, ok := bMap[key]; !ok {
			issues = append(issues, DriftIssue{Kind: DriftAdded, Category: "endpoint", Name: key, File: file})
		}
	}
	return issues
}

func diffEnvVars(baseline, current []EnvVar) []DriftIssue {
	return diffByKey(
		baseline, current,
		func(v EnvVar) string { return v.Name },
		func(b EnvVar) DriftIssue {
			return DriftIssue{Kind: DriftRemoved, Category: "env_var", Name: b.Name}
		},
		func(c EnvVar) DriftIssue {
			return DriftIssue{Kind: DriftAdded, Category: "env_var", Name: c.Name}
		},
		func(b, c EnvVar) (DriftIssue, bool) {
			if c.HasDefault == b.HasDefault && c.Required == b.Required {
				return DriftIssue{}, false
			}
			detail := fmt.Sprintf("has_default: %v → %v", b.HasDefault, c.HasDefault)
			if c.Required != b.Required {
				detail = fmt.Sprintf("required: %v → %v", b.Required, c.Required)
			}
			return DriftIssue{Kind: DriftChanged, Category: "env_var", Name: b.Name, Detail: detail}, true
		},
	)
}

func diffSchemaModels(baseline, current []SchemaModel) []DriftIssue {
	return diffByKey(
		baseline, current,
		func(m SchemaModel) string { return m.ORM + "::" + m.Name },
		func(b SchemaModel) DriftIssue {
			return DriftIssue{Kind: DriftRemoved, Category: "schema_model", Name: b.Name, File: b.File, Detail: b.ORM}
		},
		func(c SchemaModel) DriftIssue {
			return DriftIssue{Kind: DriftAdded, Category: "schema_model", Name: c.Name, File: c.File, Detail: c.ORM}
		},
		func(b, c SchemaModel) (DriftIssue, bool) {
			if len(c.Fields) == len(b.Fields) {
				return DriftIssue{}, false
			}
			detail := fmt.Sprintf("fields: %d → %d", len(b.Fields), len(c.Fields))
			return DriftIssue{Kind: DriftChanged, Category: "schema_model", Name: b.Name, File: c.File, Detail: detail}, true
		},
	)
}

func diffMiddleware(baseline, current []MiddlewareItem) []DriftIssue {
	return diffByKey(
		baseline, current,
		func(m MiddlewareItem) string { return m.Framework + "::" + m.File + "::" + m.Name },
		func(b MiddlewareItem) DriftIssue {
			return DriftIssue{Kind: DriftRemoved, Category: "middleware", Name: b.Name, File: b.File, Detail: b.Framework + "/" + b.Type}
		},
		func(c MiddlewareItem) DriftIssue {
			return DriftIssue{Kind: DriftAdded, Category: "middleware", Name: c.Name, File: c.File, Detail: c.Framework + "/" + c.Type}
		},
		nil,
	)
}

func diffUIComponents(baseline, current []UIComponent) []DriftIssue {
	return diffByKey(
		baseline, current,
		func(c UIComponent) string { return c.Framework + "::" + c.File + "::" + c.Name },
		func(b UIComponent) DriftIssue {
			return DriftIssue{Kind: DriftRemoved, Category: "ui_component", Name: b.Name, File: b.File, Detail: b.Framework}
		},
		func(c UIComponent) DriftIssue {
			return DriftIssue{Kind: DriftAdded, Category: "ui_component", Name: c.Name, File: c.File, Detail: c.Framework}
		},
		nil,
	)
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

// loadOutputFile reads and unmarshals an Output JSON file.
func loadOutputFile(path string) (Output, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Output{}, fmt.Errorf("read %s: %w", path, err)
	}
	var out Output
	if err := json.Unmarshal(data, &out); err != nil {
		return Output{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return out, nil
}

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

	failOnKind, err := parseDriftKind(*failOn)
	if err != nil {
		return err
	}

	baseline, err := loadOutputFile(*baselinePath)
	if err != nil {
		return err
	}

	var current Output
	usedCurrentFile := *currentPath
	if *currentPath != "" {
		current, err = loadOutputFile(*currentPath)
		if err != nil {
			return err
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

	if failOnKind != "none" && countKind(report.Issues, failOnKind) > 0 {
		return fmt.Errorf("drift detected: %d %s issue(s)", countKind(report.Issues, failOnKind), failOnKind)
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
