// Package templates handles HTML template rendering for Datastar SSE responses.
package templates

import (
	"bytes"
	"html/template"
	"path/filepath"
	"sync"
	texttemplate "text/template"
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
// It also loads generated templates from the sibling generated/ directory.
func New(fragmentsDir string) (*Renderer, error) {
	tmpl, err := parseTemplates(fragmentsDir)
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

// RenderPage parses a page-level template file and renders it using
// the already-loaded fragments (so {{template "layer-form" .}} works).
// Uses text/template to avoid HTML-escaping of data-signals JSON attributes.
func (r *Renderer) RenderPage(pagePath string, data any) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Build a text/template that includes all fragment definitions
	tmpl := texttemplate.New("").Funcs(texttemplate.FuncMap(funcMap))

	// Re-parse fragment sources into a text/template so {{template "layer-form"}} works
	for _, t := range r.templates.Templates() {
		if t.Name() == "" {
			continue
		}
		// Clone each defined fragment by re-parsing its tree
		if _, err := tmpl.AddParseTree(t.Name(), t.Tree); err != nil {
			return "", err
		}
	}

	// Parse the page template
	if _, err := tmpl.ParseFiles(pagePath); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	name := filepath.Base(pagePath)
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Reload reloads templates from disk (useful for dev hot-reload).
func (r *Renderer) Reload(fragmentsDir string) error {
	tmpl, err := parseTemplates(fragmentsDir)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.templates = tmpl
	r.mu.Unlock()

	return nil
}

// parseTemplates loads hand-written fragments and generated templates.
func parseTemplates(fragmentsDir string) (*template.Template, error) {
	tmpl := template.New("").Funcs(funcMap)

	// Hand-written fragments
	pattern := filepath.Join(fragmentsDir, "*.html")
	tmpl, err := tmpl.ParseGlob(pattern)
	if err != nil {
		return nil, err
	}

	// Generated templates (sibling directory)
	genDir := filepath.Join(filepath.Dir(fragmentsDir), "generated")
	genPattern := filepath.Join(genDir, "*.html")
	// Ignore error — generated dir may not exist yet
	tmpl, _ = tmpl.ParseGlob(genPattern)

	return tmpl, nil
}
