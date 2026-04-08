package internal

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunRequiresDirectory verifies that the default CLI mode prints usage and
// returns an error when no scan target is provided.
func TestRunRequiresDirectory(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := run(nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when no directory is provided")
	}
	if !strings.Contains(stderr.String(), "usage: openspec-atlas") {
		t.Fatalf("expected usage output, got %q", stderr.String())
	}
}

// TestRunWritesOutputJSON verifies the end-to-end happy path: ignored files are
// skipped, the atlas is written, and the generated JSON contains parsed
// symbols.
func TestRunWritesOutputJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".gitignore"), "ignored.go\n")
	mustWriteFile(t, filepath.Join(root, "main.go"), `package demo

func TopLevel() {}
`)
	mustWriteFile(t, filepath.Join(root, "ignored.go"), `package demo
func Ignored() {}
`)

	outputPath := filepath.Join(root, "out.json")
	var stdout, stderr bytes.Buffer

	if err := run([]string{"-o", outputPath, root}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v, stderr = %q", err, stderr.String())
	}

	data := mustReadFile(t, outputPath)
	var out Output
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(out.Files) != 1 {
		t.Fatalf("len(Files) = %d, want 1", len(out.Files))
	}
	if strings.Contains(stdout.String(), "ignored.go") {
		t.Fatalf("did not expect ignored file to be parsed, stdout = %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "generated") {
		t.Fatalf("expected generated message, stdout = %q", stdout.String())
	}
	if got := out.Files[0].Symbols[0].Name; got != "TopLevel" {
		t.Fatalf("top-level symbol = %q, want TopLevel", got)
	}
}

// TestRunVersionFlag ensures the version flag bypasses scanning and writes only
// the build version string.
func TestRunVersionFlag(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--version"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(--version) error = %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "openspec-atlas "+Version {
		t.Fatalf("unexpected version output %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
}
