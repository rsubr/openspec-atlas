package internals

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
