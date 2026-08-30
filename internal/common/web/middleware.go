// Package web serves the built single-page application.
package web

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
)

// indexFile is the document every unknown path falls back to, so that a deep
// link such as /companies/SNP survives a reload.
const indexFile = "index.html"

// Middleware is a static-assets middleware.
type Middleware func(http.Handler) http.Handler

// NewMiddleware serves assets from files, falling back to the application's
// index document.
//
// It is mounted last, after the API and the health checks, so that it only ever
// sees a path none of them claimed. When the build is missing — which is what
// an unbuilt working copy looks like, since the binary embeds a directory that
// has to exist to compile — it answers with a note saying so rather than with a
// blank page.
func NewMiddleware(files fs.FS, log *slog.Logger, reserved ...string) Middleware {
	index, err := fs.ReadFile(files, indexFile)
	if err != nil {
		log.Warn("the web application was not built into this binary; only the API is served",
			slog.Any("err", err))

		return func(next http.Handler) http.Handler {
			return unbuiltHandler(next, reserved)
		}
	}

	fileServer := http.FileServerFS(files)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isReserved(r.URL.Path, reserved) ||
				(r.Method != http.MethodGet && r.Method != http.MethodHead) {
				next.ServeHTTP(w, r)

				return
			}

			name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
			if name == "" {
				name = indexFile
			}

			if _, err := fs.Stat(files, name); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					log.ErrorContext(r.Context(), "reading a web asset",
						slog.String("path", name), slog.Any("err", err))
				}

				// An unknown path is a client-side route, not a missing file.
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(index)

				return
			}

			fileServer.ServeHTTP(w, r)
		})
	}
}

// isReserved reports whether a path belongs to something mounted before the
// web application - the API, in practice - and so must be passed through
// rather than answered with the index document.
func isReserved(urlPath string, reserved []string) bool {
	for _, prefix := range reserved {
		if strings.HasPrefix(urlPath, prefix) {
			return true
		}
	}

	return false
}

// unbuiltHandler explains the absence of the web build to anyone who asks for
// a page, and leaves every other request alone.
func unbuiltHandler(next http.Handler, reserved []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isReserved(r.URL.Path, reserved) || r.Method != http.MethodGet {
			next.ServeHTTP(w, r)

			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("the web application was not built into this binary; " +
			"run `bunx nx run web:build` and rebuild\n"))
	})
}
