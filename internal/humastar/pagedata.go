// pagedata.go — Reverse mapping: OpenAPI spec → page template data.
//
// BuildPageData extracts everything a page template needs from the spec:
//   - Signals JSON (data-signals init from schema defaults + UI state)
//   - Routes (Create, List, Get, Update, Delete, Events — discovered from OpenAPI paths)
//   - FormTmpl name (for {{template "layer-form" .}})
//
// This is the reverse of formrender.go: instead of spec → HTML fields,
// it's spec → template variables so the HTML never hardcodes URLs or signal names.
package humastar

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// PageData holds everything a page template needs from the OpenAPI spec.
// Templates use {{.Signals}} for data-signals init, {{.Routes.Create}} for
// form actions, etc. — no hardcoded URLs or signal names in HTML.
type PageData struct {
	// Signals is the JSON string for data-signals initialization.
	// Includes schema reset values + UI state signals.
	Signals string

	// Routes maps operation names to their paths.
	// e.g. Routes.List = "/api/v1/editor/layers"
	Routes SchemaRoutes

	// SSEInits holds SSE endpoint URLs from x-sse property extensions.
	SSEInits []string

	// FormTmpl is the template name for the form fragment.
	FormTmpl string
}

// SchemaRoutes holds discovered API routes for a resource.
type SchemaRoutes struct {
	List   string // GET collection
	Create string // POST collection
	Get    string // GET single
	Update string // PUT single
	Delete string // DELETE single
	Events string // GET SSE events stream
}

// DataInit returns a Datastar data-init attribute value joining all SSE init URLs.
// e.g. "@get('/api/v1/editor/tiles/select')"
func (pd PageData) DataInit() string {
	var parts []string
	for _, url := range pd.SSEInits {
		parts = append(parts, fmt.Sprintf("@get('%s')", url))
	}
	return strings.Join(parts, " ")
}

// BuildPageData builds template data for a schema from the OpenAPI spec.
// It discovers routes by matching paths that use operations tagged with the
// given tag, and builds the signals JSON from schema defaults + extra UI signals.
func BuildPageData(api huma.API, cfg DatastarSchemaConfig, uiSignals map[string]any) PageData {
	pd := PageData{
		FormTmpl: cfg.FormTmpl,
	}

	// Build signals: schema defaults + UI state
	signals := buildResetSignals(api, cfg)
	for k, v := range uiSignals {
		signals[k] = v
	}
	signalsJSON, _ := json.Marshal(signals)
	pd.Signals = string(signalsJSON)

	// Discover routes from OpenAPI paths
	pd.Routes = discoverRoutes(api, cfg)

	// Collect SSE init URLs from x-sse property extensions
	pd.SSEInits = discoverSSEInits(api, cfg)

	return pd
}

// buildResetSignals produces the initial signal values from the OpenAPI schema.
// This is the runtime equivalent of the generated ResetXxxSignals() functions.
func buildResetSignals(api huma.API, cfg DatastarSchemaConfig) map[string]any {
	schemas := api.OpenAPI().Components.Schemas.Map()
	schema, ok := schemas[cfg.Type.Name()]
	if !ok {
		return map[string]any{}
	}

	signals := map[string]any{}
	t := cfg.Type

	for i := range t.NumField() {
		sf := t.Field(i)

		jsonName := sf.Tag.Get("json")
		if idx := strings.IndexByte(jsonName, ','); idx >= 0 {
			jsonName = jsonName[:idx]
		}
		if jsonName == "" || jsonName == "-" {
			continue
		}

		prop, ok := schema.Properties[jsonName]
		if !ok {
			continue
		}

		// Skip non-primitive
		if prop.Type == "array" || prop.Type == "object" {
			continue
		}
		// Skip ID
		if sf.Name == "ID" {
			continue
		}

		// Signal name
		suffix := strings.ToLower(jsonName)
		if sig, ok := prop.Extensions["x-signal"]; ok {
			suffix = fmt.Sprint(sig)
		}
		signal := cfg.Prefix + suffix

		// Default value
		if prop.Default != nil {
			signals[signal] = prop.Default
		} else {
			switch prop.Type {
			case "boolean":
				signals[signal] = false
			case "number", "integer":
				signals[signal] = 0
			default:
				signals[signal] = ""
			}
		}
	}

	return signals
}

// discoverRoutes finds API routes for a resource by walking OpenAPI paths.
// Scopes to paths matching cfg.BasePath (e.g. "/api/v1/editor/layers").
// Also discovers the events endpoint at the sibling /events path.
func discoverRoutes(api huma.API, cfg DatastarSchemaConfig) SchemaRoutes {
	var routes SchemaRoutes

	paths := api.OpenAPI().Paths
	if paths == nil || cfg.BasePath == "" {
		return routes
	}

	// Events endpoint: sibling path (e.g. /api/v1/editor/events)
	parent := cfg.BasePath[:strings.LastIndex(cfg.BasePath, "/")]
	eventsPath := parent + "/events"

	for path, item := range paths {
		if path == eventsPath && item.Get != nil {
			routes.Events = path
		}

		if !strings.HasPrefix(path, cfg.BasePath) {
			continue
		}

		suffix := path[len(cfg.BasePath):]
		hasParam := strings.Contains(suffix, "{")

		if item.Get != nil {
			if hasParam {
				routes.Get = path
			} else {
				routes.List = path
			}
		}
		if item.Post != nil && !hasParam {
			routes.Create = path
		}
		if item.Put != nil && hasParam {
			routes.Update = path
		}
		if item.Delete != nil && hasParam {
			routes.Delete = path
		}
	}

	return routes
}

// discoverSSEInits collects SSE endpoint URLs from x-sse property extensions.
// x-sse format: "/api/v1/editor/tiles/select,pmtiles-select" → URL is first part.
func discoverSSEInits(api huma.API, cfg DatastarSchemaConfig) []string {
	schemas := api.OpenAPI().Components.Schemas.Map()
	schema, ok := schemas[cfg.Type.Name()]
	if !ok {
		return nil
	}

	var urls []string
	for _, prop := range schema.Properties {
		if prop.Extensions == nil {
			continue
		}
		xSSE, ok := prop.Extensions["x-sse"].(string)
		if !ok || xSSE == "" {
			continue
		}
		url := xSSE
		if idx := strings.IndexByte(xSSE, ','); idx >= 0 {
			url = xSSE[:idx]
		}
		urls = append(urls, url)
	}
	return urls
}

