package internal

import (
	"path/filepath"
	"testing"

	ignore "github.com/sabhiram/go-gitignore"
)

func TestLanguageForFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{path: "main.go", want: "go"},
		{path: "controller.JAVA", want: "java"},
		{path: "page.TSX", want: "tsx"},
		{path: "script.sh", want: "bash"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got, ok := languageForFile(tt.path)
			if !ok {
				t.Fatalf("languageForFile(%q) not found", tt.path)
			}
			if got.Name != tt.want {
				t.Fatalf("languageForFile(%q) = %q, want %q", tt.path, got.Name, tt.want)
			}
		})
	}
}

func TestJoinPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		path string
		want string
	}{
		{name: "empty base", base: "", path: "/users", want: "/users"},
		{name: "trim trailing slash", base: "/api/", path: "/users", want: "/api/users"},
		{name: "add missing leading slash", base: "/api", path: "users", want: "/api/users"},
		{name: "empty child path", base: "/api", path: "", want: "/api"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := joinPaths(tt.base, tt.path); got != tt.want {
				t.Fatalf("joinPaths(%q, %q) = %q, want %q", tt.base, tt.path, got, tt.want)
			}
		})
	}
}

func TestResolveSpringEndpoints(t *testing.T) {
	t.Parallel()

	symbols := []Symbol{
		{
			Name: "UserController",
			Kind: "class",
			Annotations: []Annotation{
				{Name: "RequestMapping", Value: "/api/users"},
			},
			Children: []Symbol{
				{
					Name: "listUsers",
					Kind: "method",
					Annotations: []Annotation{
						{Name: "GetMapping", Value: ""},
					},
				},
				{
					Name: "createUser",
					Kind: "method",
					Annotations: []Annotation{
						{Name: "PostMapping", Value: "/create"},
					},
				},
			},
		},
	}

	got := resolveSpringEndpoints(symbols)
	if got[0].Children[0].Endpoint == nil || got[0].Children[0].Endpoint.Path != "/api/users" {
		t.Fatalf("expected GET endpoint on listUsers, got %#v", got[0].Children[0].Endpoint)
	}
	if got[0].Children[1].Endpoint == nil || got[0].Children[1].Endpoint.Method != HTTPMethodPost || got[0].Children[1].Endpoint.Path != "/api/users/create" {
		t.Fatalf("expected POST endpoint on createUser, got %#v", got[0].Children[1].Endpoint)
	}
}

func TestBuildHierarchyAssignsLeavesToSmallestContainer(t *testing.T) {
	t.Parallel()

	raws := []rawSym{
		{name: "Outer", kind: "class", line: 1, startByte: 0, endByte: 100, isContainer: true},
		{name: "Inner", kind: "class", line: 2, startByte: 10, endByte: 60, isContainer: true},
		{name: "helper", kind: "method", line: 3, startByte: 20, endByte: 30},
		{name: "topLevel", kind: "function", line: 120, startByte: 120, endByte: 140},
	}

	got := buildHierarchy(raws)
	if len(got) != 3 {
		t.Fatalf("buildHierarchy() len = %d, want 3", len(got))
	}
	if len(got[1].Children) != 1 || got[1].Children[0].Name != "helper" {
		t.Fatalf("expected helper to belong to smallest container, got %#v", got[1].Children)
	}
	if got[2].Name != "topLevel" {
		t.Fatalf("expected top-level function to remain top level, got %#v", got[2])
	}
}

func TestIsGitIgnored(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".gitignore"), "ignored.txt\nnested/\n")
	mustWriteFile(t, filepath.Join(root, "ignored.txt"), "ignored")
	mustWriteFile(t, filepath.Join(root, "kept.txt"), "kept")
	mustWriteFile(t, filepath.Join(root, "nested", "child.txt"), "child")

	cache := map[string]*ignore.GitIgnore{}

	if !isGitIgnored(filepath.Join(root, "ignored.txt"), root, cache) {
		t.Fatal("expected ignored.txt to be ignored")
	}
	if !isGitIgnored(filepath.Join(root, "nested", "child.txt"), root, cache) {
		t.Fatal("expected nested/child.txt to be ignored")
	}
	if isGitIgnored(filepath.Join(root, "kept.txt"), root, cache) {
		t.Fatal("did not expect kept.txt to be ignored")
	}
}
