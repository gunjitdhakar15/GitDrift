package analyzer

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Cycle struct {
	Path []string
}

// PackageNode is a single Go package with its internal dependencies.
type PackageNode struct {
	Path         string
	Dependencies []string
}

// FindImportCycles parses all Go packages in the repo and detects
// circular import dependencies using depth-first search.
func FindImportCycles(rootDir, moduleName string) ([]Cycle, []PackageNode, error) {
	imports, err := parseGoImports(rootDir)
	if err != nil {
		return nil, nil, err
	}

	graph := buildPackageGraph(imports, moduleName, rootDir)

	var cycles []Cycle
	visited := make(map[string]bool)
	onStack := make(map[string]bool)
	var stack []string

	var dfs func(node string)
	dfs = func(node string) {
		if visited[node] {
			return
		}
		visited[node] = true
		onStack[node] = true
		stack = append(stack, node)

		for _, dep := range graph[node] {
			if onStack[dep] {
				idx := indexOf(stack, dep)
				cycle := append(append([]string{}, stack[idx:]...), dep)
				cycles = append(cycles, Cycle{Path: cycle})
				continue
			}
			dfs(dep)
		}

		stack = stack[:len(stack)-1]
		onStack[node] = false
	}

	for node := range graph {
		if !visited[node] {
			dfs(node)
		}
	}

	var nodes []PackageNode
	for path, deps := range graph {
		nodes = append(nodes, PackageNode{Path: path, Dependencies: deps})
	}

	return cycles, nodes, nil
}

// parseGoImports maps every package import path to its internal dependency import paths.
func parseGoImports(rootDir string) (map[string][]string, error) {
	imports := make(map[string][]string)

	ignoredDirs := map[string]bool{
		".git": true, "vendor": true, "node_modules": true, "dist": true, "build": true,
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
		if filepath.Ext(path) != ".go" {
			return nil
		}

		pkgDir := filepath.Dir(path)
		pkgImports := imports[pkgDir]

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		inImportBlock := false
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			switch {
			case line == "import (":
				inImportBlock = true
				continue
			case line == ")":
				inImportBlock = false
				continue
			case strings.HasPrefix(line, "import "):
				pkgImports = append(pkgImports, extractImport(line))
				continue
			}
			if inImportBlock {
				pkgImports = append(pkgImports, extractImport(line))
			}
		}
		imports[pkgDir] = uniqueStrings(pkgImports)
		return nil
	})

	return imports, err
}

// extractImport pulls the import path from a single-line or block import entry.
func extractImport(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "import"))
	if strings.HasPrefix(line, "(") || strings.HasSuffix(line, ")") {
		line = strings.Trim(line, "()")
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	last := fields[len(fields)-1]
	return strings.Trim(last, `"`)
}

// buildPackageGraph resolves internal import paths to package directories
// and builds the dependency graph keyed by import path.
func buildPackageGraph(imports map[string][]string, moduleName, rootDir string) map[string][]string {
	graph := make(map[string][]string)

	// import path -> package directory
	pathToDir := make(map[string]string)
	for pkgDir := range imports {
		rel, err := filepath.Rel(rootDir, pkgDir)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			continue
		}
		pathToDir[moduleName+"/"+rel] = pkgDir
	}

	// package directory -> its import path
	dirToPath := make(map[string]string)
	for path, dir := range pathToDir {
		dirToPath[dir] = path
	}

	for pkgDir, deps := range imports {
		pkgPath := dirToPath[pkgDir]
		if pkgPath == "" {
			continue
		}
		graph[pkgPath] = nil

		for _, dep := range deps {
			if depDir, ok := pathToDir[dep]; ok {
				if depPath := dirToPath[depDir]; depPath != "" {
					graph[pkgPath] = append(graph[pkgPath], depPath)
				}
			}
		}
	}

	for k, v := range graph {
		graph[k] = uniqueStrings(v)
	}
	return graph
}

func indexOf(list []string, target string) int {
	for i, s := range list {
		if s == target {
			return i
		}
	}
	return -1
}

func uniqueStrings(list []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range list {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
