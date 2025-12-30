package watch

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

type IgnoreFilter struct {
	root     string
	patterns []gitignore.Pattern
}

func NewIgnoreFilter(root string) (*IgnoreFilter, error) {
	f := &IgnoreFilter{root: root}

	// Add default patterns
	defaultPatterns := []string{
		".git",
		"node_modules",
		"__pycache__",
		"*.pyc",
		".DS_Store",
		"*.lock",
		"*.log",
		"vendor",
		"dist",
		"build",
		".idea",
		".vscode",
	}

	for _, p := range defaultPatterns {
		pattern := gitignore.ParsePattern(p, nil)
		f.patterns = append(f.patterns, pattern)
	}

	// Load .gitignore if exists
	gitignorePath := filepath.Join(root, ".gitignore")
	if file, err := os.Open(gitignorePath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			pattern := gitignore.ParsePattern(line, nil)
			f.patterns = append(f.patterns, pattern)
		}
	}

	// Load .riceignore if exists
	riceignorePath := filepath.Join(root, ".riceignore")
	if file, err := os.Open(riceignorePath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			pattern := gitignore.ParsePattern(line, nil)
			f.patterns = append(f.patterns, pattern)
		}
	}

	return f, nil
}

func (f *IgnoreFilter) ShouldIgnore(path string) bool {
	relPath, err := filepath.Rel(f.root, path)
	if err != nil {
		return false
	}

	// Check each pattern
	pathParts := strings.Split(relPath, string(filepath.Separator))
	for _, pattern := range f.patterns {
		if pattern.Match(pathParts, false) == gitignore.Exclude {
			return true
		}
	}

	return false
}
