// resource.go — Reusable action definitions and resource helpers.
//
// ActionDef is a URL pattern template for actions (e.g. "/api/v1/layers/%s").
// ActionsFor generates concrete Action values from ActionDefs for a given
// resource ID, connecting resource.go → actions.go → Link headers.
package humastar

import "fmt"

// ActionDef is a reusable action template.
// Pattern uses a single %s verb for the resource ID.
type ActionDef struct {
	Rel     string // IANA or custom rel (e.g., "delete", "duplicate")
	Pattern string // URL pattern with %s placeholder (e.g., "/api/v1/layers/%s")
	Method  string // HTTP method: POST, PUT, DELETE, etc.
	Title   string // human-readable label
	Schema  string // optional JSON Schema URL for the request body
}

// ActionsFor generates concrete Action values from ActionDefs for a given resource ID.
func ActionsFor(id string, defs []ActionDef) []Action {
	actions := make([]Action, len(defs))
	for i, d := range defs {
		actions[i] = Action{
			Rel:    d.Rel,
			Href:   fmt.Sprintf(d.Pattern, id),
			Method: d.Method,
			Title:  d.Title,
			Schema: d.Schema,
		}
	}
	return actions
}
