package internals

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// ---- Prisma ----------------------------------------------------------------

var (
	prismaModelRe = regexp.MustCompile(`(?m)^model\s+(\w+)\s*\{`)
	prismaFieldRe = regexp.MustCompile(`^\s+(\w+)\s+(\w+)(\?)?\s`)
)

func extractPrismaModels(src []byte, file string) []SchemaModel {
	var models []SchemaModel
	lines := splitLines(src)
	for i, line := range lines {
		m := prismaModelRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		model := SchemaModel{
			Name: m[1],
			File: file,
			Line: i + 1,
			ORM:  "prisma",
		}
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
		model := SchemaModel{
			Name: m[1],
			File: file,
			Line: i + 1,
			ORM:  "sql",
		}
		depth := strings.Count(line, "(") - strings.Count(line, ")")
		for j := i + 1; j < len(lines) && depth > 0; j++ {
			fl := lines[j]
			depth += strings.Count(fl, "(") - strings.Count(fl, ")")
			fm := sqlColumnRe.FindStringSubmatch(fl)
			if fm == nil {
				continue
			}
			colName := fm[1]
			colType := fm[2]
			// Skip SQL keywords that appear in constraint lines
			upper := strings.ToUpper(colName)
			if upper == "PRIMARY" || upper == "UNIQUE" || upper == "FOREIGN" ||
				upper == "INDEX" || upper == "KEY" || upper == "CHECK" ||
				upper == "CONSTRAINT" {
				continue
			}
			nullable := !sqlNotNullRe.MatchString(fl)
			model.Fields = append(model.Fields, SchemaField{
				Name:     colName,
				Type:     strings.ToLower(colType),
				Nullable: nullable,
			})
		}
		models = append(models, model)
	}
	return models
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
	inClass := false
	var current *SchemaModel
	indent := ""

	for i, line := range lines {
		if m := saClassRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				models = append(models, *current)
			}
			model := SchemaModel{
				Name: m[1],
				File: file,
				Line: i + 1,
				ORM:  "sqlalchemy",
			}
			current = &model
			inClass = true
			indent = leadingSpaces(line) + " " // at least one more space = inside class
			continue
		}
		if inClass && current != nil {
			// Detect end of class by non-empty line with equal or less indent
			if len(line) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") &&
				!strings.HasPrefix(strings.TrimSpace(line), "#") {
				models = append(models, *current)
				current = nil
				inClass = false
			} else if fm := saColumnRe.FindStringSubmatch(line); fm != nil {
				colName := fm[1]
				colArgs := fm[2]
				colType := ""
				if tm := saTypeRe.FindStringSubmatch(colArgs); tm != nil {
					colType = tm[1]
				}
				nullable := true
				if nm := saNullRe.FindStringSubmatch(colArgs); nm != nil {
					nullable = strings.EqualFold(nm[1], "True")
				}
				current.Fields = append(current.Fields, SchemaField{
					Name:     colName,
					Type:     strings.ToLower(colType),
					Nullable: nullable,
				})
			}
		}
		_ = indent
	}
	if current != nil {
		models = append(models, *current)
	}
	return models
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
				ORM:  "typeorm",
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
				ORM:  "gorm",
			}
			models = append(models, model)
		}
	}
	return models
}

// absolutePath reconstructs an absolute path from a display path + root.
func absolutePath(display, root string) string {
	if strings.HasPrefix(display, "/") {
		return display
	}
	return root + "/" + display
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

// ---- Helpers ---------------------------------------------------------------

func splitLines(src []byte) []string {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(src))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func leadingSpaces(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}
