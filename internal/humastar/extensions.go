// extensions.go — Injects x-datastar extensions into OpenAPI schemas.
//
// At server startup, InjectExtensions walks registered schemas and adds:
//   - x-datastar (per-schema): prefix, formTemplate, goPkg, goOut
//   - x-signal, x-input, x-sse, x-card (per-property): from Go struct tags
//
// These extensions make the OpenAPI spec carry all Datastar metadata, so
// downstream consumers (codegen, form renderer, page data builder) read
// from the spec instead of re-walking struct tags.
package humastar

import (
	"reflect"

	"github.com/danielgtaylor/huma/v2"
)

// DatastarSchema defines per-schema Datastar codegen metadata.
// This is injected as the "x-datastar" extension on OpenAPI schemas.
type DatastarSchema struct {
	Prefix   string `json:"prefix"`       // Signal prefix (e.g. "newlayer")
	FormTmpl string `json:"formTemplate"` // HTML template name (e.g. "layer-form")
	GoPkg    string `json:"goPkg"`        // Go package for generated code
	GoOut    string `json:"goOut"`         // Output path for signals_gen.go
}

// DatastarSchemaConfig registers a Go type for Datastar codegen extensions.
type DatastarSchemaConfig struct {
	Type     reflect.Type
	Prefix   string   // Signal prefix (e.g. "newlayer")
	FormTmpl string   // Template name (e.g. "layer-form")
	BasePath string   // API path prefix for route discovery (e.g. "/api/v1/editor/layers")
	GoPkg    string   // Go package for generated code
	GoOut    string   // Output path for signals_gen.go
}

// InjectExtensions walks the OpenAPI schema registry and adds x-datastar,
// x-signal, x-input, and x-sse extensions from Go struct tags.
// Call after all routes are registered so schemas exist.
func InjectExtensions(api huma.API, configs []DatastarSchemaConfig) {
	schemas := api.OpenAPI().Components.Schemas.Map()

	for _, cfg := range configs {
		name := cfg.Type.Name()
		schema, ok := schemas[name]
		if !ok {
			continue
		}

		// Per-schema extension
		if schema.Extensions == nil {
			schema.Extensions = map[string]any{}
		}
		schema.Extensions["x-datastar"] = DatastarSchema{
			Prefix:   cfg.Prefix,
			FormTmpl: cfg.FormTmpl,
			GoPkg:    cfg.GoPkg,
			GoOut:    cfg.GoOut,
		}

		// Per-property extensions from struct tags
		injectPropertyExtensions(schema, cfg.Type)
	}
}

func injectPropertyExtensions(schema *huma.Schema, t reflect.Type) {
	for i := range t.NumField() {
		sf := t.Field(i)

		// Find the matching property in the schema
		jsonName := sf.Tag.Get("json")
		if idx := indexOf(jsonName, ','); idx >= 0 {
			jsonName = jsonName[:idx]
		}
		if jsonName == "" || jsonName == "-" {
			continue
		}

		prop, ok := schema.Properties[jsonName]
		if !ok {
			continue
		}

		// Collect custom tags
		ext := map[string]any{}

		if sig := sf.Tag.Get("signal"); sig != "" {
			ext["x-signal"] = sig
		}
		if inp := sf.Tag.Get("input"); inp != "" {
			ext["x-input"] = inp
		}
		if sse := sf.Tag.Get("sse"); sse != "" {
			ext["x-sse"] = sse
		}
		if card := sf.Tag.Get("card"); card != "" {
			ext["x-card"] = card
		}

		if len(ext) > 0 {
			if prop.Extensions == nil {
				prop.Extensions = map[string]any{}
			}
			for k, v := range ext {
				prop.Extensions[k] = v
			}
		}
	}
}

func indexOf(s string, c byte) int {
	for i := range len(s) {
		if s[i] == c {
			return i
		}
	}
	return -1
}
