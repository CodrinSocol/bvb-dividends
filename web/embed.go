// Package web carries the built single-page application into the binary.
//
// The embed lives here rather than next to the server because a go:embed
// directive cannot reach outside its own package directory, and the build has
// to be embedded from where vite writes it.
package web

import (
	"embed"
	"io/fs"
)

// dist is the vite build. The all: prefix includes the dot-files vite emits.
//
//go:embed all:dist
var dist embed.FS

// FS returns the built application, rooted at its index document.
//
// The directory exists in the repository with only a placeholder in it, so that
// the binary compiles in a working copy where the frontend has not been built;
// the server says so rather than serving a blank page when the index is
// missing.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// dist is embedded above, so this cannot fail at run time.
		panic("web: " + err.Error())
	}

	return sub
}
