package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	sitter "github.com/smacker/go-tree-sitter"
)

// --- Output schema ---

// Annotation holds a decorator/attribute name and its optional string argument.
type Annotation struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// Endpoint is populated by language-specific post-processors when a symbol
// is identified as an HTTP handler (e.g. Spring Boot, NestJS, ASP.NET).
type Endpoint struct {
	Method string `json:"method"` // GET POST PUT DELETE PATCH
	Path   string `json:"path"`   // fully resolved path including class-level prefix
}

type Symbol struct {
	Name        string       `json:"name"`
	Kind        string       `json:"kind"`
	Line        uint32       `json:"line"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Endpoint    *Endpoint    `json:"endpoint,omitempty"`
	Children    []Symbol     `json:"children,omitempty"`
}

type FileInfo struct {
	Path      string   `json:"path"`
	Language  string   `json:"language"`
	Namespace string   `json:"namespace,omitempty"`
	Symbols   []Symbol `json:"symbols"`
}

type Output struct {
	Files []FileInfo `json:"files"`
}

// --- Entry point ---

func main() {
	outputPath := flag.String("o", "structure.json", "output JSON file")
	allFiles := flag.Bool("all", false, "ignore .gitignore files and process everything")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: openspec-atlas [-o output.json] [-all] <dir> [dir ...]")
		flag.PrintDefaults()
	}
	flag.Parse()

	dirs := flag.Args()
	if len(dirs) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	var files []FileInfo
	for _, projectDir := range dirs {
		ignoreCache := map[string]*ignore.GitIgnore{}
		err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && info.Name() == ".git" {
				return filepath.SkipDir
			}
			if !*allFiles {
				if isGitIgnored(path, info.IsDir(), projectDir, ignoreCache) {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if info.IsDir() {
				return nil
			}
			config, ok := languageForFile(path)
			if !ok {
				return nil
			}
			rel, _ := filepath.Rel(projectDir, path)
			fmt.Printf("parsing [%s] %s\n", config.Name, filepath.Join(projectDir, rel))
			fi, parseErr := parseFile(path, config)
			if parseErr != nil {
				fmt.Fprintf(os.Stderr, "parse error in %s: %v\n", path, parseErr)
				return nil
			}
			fi.Path = filepath.Join(projectDir, rel)
			files = append(files, fi)
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "walk error in %s: %v\n", projectDir, err)
		}
	}

	data, _ := json.MarshalIndent(Output{Files: files}, "", "  ")
	if writeErr := os.WriteFile(*outputPath, data, 0644); writeErr != nil {
		fmt.Fprintln(os.Stderr, "write error:", writeErr)
		os.Exit(1)
	}
	fmt.Println(*outputPath, "generated")
}

// --- Parsing ---

func parseFile(path string, config *LanguageConfig) (FileInfo, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return FileInfo{}, err
	}

	parser := sitter.NewParser()
	parser.SetLanguage(config.Grammar)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return FileInfo{}, err
	}
	root := tree.RootNode()
	if root.HasError() {
		fmt.Fprintf(os.Stderr, "syntax errors in %s\n", path)
	}

	fi := FileInfo{Language: config.Name}
	fi.Namespace = extractNamespace(root, src, config)
	fi.Symbols = extractSymbols(root, src, config)
	if config.PostProcess != nil {
		fi.Symbols = config.PostProcess(fi.Symbols)
	}
	return fi, nil
}

// --- Namespace extraction ---

func extractNamespace(root *sitter.Node, src []byte, config *LanguageConfig) string {
	if config.NamespaceQuery == "" {
		return ""
	}
	q, err := sitter.NewQuery([]byte(config.NamespaceQuery), config.Grammar)
	if err != nil {
		return ""
	}
	cur := sitter.NewQueryCursor()
	cur.Exec(q, root)
	if m, ok := cur.NextMatch(); ok {
		for _, c := range m.Captures {
			if q.CaptureNameForId(c.Index) == "name" {
				return c.Node.Content(src)
			}
		}
	}
	return ""
}

// --- Symbol extraction ---

// rawSym is an intermediate flat representation before the hierarchy is built.
type rawSym struct {
	name        string
	kind        string
	line        uint32
	startByte   uint32
	endByte     uint32
	isContainer bool
	annotations []Annotation
}

func extractSymbols(root *sitter.Node, src []byte, config *LanguageConfig) []Symbol {
	// Pre-compile annotation queries once per file.
	var compiledAnnQueries []*sitter.Query
	for _, aq := range config.AnnotationQueries {
		q, err := sitter.NewQuery([]byte(aq.Pattern), config.Grammar)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid annotation query (%s): %v\n", config.Name, err)
			continue
		}
		compiledAnnQueries = append(compiledAnnQueries, q)
	}

	var raws []rawSym
	for _, sq := range config.SymbolQueries {
		q, err := sitter.NewQuery([]byte(sq.Pattern), config.Grammar)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid query (%s %s): %v\n", config.Name, sq.Kind, err)
			continue
		}
		cur := sitter.NewQueryCursor()
		cur.Exec(q, root)
		for {
			m, ok := cur.NextMatch()
			if !ok {
				break
			}
			var nameNode, declNode *sitter.Node
			for _, c := range m.Captures {
				switch q.CaptureNameForId(c.Index) {
				case "name":
					nameNode = c.Node
				case "decl":
					declNode = c.Node
				}
			}
			if nameNode == nil {
				continue
			}
			rangeNode := declNode
			if rangeNode == nil {
				rangeNode = nameNode.Parent()
			}
			if rangeNode == nil {
				rangeNode = nameNode
			}
			var annotations []Annotation
			if declNode != nil {
				annotations = extractAnnotationsFromDecl(declNode, src, config, compiledAnnQueries)
			}
			raws = append(raws, rawSym{
				name:        nameNode.Content(src),
				kind:        sq.Kind,
				line:        nameNode.StartPoint().Row + 1,
				startByte:   rangeNode.StartByte(),
				endByte:     rangeNode.EndByte(),
				isContainer: sq.IsContainer,
				annotations: annotations,
			})
		}
	}
	return buildHierarchy(raws)
}

// extractAnnotationsFromDecl finds annotation nodes scoped to the given
// declaration and runs the pre-compiled annotation queries against them.
//
// Scoping strategy (avoids picking up nested annotations from child declarations):
//
//	AnnotationContainerType = "modifiers" / "attribute_list"
//	    → find the named container as a direct child of declNode and query it.
//	      Multiple container siblings (e.g. C# [Attr1][Attr2]) are all queried.
//
//	AnnotationContainerType = "" + AnnotationNodeTypes set
//	    → collect annotation nodes (e.g. "decorator") that are direct children
//	      of declNode and query each one.
//
//	AnnotationContainerType = "parent" + AnnotationNodeTypes set
//	    → same as above but from the parent node (Python: decorated_definition
//	      wraps function_definition; decorators live on the parent).
func extractAnnotationsFromDecl(declNode *sitter.Node, src []byte, config *LanguageConfig, compiledAnnQueries []*sitter.Query) []Annotation {
	if len(compiledAnnQueries) == 0 {
		return nil
	}

	var targets []*sitter.Node

	switch config.AnnotationContainerType {
	case "parent":
		if parent := declNode.Parent(); parent != nil {
			targets = directChildrenOfType(parent, config.AnnotationNodeTypes)
		}
	case "":
		if len(config.AnnotationNodeTypes) > 0 {
			// Decorators are direct children of the declaration node itself.
			targets = directChildrenOfType(declNode, config.AnnotationNodeTypes)
		}
	default:
		// Collect ALL container children (handles multiple [Attr] lists in C#).
		for i := 0; i < int(declNode.ChildCount()); i++ {
			if declNode.Child(i).Type() == config.AnnotationContainerType {
				targets = append(targets, declNode.Child(i))
			}
		}
	}

	if len(targets) == 0 {
		return nil
	}

	seen := map[string]bool{}
	var annotations []Annotation
	for _, target := range targets {
		for _, q := range compiledAnnQueries {
			cur := sitter.NewQueryCursor()
			cur.Exec(q, target)
			for {
				m, ok := cur.NextMatch()
				if !ok {
					break
				}
				var name, value string
				for _, c := range m.Captures {
					switch q.CaptureNameForId(c.Index) {
					case "name":
						name = c.Node.Content(src)
					case "value":
						value = strings.Trim(c.Node.Content(src), `"'`)
					}
				}
				if name == "" {
					continue
				}
				key := name + ":" + value
				if !seen[key] {
					seen[key] = true
					annotations = append(annotations, Annotation{Name: name, Value: value})
				}
			}
		}
	}
	return annotations
}

