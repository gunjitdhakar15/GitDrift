package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gitdrift/pkg/analyzer"
)

const (
	version = "v0.2.0"
	usage   = `GitDrift — monorepo architectural drift analyzer

Usage:
  gitdrift [global flags] <command> [flags]

Commands:
  scan       Run every analyzer and print the full health report
  stale      Find zombie files untouched by git for N days
  todos      Find TODO / FIXME / HACK / OPTIMIZE markers
  hotspots   Find high-churn files from git history
  deps       Detect circular Go package imports
  report     Generate a self-contained HTML dashboard
  version    Print version

Global flags:
  --dir <path>   Git repository to analyze (default ".")
  --json         Emit machine-readable JSON
  --help         Show this help
`
)

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	command := os.Args[1]

	// Extract the global --dir flag, which may appear before the subcommand.
	rootDir, rest, err := parseGlobalFlags(os.Args[1:])
	if err != nil {
		fail("%v", err)
	}
	if command == "--dir" || command == "-dir" {
		// "--dir X scan" style: re-derive command from the remaining args.
		if len(rest) == 0 {
			fmt.Print(usage)
			os.Exit(0)
		}
		command = rest[0]
		rest = rest[1:]
	}

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fail("resolve path: %v", err)
	}

	// Strip the leading command name so per-command flag parsing
	// isn't stopped by a non-flag argument.
	if len(rest) > 0 && rest[0] == command {
		rest = rest[1:]
	}

	switch command {
	case "scan":
		cmdScan(rest, absRoot)
	case "stale":
		cmdStale(rest, absRoot)
	case "todos":
		cmdTodos(rest, absRoot)
	case "hotspots":
		cmdHotspots(rest, absRoot)
	case "deps":
		cmdDeps(rest, absRoot)
	case "report":
		cmdReport(rest, absRoot)
	case "version":
		fmt.Println("gitdrift", version)
	case "--help", "-h", "help":
		fmt.Print(usage)
	default:
		fail("unknown command %q\n\n%s", command, usage)
	}
}

// parseGlobalFlags pulls --dir out of the raw args (before the subcommand).
func parseGlobalFlags(args []string) (rootDir string, rest []string, err error) {
	rootDir = "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir", "-dir":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("flag %s requires a value", args[i])
			}
			rootDir = args[i+1]
			i++
		case "--json":
			rest = append(rest, args[i])
		case "--help", "-h":
			rest = append(rest, args[i])
		default:
			rest = append(rest, args[i])
		}
	}
	return rootDir, rest, nil
}

func cmdScan(args []string, rootDir string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "JSON output")
	fs.Parse(args)

	start := time.Now()
	report, err := analyzer.BuildReport(rootDir)
	if err != nil {
		fail("%v", err)
	}

	if *jsonOut {
		writeJSON(report)
		return
	}

	banner()
	fmt.Printf("  repo:   %s\n", cyan(report.Repo))
	fmt.Printf("  engine: Go %s / %s / %d workers\n", runtime.Version(), runtime.GOOS, runtime.NumCPU())
	fmt.Printf("  time:   %s\n\n", time.Now().Format(time.RFC1123))

	printStats(report)
	printHotspots(report.Hotspots)
	printStale(report.StaleFiles)
	printTodos(report.Todos)
	printCycles(report.ImportCycles)

	fmt.Println()
	fmt.Printf("%s Analysis complete in %s %s\n", bold(green("✔")), bold(green(time.Since(start).Round(time.Millisecond).String())), gray("(gitdrift "+version+")"))
}

func cmdStale(args []string, rootDir string) {
	fs := flag.NewFlagSet("stale", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "JSON output")
	idleDays := fs.Int("idle-days", 90, "Days of inactivity to flag a file")
	fs.Parse(args)

	stale, err := analyzer.FindStaleFiles(rootDir, *idleDays)
	if err != nil {
		fail("git scan failed: %v (is this a git repository?)", err)
	}

	if *jsonOut {
		writeJSON(stale)
		return
	}
	printStale(stale)
}

func cmdTodos(args []string, rootDir string) {
	fs := flag.NewFlagSet("todos", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "JSON output")
	limit := fs.Int("limit", 50, "Max items to print")
	fs.Parse(args)

	todos, err := analyzer.ScanTodos(rootDir)
	if err != nil {
		fail("%v", err)
	}

	if *jsonOut {
		writeJSON(todos)
		return
	}

	fmt.Printf("\n%s %s\n", bold(cyan("TECH DEBT")), gray(fmt.Sprintf("(%d markers)", len(todos))))
	count := 0
	for _, t := range todos {
		if count >= *limit {
			fmt.Printf("%s\n", gray(fmt.Sprintf("  … and %d more", len(todos)-count)))
			break
		}
		count++
		fmt.Printf("  %s %s:%d %s\n", todoTag(t.Tag), cyan(t.File), t.Line, gray(shorten(t.Content, 90)))
	}
	fmt.Println()
}

func cmdHotspots(args []string, rootDir string) {
	fs := flag.NewFlagSet("hotspots", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "JSON output")
	limit := fs.Int("limit", 10, "Number of hotspots to show")
	fs.Parse(args)

	hotspots, err := analyzer.FindHotspots(rootDir, *limit)
	if err != nil {
		fail("git scan failed: %v (is this a git repository?)", err)
	}

	if *jsonOut {
		writeJSON(hotspots)
		return
	}
	printHotspots(hotspots)
}

