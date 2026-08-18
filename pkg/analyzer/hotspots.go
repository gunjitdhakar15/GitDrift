package analyzer

import (
	"bufio"
	"bytes"
	"os/exec"
	"sort"
	"strings"
)

type HotspotFile struct {
	Path        string
	CommitCount int
}

// FindHotspots analyzes git log to find the most frequently modified files (churn hotspots).
func FindHotspots(repoDir string, limit int) ([]HotspotFile, error) {
	cmd := exec.Command("git", "-C", repoDir, "log", "--name-only", "--format=")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		counts[line]++
	}

	var hotspots []HotspotFile
	for path, count := range counts {
		hotspots = append(hotspots, HotspotFile{
			Path:        path,
			CommitCount: count,
		})
	}

	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].CommitCount > hotspots[j].CommitCount
	})

	if len(hotspots) > limit {
		hotspots = hotspots[:limit]
	}

	return hotspots, nil
}
