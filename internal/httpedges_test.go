package internal

import "testing"

// TestExtractHTTPCalls verifies that supported client libraries are recognized
// and that external URLs are excluded from the inventory.
func TestExtractHTTPCalls(t *testing.T) {
	t.Parallel()

	src := []byte(`
fetch("/api/users", { method: "post" })
axios.get("/api/users/42")
fetch("https://example.com/external")
superagent.delete("http://localhost:3000/api/users/42")
`)

	got := extractHTTPCalls(src)
	if len(got) != 3 {
		t.Fatalf("len(extractHTTPCalls) = %d, want 3", len(got))
	}
	if got[0].method != HTTPMethodPost || got[0].path != "/api/users" {
		t.Fatalf("unexpected fetch call: %#v", got[0])
	}
	if got[1].method != HTTPMethodGet || got[1].path != "/api/users/42" {
		t.Fatalf("unexpected axios call: %#v", got[1])
	}
	if got[2].method != HTTPMethodDelete || got[2].path != "/api/users/42" {
		t.Fatalf("unexpected superagent call: %#v", got[2])
	}
}

// TestMatchRoute exercises exact and path-only HTTP edge matching against a
// normalized backend route index.
func TestMatchRoute(t *testing.T) {
	t.Parallel()

	routes := []backendRoute{
		{method: HTTPMethodGet, path: "/users/{id}", file: "api.go", normPath: normalizeHTTPPath("/users/{id}")},
	}

	file, confidence := matchRoute(httpCallInfo{
		method: HTTPMethodGet,
		path:   "/api/users/42",
	}, routes)
	if file != "api.go" || confidence != HTTPMatchExact {
		t.Fatalf("matchRoute exact = (%q, %q), want api.go/exact", file, confidence)
	}

	file, confidence = matchRoute(httpCallInfo{
		method: HTTPMethodPost,
		path:   "/api/users/42",
	}, routes)
	if file != "api.go" || confidence != HTTPMatchPath {
		t.Fatalf("matchRoute path = (%q, %q), want api.go/path", file, confidence)
	}
}
