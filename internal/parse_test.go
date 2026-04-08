package internal

import (
	"path/filepath"
	"testing"
)

// TestParseFileGo checks the basic Go parsing flow, including namespace
// extraction and distinguishing methods from top-level functions.
func TestParseFileGo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	mustWriteFile(t, path, `package demo

type UserService struct{}

func (s *UserService) FindUser() {}

func TopLevel() {}
`)

	config, ok := languageForFile(path)
	if !ok {
		t.Fatalf("languageForFile(%q) not found", path)
	}

	fi, err := parseFile(path, config)
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if fi.Language != "go" {
		t.Fatalf("Language = %q, want go", fi.Language)
	}
	if fi.Namespace != "demo" {
		t.Fatalf("Namespace = %q, want demo", fi.Namespace)
	}
	if len(fi.Symbols) != 3 {
		t.Fatalf("len(Symbols) = %d, want 3", len(fi.Symbols))
	}
	if fi.Symbols[0].Name != "UserService" || len(fi.Symbols[0].Children) != 0 {
		t.Fatalf("unexpected struct symbol: %#v", fi.Symbols[0])
	}

	foundTopLevel := false
	foundMethod := false
	for _, sym := range fi.Symbols[1:] {
		if sym.Name == "TopLevel" && sym.Kind == "function" {
			foundTopLevel = true
		}
		if sym.Name == "FindUser" && sym.Kind == "method" {
			foundMethod = true
		}
	}
	if !foundTopLevel || !foundMethod {
		t.Fatalf("unexpected trailing symbols: %#v", fi.Symbols[1:])
	}
}

// TestParseFileJavaSpringController verifies that Java annotations are captured
// and post-processed into fully resolved Spring endpoints.
func TestParseFileJavaSpringController(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "UserController.java")
	mustWriteFile(t, path, `package demo.api;

@RestController
@RequestMapping("/api/users")
class UserController {
  @GetMapping("/{id}")
  User show() { return null; }
}
`)

	config, ok := languageForFile(path)
	if !ok {
		t.Fatalf("languageForFile(%q) not found", path)
	}

	fi, err := parseFile(path, config)
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if fi.Namespace != "demo.api" {
		t.Fatalf("Namespace = %q, want demo.api", fi.Namespace)
	}
	if len(fi.Symbols) != 1 {
		t.Fatalf("len(Symbols) = %d, want 1", len(fi.Symbols))
	}

	controller := fi.Symbols[0]
	if controller.Name != "UserController" {
		t.Fatalf("controller.Name = %q, want UserController", controller.Name)
	}
	if len(controller.Children) != 1 {
		t.Fatalf("len(controller.Children) = %d, want 1", len(controller.Children))
	}
	if controller.Children[0].Endpoint == nil {
		t.Fatal("expected method endpoint to be resolved")
	}
	if controller.Children[0].Endpoint.Method != HTTPMethodGet || controller.Children[0].Endpoint.Path != "/api/users/{id}" {
		t.Fatalf("unexpected endpoint: %#v", controller.Children[0].Endpoint)
	}
}
