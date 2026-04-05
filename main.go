package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	ignore "github.com/sabhiram/go-gitignore"
	sitter "github.com/smacker/go-tree-sitter"
)

// --- Output schema ---

type Symbol struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Line     uint32   `json:"line"`
	Children []Symbol `json:"children,omitempty"`
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
			// Always skip .git
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
// @decl captures the full declaration node so byte ranges are used for nesting.
type rawSym struct {
	name        string
	kind        string
	line        uint32
	startByte   uint32
	endByte     uint32
	isContainer bool // containers (classes, structs, etc.) can own child symbols
}

func extractSymbols(root *sitter.Node, src []byte, config *LanguageConfig) []Symbol {
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
			// Each query pattern must capture @name (the symbol identifier)
			// and @decl (the full declaration node, for byte-range hierarchy).
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
			// Fall back to parent of @name if @decl wasn't captured
			rangeNode := declNode
			if rangeNode == nil {
				rangeNode = nameNode.Parent()
			}
			if rangeNode == nil {
				rangeNode = nameNode
			}
			raws = append(raws, rawSym{
				name:        nameNode.Content(src),
				kind:        sq.Kind,
				line:        nameNode.StartPoint().Row + 1,
				startByte:   rangeNode.StartByte(),
				endByte:     rangeNode.EndByte(),
				isContainer: sq.IsContainer,
			})
		}
	}
	return buildHierarchy(raws)
}

// buildHierarchy organises flat rawSyms into a two-level tree:
// containers (classes, structs, traits …) own any leaf symbol (method,
// function …) whose byte range falls inside them. Leaves with no
// enclosing container are returned as top-level symbols.
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
		containerSyms[i] = Symbol{Name: c.name, Kind: c.kind, Line: c.line}
	}

	var topLevel []Symbol
	for _, leaf := range leaves {
		// Assign to the innermost (smallest) container that contains this leaf.
		bestIdx := -1
		bestSize := ^uint32(0)
		for i, c := range containers {
			if leaf.startByte >= c.startByte && leaf.endByte <= c.endByte {
				size := c.endByte - c.startByte
				if size < bestSize {
					bestSize = size
					bestIdx = i
				}
			}
		}
		sym := Symbol{Name: leaf.name, Kind: leaf.kind, Line: leaf.line}
		if bestIdx >= 0 {
			containerSyms[bestIdx].Children = append(containerSyms[bestIdx].Children, sym)
		} else {
			topLevel = append(topLevel, sym)
		}
	}

	return append(containerSyms, topLevel...)
}

// --- Gitignore ---

// isGitIgnored checks path against all .gitignore files from its parent
// directory up to root. Results are cached so each .gitignore is read at most
// once. Starting from filepath.Dir(path) — rather than path itself — means
// directories are checked against their parent's rules before being entered,
// enabling filepath.SkipDir to fire without walking into them first.
func isGitIgnored(path string, isDir bool, root string, cache map[string]*ignore.GitIgnore) bool {
	_ = isDir // kept in signature for call-site clarity; logic applies to both
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
