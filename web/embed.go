// Package web embeds the built frontend so the binary can serve it.
package web

import (
	"embed"
	"io/fs"
)

// dist holds the frontend build output. Only .gitkeep is committed, so a
// fresh clone still compiles; the server shows a "not built" page until
// the frontend is built.
//
//go:embed all:dist
var dist embed.FS

// Dist returns the built frontend, rooted at the folder holding index.html.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // only fails for an invalid path, which "dist" is not
	}
	return sub
}
