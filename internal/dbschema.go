package internal

import (
	"regexp"
	"strings"
)

// ---- Prisma ----------------------------------------------------------------

var (
	prismaModelRe = regexp.MustCompile(`(?m)^model\s+(\w+)\s*\{`)
	prismaFieldRe = regexp.MustCompile(`^\s+(\w+)\s+(\w+)(\?)?\s`)
)

func newSchemaModel(name, file string, line int, orm ORMKind) SchemaModel {
	return SchemaModel{
		Name: name,
		File: file,
		Line: line,
		ORM:  orm,
	}
}

func extractPrismaModels(src []byte, file string) []SchemaModel {
	var models []SchemaModel
	lines := splitLines(src)
	for i, line := range lines {
		m := prismaModelRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		model := newSchemaModel(m[1], file, i+1, ORMPrisma)
		// Collect fields until closing brace
		for j := i + 1; j < len(lines); j++ {
			fl := lines[j]
			if strings.TrimSpace(fl) == "}" {
				break
			}
			fm := prismaFieldRe.FindStringSubmatch(fl)
			if fm == nil {
				continue
			}
			name, typ, nullable := fm[1], fm[2], fm[3] == "?"
			// skip Prisma decorators and relation keywords
			if strings.HasPrefix(name, "@") || name == "@@" {
				continue
			}
			model.Fields = append(model.Fields, SchemaField{
				Name:     name,
				Type:     typ,
				Nullable: nullable,
			})
		}
		models = append(models, model)
	}
	return models
}

// ---- SQL -------------------------------------------------------------------

