package internals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

// SymbolQuery pairs a tree-sitter S-expression pattern with a kind label.
// Every pattern MUST capture:
//
//	@name — the identifier node (for display name and line number)
//	@decl — the full declaration node (for byte-range hierarchy)
type SymbolQuery struct {
	Pattern     string
	Kind        string
	IsContainer bool // true → symbol can own children (class, struct, trait …)
}

// AnnotationQuery is a tree-sitter pattern run against an annotation/decorator
// node (or its container). It must capture @name; @value is optional and should
// capture the first string argument (e.g. the route path).
type AnnotationQuery struct {
	Pattern string
}

// PostProcessFn is an optional per-language hook called after symbols are
// extracted. Used for derived data that requires cross-symbol context, such
// as resolving fully-qualified HTTP endpoint paths in Spring Boot.
type PostProcessFn func([]Symbol) []Symbol

type compiledSymbolQuery struct {
	Kind        string
	IsContainer bool
	Query       *sitter.Query
}

// LanguageConfig bundles everything openspec-atlas needs for one language.
type LanguageConfig struct {
	Name           string
	Extensions     []string
	Grammar        *sitter.Language
	NamespaceQuery string // optional; must capture @name

	SymbolQueries []SymbolQuery

	// Annotation extraction — see extractAnnotationsFromDecl in annotations.go
	// for the three scoping strategies these fields control.
	AnnotationContainerType string   // "modifiers", "attribute_list", "parent", or ""
	AnnotationNodeTypes     []string // for "" and "parent" modes: direct child node types
	AnnotationQueries       []AnnotationQuery

	PostProcess PostProcessFn // optional; called after symbol extraction

	compiledNamespaceQuery *sitter.Query
	compiledSymbolQueries  []compiledSymbolQuery
	compiledAnnQueries     []*sitter.Query
	compileOnce            sync.Once
}

var (
	registry      []*LanguageConfig
	languageByExt map[string]*LanguageConfig
)

func prepareRegistry() {
	languageByExt = make(map[string]*LanguageConfig)

	for _, config := range registry {
		for _, ext := range config.Extensions {
			languageByExt[strings.ToLower(ext)] = config
		}
	}
}

func languageForFile(path string) (*LanguageConfig, bool) {
	config, ok := languageByExt[strings.ToLower(filepath.Ext(path))]
	return config, ok
}

func (c *LanguageConfig) ensureCompiled() {
	c.compileOnce.Do(func() {
		if c.NamespaceQuery != "" {
			q, err := sitter.NewQuery([]byte(c.NamespaceQuery), c.Grammar)
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid namespace query (%s): %v\n", c.Name, err)
			} else {
				c.compiledNamespaceQuery = q
			}
		}

		for _, sq := range c.SymbolQueries {
			q, err := sitter.NewQuery([]byte(sq.Pattern), c.Grammar)
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid query (%s %s): %v\n", c.Name, sq.Kind, err)
				continue
			}
			c.compiledSymbolQueries = append(c.compiledSymbolQueries, compiledSymbolQuery{
				Kind:        sq.Kind,
				IsContainer: sq.IsContainer,
				Query:       q,
			})
		}

		for _, aq := range c.AnnotationQueries {
			q, err := sitter.NewQuery([]byte(aq.Pattern), c.Grammar)
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid annotation query (%s): %v\n", c.Name, err)
				continue
			}
			c.compiledAnnQueries = append(c.compiledAnnQueries, q)
		}
	})
}
