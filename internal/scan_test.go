package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWalkSourceFilesContinuesOnReadError ensures a single unreadable file does
// not abort the directory walk or prevent healthy files from being parsed.
func TestWalkSourceFilesContinuesOnReadError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "ok.go"), "package demo\n\nfunc OK() {}\n")
	badPath := filepath.Join(root, "bad.go")
	mustWriteFile(t, badPath, "package demo\n\nfunc Broken() {}\n")
	if err := os.Chmod(badPath, 0); err != nil {
		t.Fatalf("chmod bad.go: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(badPath, 0o644) })

	var stdout, stderr bytes.Buffer
	files, allPaths := walkSourceFiles([]string{root}, false, &stdout, &stderr)

	if len(allPaths) != 2 {
		t.Fatalf("len(allPaths) = %d, want 2", len(allPaths))
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	if files[0].Symbols[0].Name != "OK" {
		t.Fatalf("unexpected parsed symbol %#v", files[0].Symbols)
	}
	if !strings.Contains(stderr.String(), "parse error in") {
		t.Fatalf("expected parse error in stderr, got %q", stderr.String())
	}
}
