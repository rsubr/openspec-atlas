package internal

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	ignore "github.com/sabhiram/go-gitignore"
)

// walkSourceFiles walks projectDirs, respecting .gitignore unless allFiles is
// set, and returns the parsed FileInfo list and the flat list of every file path
// encountered (used by extended analysers).
func walkSourceFiles(projectDirs []string, allFiles bool, stdout, stderr io.Writer) ([]FileInfo, []string) {
	var files []FileInfo
	var allPaths []string

	for _, projectDir := range projectDirs {
		ignoreCache := map[string]*ignore.GitIgnore{}
		err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == ".git" {
				return fs.SkipDir
			}
			if !allFiles && isGitIgnored(path, projectDir, ignoreCache) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}

			// Collect every non-directory file for extended analysis.
			allPaths = append(allPaths, path)

			config, ok := languageForFile(path)
			if !ok {
				return nil
			}

			rel, err := filepath.Rel(projectDir, path)
			if err != nil {
				return fmt.Errorf("make relative path for %s: %w", path, err)
			}

			displayPath := filepath.Join(projectDir, rel)
			fmt.Fprintf(stdout, "parsing [%s] %s\n", config.Name, displayPath)

			fi, parseErr := parseFile(path, config)
			if parseErr != nil {
				fmt.Fprintf(stderr, "parse error in %s: %v\n", path, parseErr)
				return nil
			}

			fi.Path = displayPath
			files = append(files, fi)
			return nil
		})
		if err != nil {
			fmt.Fprintf(stderr, "walk error in %s: %v\n", projectDir, err)
		}
	}
	return files, allPaths
}

// scanProjects walks projectDirs, parses source files, and runs all extended
// analysers (env vars, HTTP edges, schema models, middleware, UI components).
// When multiple directories are provided, extended-analyser paths are shown
// as absolute paths (displayRoot is left empty).
func scanProjects(projectDirs []string, allFiles bool, stdout, stderr io.Writer) Output {
	files, allPaths := walkSourceFiles(projectDirs, allFiles, stdout, stderr)

	// Use the first project directory as display root for relative paths in
	// extended analysis output. When multiple dirs are given, display root
	// is left empty and all paths are shown as absolute.
	displayRoot := ""
	if len(projectDirs) == 1 {
		displayRoot = projectDirs[0]
	}

	return Output{
		Files:        files,
		EnvVars:      collectEnvVars(allPaths, displayRoot),
		HttpEdges:    collectHTTPEdges(allPaths, files, displayRoot),
		SchemaModels: collectSchemaModels(allPaths, files, displayRoot),
		Middleware:   collectMiddleware(allPaths, files, displayRoot),
		UIComponents: collectUIComponents(allPaths, files, displayRoot),
	}
}
