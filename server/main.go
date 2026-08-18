package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed webui/index.html
var uiFS embed.FS

func main() {
	// Render and other PaaS providers inject PORT; fall back to :8080 locally.
	defaultAddr := ":" + os.Getenv("PORT")
	if defaultAddr == ":" {
		defaultAddr = ":8080"
	}

	addr := flag.String("addr", defaultAddr, "listen address")
	maxJobs := flag.Int("max-jobs", 20, "max concurrent jobs retained")
	ttl := flag.Duration("ttl", 30*time.Minute, "job retention window")
	cloneTimeout := flag.Duration("clone-timeout", 180*time.Second, "max clone time")
	flag.Parse()

	sub, err := fs.Sub(uiFS, ".")
	if err != nil {
		log.Fatalf("embed ui: %v", err)
	}

	store := NewJobStore(*maxJobs, *ttl)
	engine := &Engine{CloneTimeout: *cloneTimeout}
	handler := NewHandler(store, engine, sub)

	fmt.Printf("GitDrift web console listening on http://localhost%s\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, handler))
}
