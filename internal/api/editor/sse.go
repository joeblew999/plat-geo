// Package editor contains Datastar SSE handlers for the editor UI.
package editor

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/starfederation/datastar-go/datastar"
)

// EmptyInput is a shared empty input struct for handlers with no parameters.
type EmptyInput struct{}

// SelectOptionData holds data for rendering a select option template.
type SelectOptionData struct {
	Value string
	Label string
}

// SSEHelper wraps a Datastar SSE generator with convenience methods
// for common editor patterns (error/success signals, element patching).
type SSEHelper struct {
	*datastar.ServerSentEventGenerator
}

// NewSSE creates a Datastar SSE helper from a Huma context.
// Bridges Huma's streaming response with Datastar's SSE protocol.
func NewSSE(ctx huma.Context) SSEHelper {
	r, w := humago.Unwrap(ctx)
	return SSEHelper{datastar.NewSSE(w, r)}
}

// Patch sends HTML to replace inner content at a selector with view transitions.
func (s SSEHelper) Patch(html, selector string) {
	s.PatchElements(html,
		datastar.WithSelector(selector),
		datastar.WithModeInner(),
		datastar.WithViewTransitions(),
	)
}

// Replace replaces outer HTML at a selector with view transitions.
func (s SSEHelper) Replace(html, selector string) {
	s.PatchElements(html,
		datastar.WithSelector(selector),
		datastar.WithModeOuter(),
		datastar.WithViewTransitions(),
	)
}

// Error sends an error signal.
func (s SSEHelper) Error(msg string) {
	s.MarshalAndPatchSignals(map[string]any{"error": msg})
}

// Success sends a success signal.
func (s SSEHelper) Success(msg string) {
	s.MarshalAndPatchSignals(map[string]any{"success": msg})
}

// Signals sends arbitrary signals.
func (s SSEHelper) Signals(signals map[string]any) {
	s.MarshalAndPatchSignals(signals)
}
