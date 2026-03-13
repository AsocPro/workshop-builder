package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed frontend/dist
var frontendFS embed.FS

// frontendHandler returns an HTTP handler serving embedded frontend assets.
// For non-asset paths (no file extension or not found), serves index.html
// to support client-side routing (SPA fallback).
func frontendHandler() http.Handler {
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try to open the file
		f, err := distFS.Open(path)
		if err != nil {
			// Fallback: serve index.html for SPA routing
			data, err := fs.ReadFile(distFS, "index.html")
			if err != nil {
				http.Error(w, "index.html not found in embedded assets", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}
		f.Close()

		fileServer.ServeHTTP(w, r)
	})
}
