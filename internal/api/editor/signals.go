// Package editor contains Datastar SSE handlers for the editor UI.
package editor

import (
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"
)

// Signals provides type-safe access to Datastar signal values.
// Datastar sends all signals as a flat JSON object in the request body.
// Signal names are lowercase due to data-bind behavior.
type Signals map[string]any

// ParseSignals parses Datastar signals from a raw request body.
// Use with Huma's RawBody []byte field to capture the body before streaming.
//
// Example Huma input struct:
//
//	type MyInput struct {
//	    RawBody []byte
//	}
//
// Example handler:
//
//	func (h *Handler) DoSomething(ctx context.Context, input *MyInput) (*huma.StreamResponse, error) {
//	    signals, err := editor.ParseSignals(input.RawBody)
//	    if err != nil {
//	        return nil, huma.Error400BadRequest("Invalid request: " + err.Error())
//	    }
//	    value := signals.String("fieldname")
//	    // ...
//	}
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
// Handles both float64 (JSON default) and int types.
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

// Has returns true if the signal exists (even if empty/zero).
func (s Signals) Has(key string) bool {
	_, ok := s[key]
	return ok
}

// SignalsInput is a reusable input struct for handlers that receive Datastar signals.
// Embed this in your handler input struct or use directly.
type SignalsInput struct {
	RawBody []byte
}

// Parse parses the signals from the raw body.
func (i *SignalsInput) Parse() (Signals, error) {
	return ParseSignals(i.RawBody)
}

// MustParse parses signals or returns a Huma error.
// Useful for returning early from handlers on parse failure.
func (i *SignalsInput) MustParse() (Signals, error) {
	signals, err := ParseSignals(i.RawBody)
	if err != nil {
		return nil, huma.Error400BadRequest("Invalid request data: " + err.Error())
	}
	return signals, nil
}
