package internal

import (
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
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

// init declares the full language registry and prepares the extension lookup
// table used by the scanner.
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
			// Annotations live inside a modifiers node that is a direct child of
			// the declaration. Querying the modifiers node (not the whole declaration)
			// prevents picking up annotations from nested method declarations.
			AnnotationContainerType: "modifiers",
			AnnotationQueries: []AnnotationQuery{
				// @RestController, @Service, @Override …
				{`(marker_annotation name: (identifier) @name)`},
				// @RequestMapping("/api/users"), @GetMapping("/{id}") …
				{`(annotation name: (identifier) @name arguments: (annotation_argument_list (string_literal) @value))`},
			},
			// Resolves Spring Boot HTTP endpoints from @GetMapping / @PostMapping etc.
			PostProcess: resolveSpringEndpoints,
		},
		{
			Name:           "go",
			Extensions:     []string{".go"},
			Grammar:        golang.GetLanguage(),
			NamespaceQuery: `(package_clause (package_identifier) @name)`,
			SymbolQueries: []SymbolQuery{
				{`(type_declaration (type_spec name: (type_identifier) @name type: (struct_type))) @decl`, "struct", true},
				{`(type_declaration (type_spec name: (type_identifier) @name type: (interface_type))) @decl`, "interface", true},
				{`(function_declaration name: (identifier) @name) @decl`, "function", false},
				{`(method_declaration name: (field_identifier) @name) @decl`, "method", false},
			},
			// Go has no annotation syntax.
		},
		{
			Name:           "python",
			Extensions:     []string{".py"},
			Grammar:        python.GetLanguage(),
			NamespaceQuery: ``,
			SymbolQueries: []SymbolQuery{
				{`(class_definition name: (identifier) @name) @decl`, "class", true},
				{`(function_definition name: (identifier) @name) @decl`, "function", false},
			},
			// Python decorators appear on a decorated_definition that WRAPS the
			// function/class. The @decl captures the inner function_definition, so
			// we look at its parent for decorator children.
			AnnotationContainerType: "parent",
			AnnotationNodeTypes:     []string{"decorator"},
			AnnotationQueries: []AnnotationQuery{
				// @login_required
				{`(decorator (identifier) @name)`},
				// @app.route("/users") — capture the attribute (e.g. "app.route") as name
				{`(decorator (call function: (attribute) @name arguments: (argument_list (string) @value)))`},
				// @route("/users")
				{`(decorator (call function: (identifier) @name arguments: (argument_list (string) @value)))`},
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
			// Decorators (@Controller, @Get, @Injectable …) are direct children of
			// the declaration node in TypeScript's grammar.
			AnnotationContainerType: "",
			AnnotationNodeTypes:     []string{"decorator"},
			AnnotationQueries: []AnnotationQuery{
				// @Injectable()
				{`(decorator (identifier) @name)`},
				// @Controller('/users'), @Get('/:id')
				{`(decorator (call_expression function: (identifier) @name arguments: (arguments (string) @value)))`},
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
			AnnotationContainerType: "",
			AnnotationNodeTypes:     []string{"decorator"},
			AnnotationQueries: []AnnotationQuery{
				{`(decorator (identifier) @name)`},
				{`(decorator (call_expression function: (identifier) @name arguments: (arguments (string) @value)))`},
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
			AnnotationContainerType: "",
			AnnotationNodeTypes:     []string{"decorator"},
			AnnotationQueries: []AnnotationQuery{
				{`(decorator (identifier) @name)`},
				{`(decorator (call_expression function: (identifier) @name arguments: (arguments (string) @value)))`},
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
			// Kotlin annotations are in a modifiers node, same pattern as Java.
			AnnotationContainerType: "modifiers",
			AnnotationQueries: []AnnotationQuery{
				// @RestController, @Service …
				{`(annotation (user_type (type_identifier) @name))`},
				// @GetMapping("/users")
				{`(annotation (user_type (type_identifier) @name) (value_arguments (value_argument (string_literal) @value)))`},
			},
			PostProcess: resolveSpringEndpoints,
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
			// C# attributes sit in attribute_list nodes that are direct children of
			// the declaration. Multiple [Attr] blocks are all collected.
			AnnotationContainerType: "attribute_list",
			AnnotationQueries: []AnnotationQuery{
				// [Authorize], [ApiController]
				{`(attribute name: (identifier) @name)`},
				// [HttpGet("/users/{id}")]
				{`(attribute name: (identifier) @name (attribute_argument_clause (string_literal) @value))`},
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
				{`(impl_item type: (type_identifier) @name) @decl`, "impl", true},
				{`(function_item name: (identifier) @name) @decl`, "function", false},
			},
			// Rust proc-macro attributes (#[derive(...)], #[get(...)]) are
			// attribute_item nodes that precede the declaration as siblings,
			// not as children. Not currently extracted.
		},
		{
			Name:           "c",
			Extensions:     []string{".c", ".h"},
			Grammar:        c.GetLanguage(),
			NamespaceQuery: ``,
			SymbolQueries: []SymbolQuery{
				{`(struct_specifier name: (type_identifier) @name) @decl`, "struct", true},
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
			// PHP 8+ attributes (#[Route(...)]) use attribute_list. PHP <8 uses
			// docblock annotations which are comments — not currently extracted.
			AnnotationContainerType: "attribute_list",
			AnnotationQueries: []AnnotationQuery{
				{`(attribute name: (name) @name)`},
				{`(attribute name: (name) @name arguments: (arguments (string) @value))`},
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
			// Ruby has no annotation syntax; metadata is expressed as DSL method
			// calls (before_action, get, post …) which are not extracted here.
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
		{
			// Vue Single File Components: extract the <script> block and parse
			// it with the TypeScript grammar (a superset of JavaScript).
			Name:         "vue",
			Extensions:   []string{".vue"},
			Grammar:      typescript.GetLanguage(),
			SrcTransform: extractVueScript,
			SymbolQueries: []SymbolQuery{
				{`(class_declaration name: (type_identifier) @name) @decl`, "class", true},
				{`(interface_declaration name: (type_identifier) @name) @decl`, "interface", true},
				{`(function_declaration name: (identifier) @name) @decl`, "function", false},
				{`(method_definition name: (property_identifier) @name) @decl`, "method", false},
			},
			AnnotationContainerType: "",
			AnnotationNodeTypes:     []string{"decorator"},
			AnnotationQueries: []AnnotationQuery{
				{`(decorator (identifier) @name)`},
				{`(decorator (call_expression function: (identifier) @name arguments: (arguments (string) @value)))`},
			},
		},
	}
	prepareRegistry()
}
