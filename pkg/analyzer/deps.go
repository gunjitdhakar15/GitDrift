package analyzer

import (
	"os"
	"path/filepath"
	"strings"
)

type RepoStats struct {
	TotalFiles     int
	Languages      map[string]int
	TotalLines     int
	DirectoryCount int
}

// AnalyzeRepo computes codebase metrics and language breakdown.
func AnalyzeRepo(rootDir string) (RepoStats, error) {
	stats := RepoStats{
		Languages: make(map[string]int),
	}

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
			stats.DirectoryCount++
			return nil
		}

		stats.TotalFiles++
		ext := filepath.Ext(path)
		if ext != "" {
			stats.Languages[ext]++
		}

		// Count lines
		content, err := os.ReadFile(path)
		if err == nil {
			lines := strings.Count(string(content), "\n")
			stats.TotalLines += lines
		}

		return nil
	})

	return stats, err
}
