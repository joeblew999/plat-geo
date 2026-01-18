// Package templates handles HTML template rendering for Datastar SSE responses.
package templates

import (
	"bytes"
	"html/template"
	"path/filepath"
	"sync"
)

// funcMap provides common template functions.
var funcMap = template.FuncMap{
	// dict creates a map from key-value pairs, useful for passing multiple values to nested templates
	"dict": func(values ...any) map[string]any {
		if len(values)%2 != 0 {
			return nil
		}
		m := make(map[string]any, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			key, ok := values[i].(string)
			if !ok {
				continue
			}
			m[key] = values[i+1]
		}
		return m
	},
}

// Renderer manages HTML fragment templates.
type Renderer struct {
	templates *template.Template
	mu        sync.RWMutex
}

// New creates a new template renderer.
// fragmentsDir should be the path to web/templates/fragments/
func New(fragmentsDir string) (*Renderer, error) {
	pattern := filepath.Join(fragmentsDir, "*.html")
	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(pattern)
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: tmpl}, nil
}

// Render renders a named template to a string.
func (r *Renderer) Render(name string, data any) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderToBuffer renders a named template to a buffer.
func (r *Renderer) RenderToBuffer(buf *bytes.Buffer, name string, data any) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.templates.ExecuteTemplate(buf, name, data)
}

// MustRender renders a template and panics on error.
// Use only when you're certain the template exists.
func (r *Renderer) MustRender(name string, data any) string {
	s, err := r.Render(name, data)
	if err != nil {
		panic(err)
	}
	return s
}

// Reload reloads templates from disk (useful for dev hot-reload).
func (r *Renderer) Reload(fragmentsDir string) error {
	pattern := filepath.Join(fragmentsDir, "*.html")
	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(pattern)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.templates = tmpl
	r.mu.Unlock()

	return nil
}
