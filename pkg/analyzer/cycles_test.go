package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindImportCycles(t *testing.T) {
	root, err := os.MkdirTemp("", "gitdrift-cycle")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	// pkg/a imports pkg/b, pkg/b imports pkg/a  -> cycle
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	mustWriteFile(t, filepath.Join(root, "pkg", "a", "a.go"), "package a\n\nimport \"example.com/demo/pkg/b\"\n")
	mustWriteFile(t, filepath.Join(root, "pkg", "b", "b.go"), "package b\n\nimport \"example.com/demo/pkg/a\"\n")

	cycles, packages, err := FindImportCycles(root, "example.com/demo")
	if err != nil {
		t.Fatalf("FindImportCycles: %v", err)
	}

	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d", len(cycles))
	}
	if len(cycles[0].Path) != 3 {
		t.Errorf("expected 3-node cycle, got %v", cycles[0].Path)
	}
	if len(packages) != 2 {
		t.Errorf("expected 2 packages, got %v", packages)
	}
}

func TestNoImportCyclesForAcyclicGraph(t *testing.T) {
	root, err := os.MkdirTemp("", "gitdrift-acyclic")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	mustWriteFile(t, filepath.Join(root, "pkg", "a", "a.go"), "package a\n\nimport \"example.com/demo/pkg/b\"\n")
	mustWriteFile(t, filepath.Join(root, "pkg", "b", "b.go"), "package b\n")

	cycles, _, err := FindImportCycles(root, "example.com/demo")
	if err != nil {
		t.Fatalf("FindImportCycles: %v", err)
	}
	if len(cycles) != 0 {
		t.Errorf("expected 0 cycles, got %v", cycles)
	}
}

func TestFindHotspotsHandlesMissingGit(t *testing.T) {
	root, err := os.MkdirTemp("", "gitdrift-nogit")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	// Not a git repository -> must return an error, not a panic.
	if _, err := FindHotspots(root, 5); err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestExtractImportVariants(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`import "example.com/demo/pkg/a"`, "example.com/demo/pkg/a"},
		{`alias "example.com/demo/pkg/b"`, "example.com/demo/pkg/b"},
		{`"example.com/demo/pkg/c"`, "example.com/demo/pkg/c"},
		{`_ "example.com/demo/pkg/d"`, "example.com/demo/pkg/d"},
	}
	for _, c := range cases {
		if got := extractImport(c.input); got != c.want {
			t.Errorf("extractImport(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func mustWriteFile(t testing.TB, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