func cmdDeps(args []string, rootDir string) {
	fs := flag.NewFlagSet("deps", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "JSON output")
	fs.Parse(args)

	cycles, packages, err := analyzer.FindImportCycles(rootDir, analyzer.ModuleName(rootDir))
	if err != nil {
		fail("%v", err)
	}

	if *jsonOut {
		writeJSON(map[string]any{"cycles": cycles, "packages": packages})
		return
	}
	printCycles(cycles)

	fmt.Printf("%s %s\n", bold(blue("PACKAGE GRAPH")), gray(fmt.Sprintf("(%d packages)", len(packages))))
	for _, p := range packages {
		fmt.Printf("  %s %s\n", cyan(p.Path), gray(shorten(strings.Join(p.Dependencies, ", "), 80)))
	}
	fmt.Println()
}

func cmdReport(args []string, rootDir string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	out := fs.String("out", "gitdrift-report.html", "Output HTML file")
	fs.Parse(args)

	report, err := analyzer.BuildReport(rootDir)
	if err != nil {
		fail("%v", err)
	}

	if err := report.WriteHTML(*out); err != nil {
		fail("write report: %v", err)
	}
	fmt.Printf("%s HTML report written to %s\n", green("✔"), bold(*out))
}

// --- presentation helpers ---

func banner() {
	fmt.Println()
	fmt.Printf("%s\n", bold(cyan("┌────────────────────────────────────────────┐")))
	fmt.Printf("%s\n", bold(cyan("│  🚀 GitDrift — Monorepo Drift Analyzer    │")))
	fmt.Printf("%s\n", bold(cyan("└────────────────────────────────────────────┘")))
}

func printStats(report *analyzer.Report) {
	stats := report.Stats
	fmt.Printf("%s\n", bold(blue("📊 CODEBASE METRICS")))
	fmt.Printf("  %s %-14s %s\n", gray("files"), cyan(fmt.Sprint(stats.TotalFiles)), "")
	fmt.Printf("  %s %-14s %s\n", gray("lines"), cyan(fmt.Sprint(stats.TotalLines)), "")
	fmt.Printf("  %s %-14s %s\n", gray("packages"), cyan(fmt.Sprint(len(report.Packages))), "")
	fmt.Printf("  %s %-14s %s\n", gray("dirs"), cyan(fmt.Sprint(stats.DirectoryCount)), "")

	exts := make([]string, 0, len(stats.Languages))
	for ext := range stats.Languages {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	fmt.Printf("  %s\n", gray("languages:"))
	for _, ext := range exts {
		fmt.Printf("    %s %s\n", cyan(ext), gray(fmt.Sprintf("%d files", stats.Languages[ext])))
	}
	fmt.Println()
}

func printHotspots(hotspots []analyzer.HotspotFile) {
	fmt.Printf("%s\n", bold(yellow("🔥 HIGH-CHURN HOTSPOTS")))
	if len(hotspots) == 0 {
		fmt.Printf("  %s\n", gray("no git history or no hotspots found"))
	}
	for _, h := range hotspots {
		fmt.Printf("  %s %s %s\n", bold(fmt.Sprintf("[%3d commits]", h.CommitCount)), cyan(h.Path), "")
	}
	fmt.Println()
}

func printStale(stale []analyzer.StaleFile) {
	fmt.Printf("%s\n", bold(magenta("🧟 STALE / ZOMBIE FILES")))
	if len(stale) == 0 {
		fmt.Printf("  %s\n", gray("clean — no stale files found"))
	}
	for _, s := range stale {
		fmt.Printf("  %s %s\n", bold(fmt.Sprintf("[%4d days]", s.DaysIdle)), yellow(s.Path))
	}
	fmt.Println()
}

func printTodos(todos []analyzer.TodoItem) {
	fmt.Printf("%s\n", bold(cyan("🛠 TECH DEBT")))
	if len(todos) == 0 {
		fmt.Printf("  %s\n", gray("no tech debt markers found"))
	}
	count := 0
	for _, t := range todos {
		if count >= 12 {
			fmt.Printf("  %s\n", gray(fmt.Sprintf("… and %d more markers", len(todos)-count)))
			break
		}
		count++
		fmt.Printf("  %s %s:%d %s\n", todoTag(t.Tag), cyan(t.File), t.Line, gray(shorten(t.Content, 90)))
	}
	fmt.Println()
}

func printCycles(cycles []analyzer.Cycle) {
	fmt.Printf("%s\n", bold(red("⭕ CIRCULAR IMPORTS")))
	if len(cycles) == 0 {
		fmt.Printf("  %s\n", green("no circular imports detected ✓"))
	}
	for _, c := range cycles {
		fmt.Printf("  %s %s\n", red("✗"), cyan(strings.Join(c.Path, " → ")))
	}
	fmt.Println()
}

func todoTag(tag string) string {
	switch tag {
	case "FIXME":
		return bold(red("FIXME"))
	case "HACK":
		return bold(yellow("HACK "))
	case "OPTIMIZE":
		return bold(blue("OPTIM"))
	default:
		return bold(green("TODO "))
	}
}

func shorten(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func writeJSON(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", red("error:"), fmt.Sprintf(format, args...))
	os.Exit(1)
}
