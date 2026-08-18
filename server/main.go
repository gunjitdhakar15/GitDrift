package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"
)

//go:embed webui/index.html
var uiFS embed.FS

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	maxJobs := flag.Int("max-jobs", 20, "max concurrent jobs retained")
	ttl := flag.Duration("ttl", 30*time.Minute, "job retention window")
	cloneTimeout := flag.Duration("clone-timeout", 120*time.Second, "max clone time")
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