// directChildrenOfType returns direct children of node whose type is in the set.
func directChildrenOfType(node *sitter.Node, types []string) []*sitter.Node {
	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}
	var result []*sitter.Node
	for i := 0; i < int(node.ChildCount()); i++ {
		if child := node.Child(i); typeSet[child.Type()] {
			result = append(result, child)
		}
	}
	return result
}

// --- Hierarchy ---

func buildHierarchy(raws []rawSym) []Symbol {
	var containers, leaves []rawSym
	for _, r := range raws {
		if r.isContainer {
			containers = append(containers, r)
		} else {
			leaves = append(leaves, r)
		}
	}

	sort.Slice(containers, func(i, j int) bool {
		return containers[i].startByte < containers[j].startByte
	})

	containerSyms := make([]Symbol, len(containers))
	for i, c := range containers {
		containerSyms[i] = Symbol{Name: c.name, Kind: c.kind, Line: c.line, Annotations: c.annotations}
	}

	var topLevel []Symbol
	for _, leaf := range leaves {
		bestIdx := -1
		bestSize := ^uint32(0)
		for i, c := range containers {
			if leaf.startByte >= c.startByte && leaf.endByte <= c.endByte {
				if size := c.endByte - c.startByte; size < bestSize {
					bestSize = size
					bestIdx = i
				}
			}
		}
		sym := Symbol{Name: leaf.name, Kind: leaf.kind, Line: leaf.line, Annotations: leaf.annotations}
		if bestIdx >= 0 {
			containerSyms[bestIdx].Children = append(containerSyms[bestIdx].Children, sym)
		} else {
			topLevel = append(topLevel, sym)
		}
	}

	return append(containerSyms, topLevel...)
}

// --- Spring Boot endpoint resolution ---

// springHTTPMappings maps Spring annotation names to HTTP verbs.
var springHTTPMappings = map[string]string{
	"GetMapping":    "GET",
	"PostMapping":   "POST",
	"PutMapping":    "PUT",
	"DeleteMapping": "DELETE",
	"PatchMapping":  "PATCH",
}

// resolveSpringEndpoints is the PostProcess function for Java. It combines
// the class-level @RequestMapping base path with each method's HTTP mapping
// annotation to produce a fully-resolved Endpoint on each handler method.
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

// --- Gitignore ---

func isGitIgnored(path string, isDir bool, root string, cache map[string]*ignore.GitIgnore) bool {
	_ = isDir
	dir := filepath.Dir(path)
	for {
		if _, loaded := cache[dir]; !loaded {
			m, err := ignore.CompileIgnoreFile(filepath.Join(dir, ".gitignore"))
			if err != nil {
				cache[dir] = nil
			} else {
				cache[dir] = m
			}
		}
		if m := cache[dir]; m != nil {
			rel, _ := filepath.Rel(dir, path)
			if m.MatchesPath(rel) {
				return true
			}
		}
		if dir == root {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}
