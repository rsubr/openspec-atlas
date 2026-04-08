package internal

type HTTPMethod string

const (
	HTTPMethodGet     HTTPMethod = "GET"
	HTTPMethodPost    HTTPMethod = "POST"
	HTTPMethodPut     HTTPMethod = "PUT"
	HTTPMethodDelete  HTTPMethod = "DELETE"
	HTTPMethodPatch   HTTPMethod = "PATCH"
	HTTPMethodHead    HTTPMethod = "HEAD"
	HTTPMethodOptions HTTPMethod = "OPTIONS"
)

type HTTPMatchConfidence string

const (
	HTTPMatchExact HTTPMatchConfidence = "exact"
	HTTPMatchPath  HTTPMatchConfidence = "path"
	HTTPMatchFuzzy HTTPMatchConfidence = "fuzzy"
)

type ORMKind string

const (
	ORMSQL        ORMKind = "sql"
	ORMPrisma     ORMKind = "prisma"
	ORMTypeORM    ORMKind = "typeorm"
	ORMSQLAlchemy ORMKind = "sqlalchemy"
	ORMGORM       ORMKind = "gorm"
)

type MiddlewareType string

const (
	MiddlewareAuth         MiddlewareType = "auth"
	MiddlewareCORS         MiddlewareType = "cors"
	MiddlewareRateLimit    MiddlewareType = "rate-limit"
	MiddlewareValidation   MiddlewareType = "validation"
	MiddlewareLogging      MiddlewareType = "logging"
	MiddlewareErrorHandler MiddlewareType = "error-handler"
	MiddlewareCustom       MiddlewareType = "custom"
)

type UIFramework string

const (
	UIFrameworkReact   UIFramework = "react"
	UIFrameworkVue     UIFramework = "vue"
	UIFrameworkSvelte  UIFramework = "svelte"
	UIFrameworkAngular UIFramework = "angular"
)

// Annotation holds a decorator/attribute name and its optional string argument.
type Annotation struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// Endpoint is populated by language-specific post-processors when a symbol
// is identified as an HTTP handler (e.g. Spring Boot, NestJS, ASP.NET).
type Endpoint struct {
	Method HTTPMethod `json:"method"` // GET POST PUT DELETE PATCH
	Path   string     `json:"path"`   // fully resolved path including class-level prefix
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
	Files        []FileInfo       `json:"files"`
	EnvVars      []EnvVar         `json:"env_vars,omitempty"`
	HttpEdges    []HttpEdge       `json:"http_edges,omitempty"`
	SchemaModels []SchemaModel    `json:"schema_models,omitempty"`
	Middleware   []MiddlewareItem `json:"middleware,omitempty"`
	UIComponents []UIComponent    `json:"ui_components,omitempty"`
}

// EnvVar represents an environment variable referenced in source code or defined
// in a .env file.
type EnvVar struct {
	Name       string   `json:"name"`
	Files      []string `json:"files"`
	HasDefault bool     `json:"has_default"`
	Required   bool     `json:"required"`
}

// HttpEdge represents a detected HTTP call from a frontend file to a backend
// route handler, with a confidence level indicating how precisely the match was made.
type HttpEdge struct {
	CallerFile  string              `json:"caller_file"`
	CallerLine  int                 `json:"caller_line"`
	Method      HTTPMethod          `json:"method"`
	Path        string              `json:"path"`
	Confidence  HTTPMatchConfidence `json:"confidence"` // "exact", "path", "fuzzy"
	HandlerFile string              `json:"handler_file,omitempty"`
}

// SchemaField is a column or field within a database model.
type SchemaField struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Nullable bool   `json:"nullable,omitempty"`
}

// SchemaModel is a detected database model or table definition.
type SchemaModel struct {
	Name   string        `json:"name"`
	File   string        `json:"file"`
	Line   int           `json:"line"`
	ORM    ORMKind       `json:"orm"` // "sql", "prisma", "typeorm", "sqlalchemy", "gorm"
	Fields []SchemaField `json:"fields,omitempty"`
}

// MiddlewareItem is a detected middleware registration.
type MiddlewareItem struct {
	Name      string         `json:"name"`
	Type      MiddlewareType `json:"type"` // "auth", "cors", "rate-limit", "validation", "logging", "error-handler", "custom"
	Framework string         `json:"framework"`
	File      string         `json:"file"`
	Line      int            `json:"line"`
}

// UIComponent is a detected frontend UI component.
type UIComponent struct {
	Name      string      `json:"name"`
	Framework UIFramework `json:"framework"` // "react", "vue", "svelte", "angular"
	File      string      `json:"file"`
	Line      int         `json:"line"`
	Props     []string    `json:"props,omitempty"`
}

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
