package internals

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	ignore "github.com/sabhiram/go-gitignore"
)

func scanProjects(projectDirs []string, allFiles bool, stdout, stderr io.Writer) []FileInfo {
	var files []FileInfo

	for _, projectDir := range projectDirs {
		ignoreCache := map[string]*ignore.GitIgnore{}
		err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && info.Name() == ".git" {
				return filepath.SkipDir
			}
			if !allFiles && isGitIgnored(path, projectDir, ignoreCache) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if info.IsDir() {
				return nil
			}

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

	return files
}
