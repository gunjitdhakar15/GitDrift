package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitdrift-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dummy go file
	testFile := filepath.Join(tmpDir, "main.go")
	content := []byte("package main\n\n// TODO: fix this\nfunc main() {}\n")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	stats, err := AnalyzeRepo(tmpDir)
	if err != nil {
		t.Fatalf("AnalyzeRepo failed: %v", err)
	}

	if stats.TotalFiles != 1 {
		t.Errorf("expected 1 file, got %d", stats.TotalFiles)
	}

	todos, err := ScanTodos(tmpDir)
	if err != nil {
		t.Fatalf("ScanTodos failed: %v", err)
	}

	if len(todos) != 1 {
		t.Errorf("expected 1 TODO, got %d", len(todos))
	}

	if todos[0].Tag != "TODO" {
		t.Errorf("expected tag TODO, got %s", todos[0].Tag)
	}
}
