package internals

import (
	"path/filepath"

	ignore "github.com/sabhiram/go-gitignore"
)

func isGitIgnored(path, root string, cache map[string]*ignore.GitIgnore) bool {
	dir := filepath.Dir(path)
	for {
		if _, loaded := cache[dir]; !loaded {
			m, err := ignore.CompileIgnoreFile(filepath.Join(dir, ".gitignore"))
			if err != nil {
				cache[dir] = nil
			} else {
				cache[dir] = m
			}
		}
		if m := cache[dir]; m != nil {
			rel, err := filepath.Rel(dir, path)
			if err == nil && m.MatchesPath(rel) {
				return true
			}
		}
		if dir == root {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}
