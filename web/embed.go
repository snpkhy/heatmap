// Package web embeds the production build of the React UI (web/dist) into the
// Go binary, so `heatmap serve` can serve it without extra files.
// Run `pnpm build` in web/ (or `make web-build`) to refresh dist/.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built UI files with the dist/ prefix stripped.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// Unreachable: "dist" is a valid path embedded at compile time.
		panic(err)
	}
	return sub
}
