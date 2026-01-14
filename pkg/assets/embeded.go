package assets

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var embeddedFS embed.FS

// Assets returns an http.FileSystem that serves embedded built assets.
// Use this in production builds to serve pre-built assets compiled into
// the binary via //go:embed.
func Assets() http.FileSystem {
	sub, err := fs.Sub(embeddedFS, "dist")
	if err != nil {
		// fall back to the root FS if sub fails (shouldn't happen when dist exists)
		return http.FS(embeddedFS)
	}
	return http.FS(sub)
}

// FileServer returns an http.Handler that serves the embedded assets.
// Example usage:
//
//	http.Handle("/assets/", http.StripPrefix("/assets/", assets.FileServer()))
func FileServer() http.Handler {
	return http.FileServer(Assets())
}
