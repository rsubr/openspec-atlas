package main

import (
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/lua"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/swift"
	tsx "github.com/smacker/go-tree-sitter/typescript/tsx"
	typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
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

// LanguageConfig bundles everything openspec-atlas needs for one language.
type LanguageConfig struct {
	Name           string
	Extensions     []string
	Grammar        *sitter.Language
	NamespaceQuery string // optional; must capture @name
	SymbolQueries  []SymbolQuery
}

var registry []*LanguageConfig

func init() {
	registry = []*LanguageConfig{
		{
			Name:           "java",
			Extensions:     []string{".java"},
			Grammar:        java.GetLanguage(),
			NamespaceQuery: `(package_declaration (_) @name)`,
			SymbolQueries: []SymbolQuery{
				{`(class_declaration name: (identifier) @name) @decl`, "class", true},
				{`(interface_declaration name: (identifier) @name) @decl`, "interface", true},
				{`(enum_declaration name: (identifier) @name) @decl`, "enum", true},
				{`(method_declaration name: (identifier) @name) @decl`, "method", false},
				{`(constructor_declaration name: (identifier) @name) @decl`, "constructor", false},
			},
		},
		{
			Name:           "go",
			Extensions:     []string{".go"},
			Grammar:        golang.GetLanguage(),
			NamespaceQuery: `(package_clause (package_identifier) @name)`,
			SymbolQueries: []SymbolQuery{
				// Distinguish struct/interface from generic type aliases
				{`(type_declaration (type_spec name: (type_identifier) @name type: (struct_type))) @decl`, "struct", true},
				{`(type_declaration (type_spec name: (type_identifier) @name type: (interface_type))) @decl`, "interface", true},
				// Go methods are not nested inside the struct; top-level only
				{`(function_declaration name: (identifier) @name) @decl`, "function", false},
				{`(method_declaration name: (field_identifier) @name) @decl`, "method", false},
			},
		},
		{
			Name:           "python",
			Extensions:     []string{".py"},
			Grammar:        python.GetLanguage(),
			NamespaceQuery: ``,
			SymbolQueries: []SymbolQuery{
				{`(class_definition name: (identifier) @name) @decl`, "class", true},
				// function_definition matches both top-level and methods; hierarchy sorts them out
				{`(function_definition name: (identifier) @name) @decl`, "function", false},
			},
		},
		{
			Name:           "typescript",
			Extensions:     []string{".ts"},
			Grammar:        typescript.GetLanguage(),
			NamespaceQuery: `(internal_module name: (identifier) @name)`,
			SymbolQueries: []SymbolQuery{
				{`(class_declaration name: (type_identifier) @name) @decl`, "class", true},
				{`(interface_declaration name: (type_identifier) @name) @decl`, "interface", true},
				{`(type_alias_declaration name: (type_identifier) @name) @decl`, "type", false},
				{`(enum_declaration name: (identifier) @name) @decl`, "enum", true},
				{`(function_declaration name: (identifier) @name) @decl`, "function", false},
				{`(method_definition name: (property_identifier) @name) @decl`, "method", false},
			},
		},
		{
			Name:           "tsx",
			Extensions:     []string{".tsx"},
			Grammar:        tsx.GetLanguage(),
			NamespaceQuery: ``,
			SymbolQueries: []SymbolQuery{
				{`(class_declaration name: (type_identifier) @name) @decl`, "class", true},
				{`(interface_declaration name: (type_identifier) @name) @decl`, "interface", true},
				{`(function_declaration name: (identifier) @name) @decl`, "function", false},
				{`(method_definition name: (property_identifier) @name) @decl`, "method", false},
			},
		},
		{
			Name:           "javascript",
			Extensions:     []string{".js", ".mjs", ".cjs"},
			Grammar:        javascript.GetLanguage(),
			NamespaceQuery: ``,
			SymbolQueries: []SymbolQuery{
				{`(class_declaration name: (identifier) @name) @decl`, "class", true},
				{`(function_declaration name: (identifier) @name) @decl`, "function", false},
				{`(method_definition name: (property_identifier) @name) @decl`, "method", false},
			},
		},
		{
			Name:           "rust",
			Extensions:     []string{".rs"},
			Grammar:        rust.GetLanguage(),
			NamespaceQuery: `(mod_item name: (identifier) @name)`,
			SymbolQueries: []SymbolQuery{
				{`(struct_item name: (type_identifier) @name) @decl`, "struct", true},
				{`(enum_item name: (type_identifier) @name) @decl`, "enum", true},
				{`(trait_item name: (type_identifier) @name) @decl`, "trait", true},
				// impl blocks: use the type name as the symbol name
				{`(impl_item type: (type_identifier) @name) @decl`, "impl", true},
				{`(function_item name: (identifier) @name) @decl`, "function", false},
			},
		},
		{
			Name:           "c",
			Extensions:     []string{".c", ".h"},
			Grammar:        c.GetLanguage(),
			NamespaceQuery: ``,
			SymbolQueries: []SymbolQuery{
				{`(struct_specifier name: (type_identifier) @name) @decl`, "struct", true},
				// C function: declarator is nested two levels deep
				{`(function_definition declarator: (function_declarator declarator: (identifier) @name)) @decl`, "function", false},
			},
		},
		{
			Name:           "cpp",
			Extensions:     []string{".cpp", ".cc", ".cxx", ".hpp"},
			Grammar:        cpp.GetLanguage(),
			NamespaceQuery: `(namespace_definition name: (identifier) @name)`,
			SymbolQueries: []SymbolQuery{
				{`(class_specifier name: (type_identifier) @name) @decl`, "class", true},
				{`(struct_specifier name: (type_identifier) @name) @decl`, "struct", true},
				{`(function_definition declarator: (function_declarator declarator: (identifier) @name)) @decl`, "function", false},
			},
		},
		{
			Name:           "csharp",
			Extensions:     []string{".cs"},
			Grammar:        csharp.GetLanguage(),
			NamespaceQuery: `(namespace_declaration name: (_) @name)`,
			SymbolQueries: []SymbolQuery{
				{`(class_declaration name: (identifier) @name) @decl`, "class", true},
				{`(interface_declaration name: (identifier) @name) @decl`, "interface", true},
				{`(struct_declaration name: (identifier) @name) @decl`, "struct", true},
				{`(enum_declaration name: (identifier) @name) @decl`, "enum", true},
				{`(method_declaration name: (identifier) @name) @decl`, "method", false},
			},
		},
		{
			Name:           "ruby",
			Extensions:     []string{".rb"},
			Grammar:        ruby.GetLanguage(),
			NamespaceQuery: `(module name: (constant) @name)`,
			SymbolQueries: []SymbolQuery{
				{`(class name: (constant) @name) @decl`, "class", true},
				{`(method name: (identifier) @name) @decl`, "method", false},
				{`(singleton_method name: (identifier) @name) @decl`, "method", false},
			},
		},
		{
			Name:           "kotlin",
			Extensions:     []string{".kt", ".kts"},
			Grammar:        kotlin.GetLanguage(),
			NamespaceQuery: `(package_header (identifier) @name)`,
			SymbolQueries: []SymbolQuery{
				{`(class_declaration name: (simple_identifier) @name) @decl`, "class", true},
				{`(object_declaration name: (simple_identifier) @name) @decl`, "object", true},
				{`(function_declaration name: (simple_identifier) @name) @decl`, "function", false},
			},
		},
		{
			Name:           "scala",
			Extensions:     []string{".scala"},
			Grammar:        scala.GetLanguage(),
			NamespaceQuery: `(package_clause (package_identifier) @name)`,
			SymbolQueries: []SymbolQuery{
				{`(class_definition name: (identifier) @name) @decl`, "class", true},
				{`(trait_definition name: (identifier) @name) @decl`, "trait", true},
				{`(object_definition name: (identifier) @name) @decl`, "object", true},
				{`(function_definition name: (identifier) @name) @decl`, "function", false},
			},
		},
		{
			Name:           "swift",
			Extensions:     []string{".swift"},
			Grammar:        swift.GetLanguage(),
			NamespaceQuery: ``,
			SymbolQueries: []SymbolQuery{
				{`(class_declaration name: (type_identifier) @name) @decl`, "class", true},
				{`(struct_declaration name: (type_identifier) @name) @decl`, "struct", true},
				{`(protocol_declaration name: (type_identifier) @name) @decl`, "protocol", true},
				{`(enum_declaration name: (type_identifier) @name) @decl`, "enum", true},
				{`(function_declaration name: (simple_identifier) @name) @decl`, "function", false},
			},
		},
		{
			Name:           "php",
			Extensions:     []string{".php"},
			Grammar:        php.GetLanguage(),
			NamespaceQuery: `(namespace_definition name: (namespace_name) @name)`,
			SymbolQueries: []SymbolQuery{
				{`(class_declaration name: (name) @name) @decl`, "class", true},
				{`(interface_declaration name: (name) @name) @decl`, "interface", true},
				{`(trait_declaration name: (name) @name) @decl`, "trait", true},
				{`(method_declaration name: (name) @name) @decl`, "method", false},
				{`(function_definition name: (name) @name) @decl`, "function", false},
			},
		},
		{
			Name:           "lua",
			Extensions:     []string{".lua"},
			Grammar:        lua.GetLanguage(),
			NamespaceQuery: ``,
			SymbolQueries: []SymbolQuery{
				{`(function_declaration name: (identifier) @name) @decl`, "function", false},
				{`(local_function name: (identifier) @name) @decl`, "function", false},
			},
		},
		{
			Name:           "bash",
			Extensions:     []string{".sh", ".bash"},
			Grammar:        bash.GetLanguage(),
			NamespaceQuery: ``,
			SymbolQueries: []SymbolQuery{
				{`(function_definition name: (word) @name) @decl`, "function", false},
			},
		},
	}
}

func languageForFile(path string) (*LanguageConfig, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	for _, config := range registry {
		for _, e := range config.Extensions {
			if e == ext {
				return config, true
			}
		}
	}
	return nil, false
}
