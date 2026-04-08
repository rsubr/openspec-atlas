package internal

import "strings"

// springHTTPMappings maps Spring annotation names to HTTP verbs.
var springHTTPMappings = map[string]string{
	"GetMapping":    "GET",
	"PostMapping":   "POST",
	"PutMapping":    "PUT",
	"DeleteMapping": "DELETE",
	"PatchMapping":  "PATCH",
}

// resolveSpringEndpoints is the PostProcess function for Java and Kotlin.
// It combines the class-level @RequestMapping base path with each method's
// HTTP mapping annotation to produce a fully-resolved Endpoint.
func resolveSpringEndpoints(symbols []Symbol) []Symbol {
	for i := range symbols {
		basePath := annotationValue(symbols[i].Annotations, "RequestMapping")
		for j := range symbols[i].Children {
			child := &symbols[i].Children[j]
			for ann, method := range springHTTPMappings {
				if hasAnnotation(child.Annotations, ann) {
					child.Endpoint = &Endpoint{
						Method: method,
						Path:   joinPaths(basePath, annotationValue(child.Annotations, ann)),
					}
					break
				}
			}
		}
	}
	return symbols
}

func annotationValue(annotations []Annotation, name string) string {
	for _, a := range annotations {
		if a.Name == name {
			return a.Value
		}
	}
	return ""
}

func hasAnnotation(annotations []Annotation, name string) bool {
	for _, a := range annotations {
		if a.Name == name {
			return true
		}
	}
	return false
}

func joinPaths(base, path string) string {
	base = strings.TrimRight(base, "/")
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if base == "" {
		return path
	}
	return base + path
}
