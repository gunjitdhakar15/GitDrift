package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ModuleName reads the module path from a Go module root.
func ModuleName(rootDir string) string {
	data, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// BuildReport runs every analyzer once and packages the results.
func BuildReport(rootDir string) (*Report, error) {
	stats, err := AnalyzeRepo(rootDir)
	if err != nil {
		return nil, fmt.Errorf("repo stats: %w", err)
	}

	hotspots, err := FindHotspots(rootDir, 15)
	if err != nil {
		return nil, fmt.Errorf("hotspots: %w", err)
	}

	stale, err := FindStaleFiles(rootDir, 90)
	if err != nil {
		return nil, fmt.Errorf("stale files: %w", err)
	}

	todos, err := ScanTodos(rootDir)
	if err != nil {
		return nil, fmt.Errorf("todos: %w", err)
	}

	cycles, packages, err := FindImportCycles(rootDir, ModuleName(rootDir))
	if err != nil {
		return nil, fmt.Errorf("import cycles: %w", err)
	}

	return &Report{
		GeneratedAt:  time.Now(),
		Repo:         rootDir,
		Stats:        stats,
		Hotspots:     hotspots,
		StaleFiles:   stale,
		Todos:        todos,
		ImportCycles: cycles,
		Packages:     packages,
	}, nil
}
