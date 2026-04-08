package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// mustWriteFile creates parent directories as needed and fails the test
// immediately if the fixture file cannot be written.
func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

// mustReadFile reads a fixture file and aborts the test on any filesystem
// error.
func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}
