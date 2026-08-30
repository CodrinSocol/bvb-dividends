// Package openapi serves the generated API specification and a rendering of it.
package openapi

import (
	"embed"
	"html/template"
	"net/http"
)

// The paths the specification and its rendering are served on. They sit under
// the API prefix, so everything outside it belongs to the web application.
const (
	DocsPath = "/api/docs"
	SpecPath = DocsPath + "/openapi.yaml"
)

var (
	//go:embed view/*
	views embed.FS
	tmpl  = template.Must(template.ParseFS(views, "view/index.html.tmpl"))
)

// Middleware is an OpenAPI middleware.
type Middleware func(http.Handler) http.Handler

// Spec is the generated OpenAPI document.
type Spec []byte

// NewMiddleware serves spec as YAML, and a browsable rendering of it.
//
// Both are reachable without any credential, because they are opened directly
// in a browser.
func NewMiddleware(spec Spec) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)

				return
			}

			switch r.URL.Path {
			case SpecPath:
				w.Header().Set("Content-Type", "application/yaml")
				w.Header().Set("Cache-Control", "public, max-age=300")
				_, _ = w.Write(spec)
			case DocsPath:
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_ = tmpl.Execute(w, map[string]string{"SpecPath": SpecPath})
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}
