package internal

import (
	"testing"
)

// helper: find issues of a given kind and category
func findIssues(issues []DriftIssue, kind DriftKind, category string) []DriftIssue {
	var out []DriftIssue
	for _, iss := range issues {
		if iss.Kind == kind && iss.Category == category {
			out = append(out, iss)
		}
	}
	return out
}

func issueNames(issues []DriftIssue) []string {
	names := make([]string, len(issues))
	for i, iss := range issues {
		names[i] = iss.Name
	}
	return names
}

// --------------------------------------------------------------------------
// diffSymbols
// --------------------------------------------------------------------------

func TestDiffSymbols(t *testing.T) {
	base := []FileInfo{
		{Path: "pkg/foo.go", Language: "go", Symbols: []Symbol{
			{Name: "Foo", Kind: "struct", Line: 1},
			{Name: "Bar", Kind: "function", Line: 10},
		}},
	}
	cur := []FileInfo{
		{Path: "pkg/foo.go", Language: "go", Symbols: []Symbol{
			{Name: "Foo", Kind: "interface", Line: 1}, // kind changed
			{Name: "Baz", Kind: "function", Line: 20}, // added
			// Bar removed
		}},
	}

	issues := diffSymbols(base, cur)

	removed := findIssues(issues, DriftRemoved, "symbol")
	if len(removed) != 1 || removed[0].Name != "Bar" {
		t.Errorf("expected Bar removed, got %v", issueNames(removed))
	}

	added := findIssues(issues, DriftAdded, "symbol")
	if len(added) != 1 || added[0].Name != "Baz" {
		t.Errorf("expected Baz added, got %v", issueNames(added))
	}

	changed := findIssues(issues, DriftChanged, "symbol")
	if len(changed) != 1 || changed[0].Name != "Foo" {
		t.Errorf("expected Foo changed, got %v", issueNames(changed))
	}
}