var (
	sqlCreateTableRe = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["` + "`" + `]?(\w+)["` + "`" + `]?\s*\(`)
	sqlColumnRe      = regexp.MustCompile(`(?i)^\s*["` + "`" + `]?(\w+)["` + "`" + `]?\s+(\w+)`)
	sqlNotNullRe     = regexp.MustCompile(`(?i)NOT\s+NULL`)
)

func extractSQLModels(src []byte, file string) []SchemaModel {
	var models []SchemaModel
	lines := splitLines(src)
	for i, line := range lines {
		m := sqlCreateTableRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		model := newSchemaModel(m[1], file, i+1, ORMSQL)
		model.Fields = extractSQLFields(lines, i)
		models = append(models, model)
	}
	return models
}

func extractSQLFields(lines []string, start int) []SchemaField {
	var fields []SchemaField
	depth := strings.Count(lines[start], "(") - strings.Count(lines[start], ")")
	for j := start + 1; j < len(lines) && depth > 0; j++ {
		line := lines[j]
		depth += strings.Count(line, "(") - strings.Count(line, ")")
		field, ok := parseSQLField(line)
		if ok {
			fields = append(fields, field)
		}
	}
	return fields
}

func parseSQLField(line string) (SchemaField, bool) {
	match := sqlColumnRe.FindStringSubmatch(line)
	if match == nil {
		return SchemaField{}, false
	}
	colName := match[1]
	if isSQLConstraintKeyword(colName) {
		return SchemaField{}, false
	}
	return SchemaField{
		Name:     colName,
		Type:     strings.ToLower(match[2]),
		Nullable: !sqlNotNullRe.MatchString(line),
	}, true
}

func isSQLConstraintKeyword(name string) bool {
	switch strings.ToUpper(name) {
	case "PRIMARY", "UNIQUE", "FOREIGN", "INDEX", "KEY", "CHECK", "CONSTRAINT":
		return true
	default:
		return false
	}
}

// ---- SQLAlchemy ------------------------------------------------------------

var (
	// class User(Base): or class User(db.Model):
	saClassRe  = regexp.MustCompile(`^class\s+(\w+)\s*\(\s*(?:\w+\.)*(?:Base|Model)\s*\)`)
	saColumnRe = regexp.MustCompile(`(\w+)\s*=\s*(?:db\.)?(?:mapped_column|Column)\s*\(([^)]+)\)`)
	saTypeRe   = regexp.MustCompile(`(?i)(?:db\.)?(\w+(?:\(\d+\))?)\s*(?:,|$)`)
	saNullRe   = regexp.MustCompile(`(?i)nullable\s*=\s*(True|False)`)
)

func extractSQLAlchemyModels(src []byte, file string) []SchemaModel {
	var models []SchemaModel
	lines := splitLines(src)
	var current *SchemaModel
	indent := ""

	for i, line := range lines {
		if m := saClassRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				models = append(models, *current)
			}
			model := newSchemaModel(m[1], file, i+1, ORMSQLAlchemy)
			current = &model
			indent = classBodyIndent(line)
			continue
		}
		if current == nil {
			continue
		}
		if endsPythonClass(line, indent) {
			models = append(models, *current)
			current = nil
			continue
		}
		if field, ok := parseSQLAlchemyField(line); ok {
			current.Fields = append(current.Fields, field)
		}
	}
	if current != nil {
		models = append(models, *current)
	}
	return models
}

func classBodyIndent(line string) string {
	return leadingSpaces(line) + " "
}

func endsPythonClass(line, indent string) bool {
	return len(line) > 0 &&
		!strings.HasPrefix(line, indent) &&
		!strings.HasPrefix(strings.TrimSpace(line), "#")
}

func parseSQLAlchemyField(line string) (SchemaField, bool) {
	match := saColumnRe.FindStringSubmatch(line)
	if match == nil {
		return SchemaField{}, false
	}
	colType := ""
	if tm := saTypeRe.FindStringSubmatch(match[2]); tm != nil {
		colType = tm[1]
	}
	nullable := true
	if nm := saNullRe.FindStringSubmatch(match[2]); nm != nil {
		nullable = strings.EqualFold(nm[1], "True")
	}
	return SchemaField{
		Name:     match[1],
		Type:     strings.ToLower(colType),
		Nullable: nullable,
	}, true
}

// ---- TypeORM ---------------------------------------------------------------

// TypeORM models are detected from the already-parsed symbol annotations:
// any class with @Entity() is a TypeORM model. Fields use @Column() annotation.
// We pull these from the FileInfo symbols rather than raw source.
func extractTypeORMModels(files []FileInfo) []SchemaModel {
	var models []SchemaModel
	for _, fi := range files {
		for _, sym := range fi.Symbols {
			if sym.Kind != "class" {
				continue
			}
			if !hasAnnotation(sym.Annotations, "Entity") {
				continue
			}
			model := SchemaModel{
				Name: sym.Name,
				File: fi.Path,
				Line: int(sym.Line),
				ORM:  ORMTypeORM,
			}
			for _, child := range sym.Children {
				if hasAnnotation(child.Annotations, "Column") ||
					hasAnnotation(child.Annotations, "PrimaryColumn") ||
					hasAnnotation(child.Annotations, "PrimaryGeneratedColumn") {
					model.Fields = append(model.Fields, SchemaField{
						Name: child.Name,
					})
				}
			}
			models = append(models, model)
		}
	}
	return models
}

// ---- GORM ------------------------------------------------------------------

// gormStructRe matches a Go struct whose fields have `gorm:"..."` tags.
// We detect the struct via the already-parsed symbols: any struct that has
// at least one field tag containing "gorm" in the raw source.
var gormTagRe = regexp.MustCompile("(?m)`[^`]*gorm:\"[^`]*`")

func extractGORMModels(allPaths []string, files []FileInfo, displayRoot string) []SchemaModel {
	// Build a set of .go files that contain gorm tags
	gormFiles := map[string]bool{}
	for _, path := range allPaths {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		src, err := readFileSafe(path)
		if err != nil || src == nil {
			continue
		}
		if gormTagRe.Match(src) {
			gormFiles[path] = true
		}
	}
	if len(gormFiles) == 0 {
		return nil
	}

	var models []SchemaModel
	for _, fi := range files {
		if !gormFiles[fi.Path] && !gormFiles[absolutePath(fi.Path, displayRoot)] {
			continue
		}
		for _, sym := range fi.Symbols {
			if sym.Kind != "struct" {
				continue
			}
			model := SchemaModel{
				Name: sym.Name,
				File: fi.Path,
				Line: int(sym.Line),
				ORM:  ORMGORM,
			}
			models = append(models, model)
		}
	}
	return models
}

// ---- Dispatcher ------------------------------------------------------------

// schemaExtensions maps file extensions to extractor functions.
func collectSchemaModels(allPaths []string, files []FileInfo, displayRoot string) []SchemaModel {
	var models []SchemaModel

	// TypeORM: sourced from already-parsed symbol annotations
	models = append(models, extractTypeORMModels(files)...)

	// GORM: sourced from .go struct symbols with gorm tags
	models = append(models, extractGORMModels(allPaths, files, displayRoot)...)

	for _, path := range allPaths {
		ext := strings.ToLower(fileExt(path))
		var newModels []SchemaModel

		src, err := readFileSafe(path)
		if err != nil || src == nil {
			continue
		}
		displayPath := relativePath(path, displayRoot)

		switch ext {
		case ".prisma":
			newModels = extractPrismaModels(src, displayPath)
		case ".sql":
			newModels = extractSQLModels(src, displayPath)
		case ".py":
			newModels = extractSQLAlchemyModels(src, displayPath)
		}
		models = append(models, newModels...)
	}

	return models
}
