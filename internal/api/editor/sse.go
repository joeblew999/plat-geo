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
// Used by both tiles and sources handlers.
type SelectOptionData struct {
	Value string
	Label string
}

// SSEContext wraps the Datastar SSE generator with helper methods.
type SSEContext struct {
	SSE *datastar.ServerSentEventGenerator
}

// NewSSEContext creates an SSE context from a Huma context.
func NewSSEContext(humaCtx huma.Context) *SSEContext {
	r, w := humago.Unwrap(humaCtx)
	return &SSEContext{
		SSE: datastar.NewSSE(w, r),
	}
}

// PatchElements sends HTML to replace content at a selector.
func (c *SSEContext) PatchElements(html, selector string) {
	c.SSE.PatchElements(html, datastar.WithSelector(selector), datastar.WithModeInner())
}

// SendError sends an error signal to the client.
func (c *SSEContext) SendError(msg string) {
	c.SSE.MarshalAndPatchSignals(map[string]any{
		"error": msg,
	})
}

// SendSuccess sends a success signal to the client.
func (c *SSEContext) SendSuccess(msg string) {
	c.SSE.MarshalAndPatchSignals(map[string]any{
		"success": msg,
	})
}

// SendSignals sends arbitrary signals to the client.
func (c *SSEContext) SendSignals(signals map[string]any) {
	c.SSE.MarshalAndPatchSignals(signals)
}
