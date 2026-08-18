package analyzer

import (
	"os"
	"path/filepath"
	"sync"
)

type RepoStats struct {
	TotalFiles     int
	Languages      map[string]int
	TotalLines     int
	DirectoryCount int
}

// AnalyzeRepo computes codebase metrics and language breakdown concurrently.
func AnalyzeRepo(rootDir string) (RepoStats, error) {
	stats := RepoStats{Languages: make(map[string]int)}
	var mu sync.Mutex

	dirCount, err := countDirectories(rootDir)
	if err != nil {
		return stats, err
	}
	stats.DirectoryCount = dirCount

	_, err = ConcurrentMap(rootDir, func(path string, info os.FileInfo) struct{} {
		lines := countLines(path)

		mu.Lock()
		stats.TotalFiles++
		stats.TotalLines += lines
		ext := filepath.Ext(path)
		if ext != "" {
			stats.Languages[ext]++
		}
		mu.Unlock()
		return struct{}{}
	})
	if err != nil {
		return stats, err
	}

	return stats, nil
}

func countDirectories(rootDir string) (int, error) {
	count := 0
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if path != rootDir && IgnoredDirs[info.Name()] {
			return filepath.SkipDir
		}
		count++
		return nil
	})
	return count, err
}

func countLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	lines := 1
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	return lines
}
