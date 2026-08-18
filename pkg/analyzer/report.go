package analyzer

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"time"
)

// Report is the full snapshot of a repository scan, exportable to JSON or HTML.
type Report struct {
	GeneratedAt  time.Time     `json:"generated_at"`
	Repo         string        `json:"repo"`
	Stats        RepoStats     `json:"stats"`
	Hotspots     []HotspotFile `json:"hotspots"`
	StaleFiles   []StaleFile   `json:"stale_files"`
	Todos        []TodoItem    `json:"todos"`
	ImportCycles []Cycle       `json:"import_cycles"`
	Packages     []PackageNode `json:"packages"`
}

// WriteJSON serializes the report as indented JSON.
func (r *Report) WriteJSON(w *os.File) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteHTML renders the report as a self-contained dashboard page.
func (r *Report) WriteHTML(path string) error {
	type langRow struct {
		Ext   string
		Count int
	}
	var langs []langRow
	for ext, count := range r.Stats.Languages {
		langs = append(langs, langRow{ext, count})
	}
	sort.Slice(langs, func(i, j int) bool { return langs[i].Count > langs[j].Count })

	total := 0
	for _, l := range langs {
		total += l.Count
	}

	data := struct {
		Report
		Langs []langRow
	}{*r, langs}

	funcs := template.FuncMap{
		"lower": func(s string) string { return strings.ToLower(s) },
		"pct":   func(count int) string { return fmt.Sprintf("%.0f", float64(count)/float64(max(total, 1))*100) },
	}

	tmpl, err := template.New("report").Funcs(funcs).Parse(htmlTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// summary computes headline numbers for the report banner.
func (r Report) Summary() string {
	return fmt.Sprintf(
		"%d files / %d lines / %d packages / %d TODOs / %d hotspots / %d stale / %d cycles",
		r.Stats.TotalFiles, r.Stats.TotalLines, len(r.Packages),
		len(r.Todos), len(r.Hotspots), len(r.StaleFiles), len(r.ImportCycles),
	)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>GitDrift Report — {{.Repo}}</title>
<style>
  :root { --bg:#0d1117; --card:#161b22; --border:#30363d; --fg:#c9d1d9; --muted:#8b949e;
          --accent:#58a6ff; --warn:#d29922; --danger:#f85149; --ok:#3fb950; }
  * { box-sizing:border-box; margin:0; padding:0; }
  body { background:var(--bg); color:var(--fg); font:14px/1.6 "Segoe UI", system-ui, sans-serif; padding:32px; }
  h1 { font-size:22px; margin-bottom:4px; }
  .sub { color:var(--muted); margin-bottom:24px; }
  .grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(170px,1fr)); gap:12px; margin-bottom:28px; }
  .card { background:var(--card); border:1px solid var(--border); border-radius:8px; padding:14px; }
  .card .num { font-size:26px; font-weight:700; }
  .card .lbl { color:var(--muted); font-size:12px; text-transform:uppercase; letter-spacing:.5px; }
  h2 { font-size:16px; margin:22px 0 10px; color:var(--accent); }
  table { width:100%; border-collapse:collapse; background:var(--card); border:1px solid var(--border); border-radius:8px; overflow:hidden; }
  th,td { text-align:left; padding:8px 12px; border-bottom:1px solid var(--border); font-size:13px; }
  th { background:#21262d; color:var(--muted); font-size:11px; text-transform:uppercase; letter-spacing:.5px; }
  tr:last-child td { border-bottom:none; }
  .tag { display:inline-block; padding:1px 8px; border-radius:10px; font-size:11px; font-weight:600; }
  .tag.todo{ background:rgba(210,153,34,.15); color:var(--warn); }
  .tag.fixme{ background:rgba(248,81,73,.15); color:var(--danger); }
  .tag.hack{ background:rgba(210,153,34,.15); color:var(--warn); }
  .tag.optimize{ background:rgba(88,166,255,.15); color:var(--accent); }
  .ok { color:var(--ok); } .warn { color:var(--warn); } .danger { color:var(--danger); }
  code { font-family:Consolas,monospace; font-size:12px; color:var(--accent); }
  .bar { height:6px; background:var(--border); border-radius:3px; overflow:hidden; }
  .bar span { display:block; height:100%; background:var(--accent); }
</style>
</head>
<body>
  <h1>GitDrift — {{.Repo}}</h1>
  <div class="sub">Generated {{.GeneratedAt.Format "Jan 02, 2006 15:04"}} &middot; {{.Summary}}</div>

  <div class="grid">
    <div class="card"><div class="num">{{.Stats.TotalFiles}}</div><div class="lbl">Files</div></div>
    <div class="card"><div class="num">{{.Stats.TotalLines}}</div><div class="lbl">Lines</div></div>
    <div class="card"><div class="num">{{len .Packages}}</div><div class="lbl">Packages</div></div>
    <div class="card"><div class="num">{{len .Todos}}</div><div class="lbl">TODOs</div></div>
    <div class="card"><div class="num">{{len .Hotspots}}</div><div class="lbl">Hotspots</div></div>
    <div class="card"><div class="num">{{len .StaleFiles}}</div><div class="lbl">Stale files</div></div>
    <div class="card"><div class="num {{if .ImportCycles}}danger{{else}}ok{{end}}">{{len .ImportCycles}}</div><div class="lbl">Import cycles</div></div>
  </div>

  <h2>Languages</h2>
  <table>
    <tr><th>Extension</th><th>Files</th><th>Share</th></tr>
    {{range .Langs}}
    <tr><td><code>{{.Ext}}</code></td><td>{{.Count}}</td>
        <td><div class="bar"><span style="width:{{pct .Count}}%"></span></div></td></tr>
    {{end}}
  </table>

  {{if .Hotspots}}
  <h2>High-Churn Hotspots</h2>
  <table><tr><th>Commits</th><th>File</th></tr>
    {{range .Hotspots}}<tr><td>{{.CommitCount}}</td><td><code>{{.Path}}</code></td></tr>{{end}}
  </table>
  {{end}}

  {{if .StaleFiles}}
  <h2>Stale Files</h2>
  <table><tr><th>Days idle</th><th>File</th></tr>
    {{range .StaleFiles}}<tr><td class="warn">{{.DaysIdle}}</td><td><code>{{.Path}}</code></td></tr>{{end}}
  </table>
  {{end}}

  {{if .Todos}}
  <h2>Tech Debt</h2>
  <table><tr><th>Tag</th><th>Location</th><th>Content</th></tr>
    {{range .Todos}}
    <tr><td><span class="tag {{lower .Tag}}">{{.Tag}}</span></td><td><code>{{.File}}:{{.Line}}</code></td><td>{{.Content}}</td></tr>
    {{end}}
  </table>
  {{end}}

  {{if .ImportCycles}}
  <h2>Circular Imports</h2>
  <table><tr><th>Cycle</th></tr>
    {{range .ImportCycles}}<tr><td class="danger">{{range $i, $p := .Path}}{{if $i}} → {{end}}<code>{{$p}}</code>{{end}}</td></tr>{{end}}
  </table>
  {{end}}
</body>
</html>`
