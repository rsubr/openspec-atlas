package internal

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	ignore "github.com/sabhiram/go-gitignore"
)

type scanner struct {
	allFiles bool
	stdout   io.Writer
	stderr   io.Writer
}

type scanResult struct {
	files    []FileInfo
	allPaths []string
}

// walkSourceFiles walks projectDirs, respecting .gitignore unless allFiles is
// set, and returns the parsed FileInfo list and the flat list of every file path
// encountered (used by extended analysers).
func walkSourceFiles(projectDirs []string, allFiles bool, stdout, stderr io.Writer) ([]FileInfo, []string) {
	result := scanner{
		allFiles: allFiles,
		stdout:   stdout,
		stderr:   stderr,
	}.walkProjects(projectDirs)
	return result.files, result.allPaths
}

// walkProjects scans each requested directory independently and merges the
// results so one failing project does not block the others.
func (s scanner) walkProjects(projectDirs []string) scanResult {
	var result scanResult
	for _, projectDir := range projectDirs {
		files, paths, err := s.walkProject(projectDir)
		if err != nil {
			fmt.Fprintf(s.stderr, "walk error in %s: %v\n", projectDir, err)
		}
		result.files = append(result.files, files...)
		result.allPaths = append(result.allPaths, paths...)
	}
	return result
}

// walkProject traverses one project directory, keeps a flat list of every path
// encountered for the secondary analyzers, and parses only files with a known
// language configuration.
func (s scanner) walkProject(projectDir string) ([]FileInfo, []string, error) {
	var files []FileInfo
	var allPaths []string
	ignoreCache := map[string]*ignore.GitIgnore{}

	err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skip, err := s.shouldSkip(path, d, projectDir, ignoreCache); skip || err != nil {
			return err
		}

		allPaths = append(allPaths, path)

		fi, ok, err := s.parseSourceFile(projectDir, path)
		if err != nil {
			return err
		}
		if ok {
			files = append(files, fi)
		}
		return nil
	})
	return files, allPaths, err
}

// shouldSkip centralizes walk filtering so ignored directories, ignored files,
// and the repository's .git directory are handled consistently.
func (s scanner) shouldSkip(path string, d fs.DirEntry, projectDir string, ignoreCache map[string]*ignore.GitIgnore) (bool, error) {
	if d.IsDir() && d.Name() == ".git" {
		return true, fs.SkipDir
	}
	if !s.allFiles && isGitIgnored(path, projectDir, ignoreCache) {
		if d.IsDir() {
			return true, fs.SkipDir
		}
		return true, nil
	}
	return d.IsDir(), nil
}

// parseSourceFile parses one supported source file. Parse failures are reported
// to stderr and skipped so a single bad file does not abort the whole scan.
func (s scanner) parseSourceFile(projectDir, path string) (FileInfo, bool, error) {
	config, ok := languageForFile(path)
	if !ok {
		return FileInfo{}, false, nil
	}

	displayPath, err := displayPathForProject(projectDir, path)
	if err != nil {
		return FileInfo{}, false, err
	}

	fmt.Fprintf(s.stdout, "parsing [%s] %s\n", config.Name, displayPath)

	fi, err := parseFile(path, config)
	if err != nil {
		fmt.Fprintf(s.stderr, "parse error in %s: %v\n", path, err)
		return FileInfo{}, false, nil
	}

	fi.Path = displayPath
	return fi, true, nil
}

// displayPathForProject keeps file paths stable in output by expressing each
// file relative to the project root that was scanned.
func displayPathForProject(projectDir, path string) (string, error) {
	rel, err := filepath.Rel(projectDir, path)
	if err != nil {
		return "", fmt.Errorf("make relative path for %s: %w", path, err)
	}
	return filepath.Join(projectDir, rel), nil
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
