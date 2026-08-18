package analyzer

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type TodoItem struct {
	File    string
	Line    int
	Tag     string // TODO, FIXME, HACK
	Content string
}

// ScanTodos scans all source files in directory for TODO/FIXME comments.
func ScanTodos(rootDir string) ([]TodoItem, error) {
	var items []TodoItem

	ignoredDirs := map[string]bool{
		".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true,
	}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if ignoredDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Only check source files
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".ts" && ext != ".js" && ext != ".py" && ext != ".rs" {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			text := scanner.Text()
			upper := strings.ToUpper(text)

			var tag string
			if strings.Contains(upper, "TODO") {
				tag = "TODO"
			} else if strings.Contains(upper, "FIXME") {
				tag = "FIXME"
			} else if strings.Contains(upper, "HACK") {
				tag = "HACK"
			} else if strings.Contains(upper, "OPTIMIZE") {
				tag = "OPTIMIZE"
			}

			if tag != "" {
				rel, _ := filepath.Rel(rootDir, path)
				items = append(items, TodoItem{
					File:    filepath.ToSlash(rel),
					Line:    lineNum,
					Tag:     tag,
					Content: strings.TrimSpace(text),
				})
			}
		}

		return nil
	})

	return items, err
}
