package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// fixture generates a synthetic monorepo with N Go files across M packages.
func fixture(t testing.TB, packages, filesPerPackage int) string {
	t.Helper()
	if pt, ok := t.(*testing.T); ok {
		root := pt.TempDir()
		buildFixture(t, root, packages, filesPerPackage)
		return root
	}
	root, err := os.MkdirTemp("", "gitdrift-fixture")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	buildFixture(t, root, packages, filesPerPackage)
	return root
}

func buildFixture(t testing.TB, root string, packages, filesPerPackage int) {
	for p := 0; p < packages; p++ {
		dir := filepath.Join(root, "pkg", fmt.Sprintf("p%02d", p))
		for f := 0; f < filesPerPackage; f++ {
			path := filepath.Join(dir, fmt.Sprintf("file%02d.go", f))
			content := fmt.Sprintf("package p%02d\n\n// TODO: fix this later\nfunc F%02d() {}\n", p, f)
			mustWriteFile(t, path, content)
		}
	}
}

// BenchmarkScanTodos measures the concurrent TODO scanner over a synthetic
// monorepo of 200 packages x 10 files = 2000 files.
func BenchmarkScanTodos(b *testing.B) {
	root := fixture(b, 200, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items, err := ScanTodos(root)
		if err != nil {
			b.Fatal(err)
		}
		if len(items) != 2000 {
			b.Fatalf("expected 2000 todos, got %d", len(items))
		}
	}
}

// BenchmarkAnalyzeRepo measures concurrent repo metrics over 2000 files.
func BenchmarkAnalyzeRepo(b *testing.B) {
	root := fixture(b, 200, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats, err := AnalyzeRepo(root)
		if err != nil {
			b.Fatal(err)
		}
		if stats.TotalFiles == 0 {
			b.Fatal("expected files")
		}
	}
}

// BenchmarkFindImportCycles measures graph construction and DFS over a
// realistic package graph with 100 packages.
func BenchmarkFindImportCycles(b *testing.B) {
	root := fixture(b, 100, 5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := FindImportCycles(root, "example.com/demo")
		if err != nil {
			b.Fatal(err)
		}
	}
}

var _ = os.RemoveAll
