package humastar

import "fmt"

// Action is a state-dependent hypermedia action link.
// Response bodies implement the Actor interface to emit conditional
// RFC 8288 Link headers with method, title, and schema extension parameters.
//
// Example Link header output:
//
//	<url>; rel="cancel"; method="POST"; title="Cancel order"; schema="/schemas/CancelInput.json"
type Action struct {
	Rel    string // IANA rel or custom (e.g., "cancel", "approve")
	Href   string // target URL
	Method string // HTTP method: POST, PUT, DELETE, etc.
	Title  string // optional human-readable label
	Schema string // optional JSON Schema URL for the request body
}

// Actor is implemented by response bodies that provide state-dependent actions.
type Actor interface {
	Actions() []Action
}

// LinkHeader formats the action as an RFC 8288 Link header value
// with method and title extension parameters.
func (a Action) LinkHeader() string {
	h := fmt.Sprintf(`<%s>; rel="%s"`, a.Href, a.Rel)
	if a.Method != "" {
		h += fmt.Sprintf(`; method="%s"`, a.Method)
	}
	if a.Title != "" {
		h += fmt.Sprintf(`; title="%s"`, a.Title)
	}
	if a.Schema != "" {
		h += fmt.Sprintf(`; schema="%s"`, a.Schema)
	}
	return h
}
