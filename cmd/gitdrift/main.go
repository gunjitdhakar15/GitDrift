package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitdrift/pkg/analyzer"
)

func main() {
	dirPtr := flag.String("dir", ".", "Path to git repository")
	idleDaysPtr := flag.Int("idle-days", 90, "Days of inactivity to consider a file stale")
	hotspotLimitPtr := flag.Int("limit", 10, "Limit for hotspot files")
	jsonPtr := flag.Bool("json", false, "Output results as JSON")
	flag.Parse()

	absDir, err := filepath.Abs(*dirPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	if !*jsonPtr {
		fmt.Println("==================================================")
		fmt.Println(" 🚀 GitDrift — Monorepo Architectural Analyzer")
		fmt.Println("==================================================")
		fmt.Printf("Analyzing repository: %s\n", absDir)
		fmt.Printf("Time: %s\n\n", time.Now().Format(time.RFC1123))
	}

	// 1. Repo Stats
	stats, err := analyzer.AnalyzeRepo(absDir)
	if err != nil && !*jsonPtr {
		fmt.Printf("Warning: Failed to analyze repo stats: %v\n", err)
	}

	// 2. Stale Files
	staleFiles, err := analyzer.FindStaleFiles(absDir, *idleDaysPtr)
	if err != nil && !*jsonPtr {
		fmt.Printf("Warning: Failed to scan stale files (Is this a git repo?): %v\n", err)
	}

	// 3. Hotspots
	hotspots, err := analyzer.FindHotspots(absDir, *hotspotLimitPtr)
	if err != nil && !*jsonPtr {
		fmt.Printf("Warning: Failed to scan git hotspots: %v\n", err)
	}

	// 4. TODOs
	todos, err := analyzer.ScanTodos(absDir)
	if err != nil && !*jsonPtr {
		fmt.Printf("Warning: Failed to scan TODOs: %v\n", err)
	}

	if *jsonPtr {
		output := map[string]interface{}{
			"stats":    stats,
			"stale":    staleFiles,
			"hotspots": hotspots,
			"todos":    todos,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Print Summary
	fmt.Println("📊 Codebase Metrics:")
	fmt.Printf("   • Total Files:     %d\n", stats.TotalFiles)
	fmt.Printf("   • Total Lines:     %d\n", stats.TotalLines)
	fmt.Printf("   • Directories:     %d\n", stats.DirectoryCount)
	fmt.Println("   • Language Breakdown:")
	for ext, count := range stats.Languages {
		if ext != "" {
			fmt.Printf("     - %s: %d files\n", ext, count)
		}
	}
	fmt.Println()

	// Print Hotspots
	fmt.Printf("🔥 Top High-Churn Files (Hotspots):\n")
	if len(hotspots) == 0 {
		fmt.Println("   (No git history or no hotspots found)")
	}
	for _, h := range hotspots {
		fmt.Printf("   [%d commits] %s\n", h.CommitCount, h.Path)
	}
	fmt.Println()

	// Print Stale Files
	fmt.Printf("🧟 Stale / Zombie Files (Unmodified > %d days):\n", *idleDaysPtr)
	if len(staleFiles) == 0 {
		fmt.Println("   (No stale files found)")
	}
	for _, sf := range staleFiles {
		fmt.Printf("   [%d days idle] %s\n", sf.DaysIdle, sf.Path)
	}
	fmt.Println()

	// Print Tech Debt
	fmt.Printf("🛠️  Tech Debt & TODOs (Total: %d):\n", len(todos))
	limitTodos := 10
	if len(todos) < limitTodos {
		limitTodos = len(todos)
	}
	for i := 0; i < limitTodos; i++ {
		t := todos[i]
		fmt.Printf("   [%s] %s:%d - %s\n", t.Tag, t.File, t.Line, t.Content)
	}
	if len(todos) > limitTodos {
		fmt.Printf("   ... and %d more TODOs\n", len(todos)-limitTodos)
	}

	fmt.Println("\n==================================================")
	fmt.Println(" ✨ GitDrift Analysis Complete")
	fmt.Println("==================================================")
}
