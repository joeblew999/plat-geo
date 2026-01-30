// Package humastar bridges Huma (REST/OpenAPI) with Datastar (SSE/hypermedia).
//
// It provides:
//   - SSE: Huma streaming → Datastar SSE protocol via [SSE] and [NewSSE]
//   - Signals: Type-safe Datastar signal parsing via [Signals] and [SignalsInput]
//   - Rendering: Template list/select helpers via [RenderList] and [RenderSelect]
//   - Handler: Embeddable base for editor-style SSE handlers via [Handler]
//
// Usage:
//
//	type MyHandler struct {
//	    humastar.Handler
//	    myService *service.MyService
//	}
//
//	func (h *MyHandler) List(ctx context.Context, input *humastar.EmptyInput) (*huma.StreamResponse, error) {
//	    return h.Stream(func(sse humastar.SSE) {
//	        sse.Patch(h.RenderList("card", items, "Empty", "Nothing here"), "#my-list")
//	    }), nil
//	}
package humastar

import (
	"bytes"
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/starfederation/datastar-go/datastar"
)

// ---------------------------------------------------------------------------
// Handler — embeddable base for Datastar SSE handlers
// ---------------------------------------------------------------------------

// Handler is an embeddable base for Huma handlers that produce Datastar SSE
// responses. It holds a [templates.Renderer] and provides convenience methods
// to create streams and render templates.
type Handler struct {
	Renderer *Renderer
}

// Stream returns a Huma StreamResponse that calls fn with a ready SSE helper.
// Use this instead of manually constructing &huma.StreamResponse{Body: ...}.
func (h *Handler) Stream(fn func(sse SSE)) *huma.StreamResponse {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			fn(NewSSE(humaCtx))
		},
	}
}

// RenderList renders items with a named template, or an empty state if none.
func (h *Handler) RenderList(tmpl string, items []any, emptyTitle, emptyMsg string) string {
	return RenderList(h.Renderer, tmpl, items, emptyTitle, emptyMsg)
}

// RenderSelect renders select options from a placeholder and option list.
func (h *Handler) RenderSelect(placeholder string, options []SelectOptionData) string {
	return RenderSelect(h.Renderer, placeholder, options)
}

// ---------------------------------------------------------------------------
// SSE — Huma ↔ Datastar bridge
// ---------------------------------------------------------------------------

// SSE wraps a Datastar SSE generator with convenience methods for common
// patterns: error/success signals, inner/outer element patching.
type SSE struct {
	*datastar.ServerSentEventGenerator
}

// NewSSE creates a Datastar SSE helper from a Huma streaming context.
func NewSSE(ctx huma.Context) SSE {
	r, w := humago.Unwrap(ctx)
	return SSE{datastar.NewSSE(w, r)}
}

// Patch sends HTML to replace inner content at a CSS selector.
func (s SSE) Patch(html, selector string) {
	s.PatchElements(html,
		datastar.WithSelector(selector),
		datastar.WithModeInner(),
		datastar.WithViewTransitions(),
	)
}

// Replace replaces outer HTML at a CSS selector.
func (s SSE) Replace(html, selector string) {
	s.PatchElements(html,
		datastar.WithSelector(selector),
		datastar.WithModeOuter(),
		datastar.WithViewTransitions(),
	)
}

// Error sends an error signal to the UI.
func (s SSE) Error(msg string) {
	s.MarshalAndPatchSignals(map[string]any{"error": msg})
}

// Success sends a success signal to the UI.
func (s SSE) Success(msg string) {
	s.MarshalAndPatchSignals(map[string]any{"success": msg})
}

// Signals sends arbitrary signals to the UI.
func (s SSE) Signals(signals map[string]any) {
	s.MarshalAndPatchSignals(signals)
}

// ---------------------------------------------------------------------------
// Signals — Datastar signal parsing
// ---------------------------------------------------------------------------

// Signals provides type-safe access to Datastar signal values.
// Datastar sends all signals as a flat JSON object in the request body.
type Signals map[string]any

// ParseSignals parses Datastar signals from a raw request body.
func ParseSignals(body []byte) (Signals, error) {
	var signals Signals
	if err := json.Unmarshal(body, &signals); err != nil {
		return nil, err
	}
	return signals, nil
}

// String returns a string signal value, or empty string if not found.
func (s Signals) String(key string) string {
	if v, ok := s[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

// Int returns an int signal value, or 0 if not found.
func (s Signals) Int(key string) int {
	if v, ok := s[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}

// Float returns a float64 signal value, or 0 if not found.
func (s Signals) Float(key string) float64 {
	if v, ok := s[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

// Bool returns a bool signal value, or false if not found.
func (s Signals) Bool(key string) bool {
	if v, ok := s[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// Has returns true if the signal key exists (even if zero-valued).
func (s Signals) Has(key string) bool {
	_, ok := s[key]
	return ok
}

// ---------------------------------------------------------------------------
// Input types
// ---------------------------------------------------------------------------

// EmptyInput is a shared input struct for handlers with no parameters.
type EmptyInput struct{}

// SignalsInput is an input struct for handlers that receive Datastar signals.
type SignalsInput struct {
	RawBody []byte
}

// Parse parses the signals from the raw body.
func (i *SignalsInput) Parse() (Signals, error) {
	return ParseSignals(i.RawBody)
}

// MustParse parses signals or returns a Huma 400 error.
func (i *SignalsInput) MustParse() (Signals, error) {
	signals, err := ParseSignals(i.RawBody)
	if err != nil {
		return nil, huma.Error400BadRequest("Invalid request data: " + err.Error())
	}
	return signals, nil
}

// ---------------------------------------------------------------------------
// Rendering helpers
// ---------------------------------------------------------------------------

// SelectOptionData holds data for rendering a <select> option template.
type SelectOptionData struct {
	Value string
	Label string
}

// RenderList renders items with a named template, or an empty state if none.
func RenderList(r *Renderer, tmpl string, items []any, emptyTitle, emptyMsg string) string {
	var buf bytes.Buffer
	if len(items) == 0 {
		r.RenderToBuffer(&buf, "empty-state", map[string]string{
			"Title": emptyTitle, "Message": emptyMsg,
		})
	} else {
		for _, item := range items {
			r.RenderToBuffer(&buf, tmpl, item)
		}
	}
	return buf.String()
}

// RenderSelect renders <option> elements from a placeholder and option list.
func RenderSelect(r *Renderer, placeholder string, options []SelectOptionData) string {
	var buf bytes.Buffer
	r.RenderToBuffer(&buf, "select-option", SelectOptionData{Label: placeholder})
	for _, opt := range options {
		r.RenderToBuffer(&buf, "select-option", opt)
	}
	return buf.String()
}
