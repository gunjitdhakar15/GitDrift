package analyzer

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// SourceExts are the file extensions scanned for code-level analysis.
var SourceExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".rs": true, ".java": true, ".c": true, ".cpp": true,
	".h": true, ".rb": true, ".php": true, ".kt": true, ".swift": true,
	".cs": true, ".scala": true, ".sh": true, ".sql": true,
}

// IgnoredDirs are directories skipped during recursive scans.
var IgnoredDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true, "dist": true,
	"build": true, ".idea": true, ".vscode": true, "target": true,
	"__pycache__": true, "coverage": true,
}

// WalkSourceFiles walks rootDir and sends every source file path to
// the callback, which runs concurrently on a worker pool.
func WalkSourceFiles(rootDir string, fn func(path string, info os.FileInfo)) error {
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}

	paths := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range paths {
				info, err := os.Stat(path)
				if err != nil {
					continue
				}
				fn(path, info)
			}
		}()
	}

	walkErr := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path != rootDir && IgnoredDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !SourceExts[filepath.Ext(info.Name())] {
			return nil
		}
		paths <- path
		return nil
	})

	close(paths)
	wg.Wait()
	return walkErr
}

// ConcurrentMap applies fn to every source file concurrently and collects
// the typed results, preserving no ordering guarantees.
func ConcurrentMap[T any](rootDir string, fn func(path string, info os.FileInfo) T) ([]T, error) {
	var results []T
	var mu sync.Mutex

	err := WalkSourceFiles(rootDir, func(path string, info os.FileInfo) {
		res := fn(path, info)
		mu.Lock()
		results = append(results, res)
		mu.Unlock()
	})

	return results, err
}