func TestDiffSymbols_NoChange(t *testing.T) {
	files := []FileInfo{
		{Path: "main.go", Symbols: []Symbol{{Name: "main", Kind: "function"}}},
	}
	issues := diffSymbols(files, files)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

func TestDiffSymbols_Nested(t *testing.T) {
	base := []FileInfo{{
		Path: "svc.go",
		Symbols: []Symbol{{
			Name: "Service", Kind: "struct",
			Children: []Symbol{
				{Name: "Create", Kind: "method"},
				{Name: "Delete", Kind: "method"},
			},
		}},
	}}
	cur := []FileInfo{{
		Path: "svc.go",
		Symbols: []Symbol{{
			Name: "Service", Kind: "struct",
			Children: []Symbol{
				{Name: "Create", Kind: "method"},
				// Delete removed, Update added
				{Name: "Update", Kind: "method"},
			},
		}},
	}}

	issues := diffSymbols(base, cur)
	removed := findIssues(issues, DriftRemoved, "symbol")
	added := findIssues(issues, DriftAdded, "symbol")
	if len(removed) != 1 || removed[0].Name != "Delete" {
		t.Errorf("expected Delete removed, got %v", issueNames(removed))
	}
	if len(added) != 1 || added[0].Name != "Update" {
		t.Errorf("expected Update added, got %v", issueNames(added))
	}
}

// --------------------------------------------------------------------------
// diffEndpoints
// --------------------------------------------------------------------------

func TestDiffEndpoints(t *testing.T) {
	ep := func(method, path string) *Endpoint { return &Endpoint{Method: method, Path: path} }
	base := []FileInfo{{
		Path: "handler.go",
		Symbols: []Symbol{
			{Name: "GetUser", Kind: "method", Endpoint: ep("GET", "/users/{id}")},
			{Name: "DeleteUser", Kind: "method", Endpoint: ep("DELETE", "/users/{id}")},
		},
	}}
	cur := []FileInfo{{
		Path: "handler.go",
		Symbols: []Symbol{
			{Name: "GetUser", Kind: "method", Endpoint: ep("GET", "/users/{id}")},
			{Name: "CreateUser", Kind: "method", Endpoint: ep("POST", "/users")},
			// DeleteUser removed
		},
	}}

	issues := diffEndpoints(base, cur)
	removed := findIssues(issues, DriftRemoved, "endpoint")
	added := findIssues(issues, DriftAdded, "endpoint")

	if len(removed) != 1 || removed[0].Name != "DELETE /users/{id}" {
		t.Errorf("expected DELETE /users/{id} removed, got %v", issueNames(removed))
	}
	if len(added) != 1 || added[0].Name != "POST /users" {
		t.Errorf("expected POST /users added, got %v", issueNames(added))
	}
}

// --------------------------------------------------------------------------
// diffEnvVars
// --------------------------------------------------------------------------

func TestDiffEnvVars(t *testing.T) {
	base := []EnvVar{
		{Name: "DATABASE_URL", Required: true},
		{Name: "API_KEY", HasDefault: true},
		{Name: "OLD_VAR"},
	}
	cur := []EnvVar{
		{Name: "DATABASE_URL", Required: true},          // unchanged
		{Name: "API_KEY", HasDefault: false, Required: true}, // changed
		{Name: "NEW_VAR"},                               // added
		// OLD_VAR removed
	}

	issues := diffEnvVars(base, cur)

	removed := findIssues(issues, DriftRemoved, "env_var")
	if len(removed) != 1 || removed[0].Name != "OLD_VAR" {
		t.Errorf("expected OLD_VAR removed, got %v", issueNames(removed))
	}

	added := findIssues(issues, DriftAdded, "env_var")
	if len(added) != 1 || added[0].Name != "NEW_VAR" {
		t.Errorf("expected NEW_VAR added, got %v", issueNames(added))
	}

	changed := findIssues(issues, DriftChanged, "env_var")
	if len(changed) != 1 || changed[0].Name != "API_KEY" {
		t.Errorf("expected API_KEY changed, got %v", issueNames(changed))
	}
}

// --------------------------------------------------------------------------
// diffSchemaModels
// --------------------------------------------------------------------------

func TestDiffSchemaModels(t *testing.T) {
	base := []SchemaModel{
		{Name: "User", ORM: "prisma", Fields: []SchemaField{{Name: "id"}, {Name: "email"}}},
		{Name: "Post", ORM: "prisma", Fields: []SchemaField{{Name: "id"}}},
	}
	cur := []SchemaModel{
		{Name: "User", ORM: "prisma", Fields: []SchemaField{{Name: "id"}, {Name: "email"}, {Name: "name"}}}, // field added
		{Name: "Comment", ORM: "prisma"},                                                                    // new model
		// Post removed
	}

	issues := diffSchemaModels(base, cur)

	removed := findIssues(issues, DriftRemoved, "schema_model")
	if len(removed) != 1 || removed[0].Name != "Post" {
		t.Errorf("expected Post removed, got %v", issueNames(removed))
	}

	added := findIssues(issues, DriftAdded, "schema_model")
	if len(added) != 1 || added[0].Name != "Comment" {
		t.Errorf("expected Comment added, got %v", issueNames(added))
	}

	changed := findIssues(issues, DriftChanged, "schema_model")
	if len(changed) != 1 || changed[0].Name != "User" {
		t.Errorf("expected User changed, got %v", issueNames(changed))
	}
}

// --------------------------------------------------------------------------
// diffMiddleware
// --------------------------------------------------------------------------

func TestDiffMiddleware(t *testing.T) {
	base := []MiddlewareItem{
		{Name: "cors", Framework: "express", File: "server.ts", Type: "cors"},
		{Name: "helmet", Framework: "express", File: "server.ts", Type: "auth"},
	}
	cur := []MiddlewareItem{
		{Name: "cors", Framework: "express", File: "server.ts", Type: "cors"},
		{Name: "morgan", Framework: "express", File: "server.ts", Type: "logging"},
		// helmet removed
	}

	issues := diffMiddleware(base, cur)

	removed := findIssues(issues, DriftRemoved, "middleware")
	if len(removed) != 1 || removed[0].Name != "helmet" {
		t.Errorf("expected helmet removed, got %v", issueNames(removed))
	}

	added := findIssues(issues, DriftAdded, "middleware")
	if len(added) != 1 || added[0].Name != "morgan" {
		t.Errorf("expected morgan added, got %v", issueNames(added))
	}
}

// --------------------------------------------------------------------------
// buildDriftReport
// --------------------------------------------------------------------------

func TestBuildDriftReport_Empty(t *testing.T) {
	out := Output{}
	report := buildDriftReport(out, out, "baseline.json", "")
	if report.Summary.Total != 0 {
		t.Errorf("expected zero issues for identical empty outputs, got %d", report.Summary.Total)
	}
}

func TestBuildDriftReport_Summary(t *testing.T) {
	base := Output{
		EnvVars: []EnvVar{{Name: "OLD"}},
	}
	cur := Output{
		EnvVars: []EnvVar{{Name: "NEW"}},
	}
	report := buildDriftReport(base, cur, "b.json", "c.json")
	if report.Summary.Removed != 1 {
		t.Errorf("expected 1 removed, got %d", report.Summary.Removed)
	}
	if report.Summary.Added != 1 {
		t.Errorf("expected 1 added, got %d", report.Summary.Added)
	}
	if report.Summary.Total != 2 {
		t.Errorf("expected total 2, got %d", report.Summary.Total)
	}
}
