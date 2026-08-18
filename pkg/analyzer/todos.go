package analyzer

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type TodoItem struct {
	File    string
	Line    int
	Tag     string // TODO, FIXME, HACK, OPTIMIZE
	Content string
}

var todoPattern = regexp.MustCompile(`\b(TODO|FIXME|HACK|OPTIMIZE)\b`)

// ScanTodos scans all source files concurrently for TODO/FIXME comments.
func ScanTodos(rootDir string) ([]TodoItem, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	items, err := ConcurrentMap(rootDir, func(path string, info os.FileInfo) []TodoItem {
		rel, _ := filepath.Rel(absRoot, path)
		return scanFileTodos(filepath.ToSlash(rel), path)
	})
	if err != nil {
		return nil, err
	}

	var flattened []TodoItem
	for _, fileItems := range items {
		flattened = append(flattened, fileItems...)
	}
	return flattened, nil
}

// scanFileTodos extracts every TODO-family marker in a single file,
// matching at word boundaries so identifiers like TodoItem are ignored.
func scanFileTodos(displayPath, path string) []TodoItem {
	var items []TodoItem

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		text := scanner.Text()
		match := todoPattern.FindString(text)
		if match == "" {
			continue
		}
		items = append(items, TodoItem{
			File:    displayPath,
			Line:    lineNum,
			Tag:     strings.ToUpper(match),
			Content: strings.TrimSpace(text),
		})
	}
	return items
}
