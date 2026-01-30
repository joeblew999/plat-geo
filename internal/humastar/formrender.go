// formrender.go — Runtime HTML form generation from OpenAPI schemas.
//
// At server startup, RegisterFormTemplates walks schemas with x-datastar
// extensions and builds Datastar-bound HTML form fragments:
//
//	string           → <input type="text">
//	string + enum    → <select> with options
//	string + x-input:"color" → color picker + text input
//	string + x-input:"sse"   → <select> with SSE loading
//	boolean          → <input type="checkbox">
//	number/integer   → <input type="number"> with min/max/step
//
// Each form is registered as a named Go template (e.g. "layer-form") in
// the Renderer, replacing static generated HTML files.
package humastar

import (
	"fmt"
	"html/template"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterFormTemplates walks OpenAPI schemas with x-datastar extensions and
// registers dynamic form templates in the Renderer. This replaces static
// generated HTML forms — the server renders forms at runtime from the spec.
//
// Call after InjectExtensions and before serving pages.
func RegisterFormTemplates(api huma.API, r *Renderer) {
	schemas := api.OpenAPI().Components.Schemas.Map()

	for name, schema := range schemas {
		ds, ok := schema.Extensions["x-datastar"]
		if !ok {
			continue
		}
		dsMeta, ok := ds.(DatastarSchema)
		if !ok {
			continue
		}
		if dsMeta.FormTmpl == "" {
			continue
		}

		html := renderFormHTML(name, schema, dsMeta)
		tmplText := fmt.Sprintf(`{{define "%s"}}%s{{end}}`, dsMeta.FormTmpl, html)

		r.mu.Lock()
		template.Must(r.templates.Parse(tmplText))
		r.mu.Unlock()
	}
}

// renderFormHTML builds the HTML form groups for a schema.
func renderFormHTML(name string, schema *huma.Schema, ds DatastarSchema) string {
	var b strings.Builder

	// Walk properties in required-first order, then alphabetical
	propNames := sortedPropertyNames(schema)

	for _, jsonName := range propNames {
		prop := schema.Properties[jsonName]

		// Skip $schema (OpenAPI meta-property)
		if strings.HasPrefix(jsonName, "$") {
			continue
		}
		// Skip non-primitive types (arrays, objects)
		if prop.Type == "array" || prop.Type == "object" {
			continue
		}
		// Skip ID — not a form field
		xCard, _ := prop.Extensions["x-card"].(string)
		if xCard == "id" {
			continue
		}

		// Signal name: prefix + (x-signal override or lowercase json name)
		suffix := strings.ToLower(jsonName)
		if sig, ok := prop.Extensions["x-signal"]; ok {
			suffix = fmt.Sprint(sig)
		}
		signal := ds.Prefix + suffix

		required := slices.Contains(schema.Required, jsonName)
		label := prop.Description
		if label == "" {
			label = jsonName
		}

		xInput, _ := prop.Extensions["x-input"].(string)

		switch {
		case prop.Type == "boolean":
			renderCheckbox(&b, label, signal, required)

		case xInput == "color":
			renderColorPicker(&b, label, signal, prop, required)

		case xInput == "sse":
			xSSE, _ := prop.Extensions["x-sse"].(string)
			renderSSESelect(&b, label, signal, xSSE, required)

		case len(prop.Enum) > 0:
			renderEnumSelect(&b, label, signal, prop, required)

		case prop.Type == "number" || prop.Type == "integer":
			renderNumberInput(&b, label, signal, prop, required)

		default: // string text input
			renderTextInput(&b, label, signal, prop, required)
		}
	}

	return b.String()
}

func renderTextInput(b *strings.Builder, label, signal string, prop *huma.Schema, required bool) {
	b.WriteString(`<div class="form-group">`)
	fmt.Fprintf(b, "\n    <label>%s</label>\n", label)
	fmt.Fprintf(b, `    <input type="text" data-bind:%s`, signal)
	if prop.Default != nil {
		fmt.Fprintf(b, ` placeholder="%v"`, prop.Default)
	}
	if required {
		b.WriteString(` required`)
	}
	b.WriteString(">\n</div>\n")
}

func renderNumberInput(b *strings.Builder, label, signal string, prop *huma.Schema, required bool) {
	b.WriteString(`<div class="form-group">`)
	fmt.Fprintf(b, "\n    <label>%s</label>\n", label)
	fmt.Fprintf(b, `    <input type="number" data-bind:%s`, signal)
	if prop.Minimum != nil {
		fmt.Fprintf(b, ` min="%v"`, *prop.Minimum)
	}
	if prop.Maximum != nil {
		fmt.Fprintf(b, ` max="%v"`, *prop.Maximum)
	}
	// Step: use 0.1 for floats, 1 for integers
	if prop.Type == "number" {
		b.WriteString(` step="0.1"`)
	}
	if prop.Default != nil {
		fmt.Fprintf(b, ` placeholder="%v"`, prop.Default)
	}
	if required {
		b.WriteString(` required`)
	}
	b.WriteString(">\n</div>\n")
}

func renderCheckbox(b *strings.Builder, label, signal string, _ bool) {
	b.WriteString(`<div class="form-group">`)
	// Checkboxes: unchecked is a valid state, never mark required
	fmt.Fprintf(b, "\n    <label><input type=\"checkbox\" data-bind:%s> %s</label>\n</div>\n", signal, label)
}

func renderColorPicker(b *strings.Builder, label, signal string, prop *huma.Schema, required bool) {
	b.WriteString(`<div class="form-group">`)
	fmt.Fprintf(b, "\n    <label>%s</label>\n", label)
	b.WriteString(`    <div class="color-group">`)
	fmt.Fprintf(b, "\n        <input type=\"color\" data-bind:%s>\n", signal)
	fmt.Fprintf(b, `        <input type="text" data-bind:%s`, signal)
	if prop.Default != nil {
		fmt.Fprintf(b, ` placeholder="%v"`, prop.Default)
	}
	b.WriteString(">\n    </div>\n</div>\n")
}

func renderEnumSelect(b *strings.Builder, label, signal string, prop *huma.Schema, required bool) {
	b.WriteString(`<div class="form-group">`)
	fmt.Fprintf(b, "\n    <label>%s</label>\n", label)
	fmt.Fprintf(b, `    <select data-bind:%s`, signal)
	if required {
		b.WriteString(` required`)
	}
	b.WriteString(">\n")
	for _, v := range prop.Enum {
		fmt.Fprintf(b, "        <option value=\"%v\">%v</option>\n", v, v)
	}
	b.WriteString("    </select>\n</div>\n")
}

func renderSSESelect(b *strings.Builder, label, signal, xSSE string, required bool) {
	// xSSE format: "/api/v1/editor/tiles/select,pmtiles-select"
	parts := strings.SplitN(xSSE, ",", 2)
	elementID := ""
	if len(parts) == 2 {
		elementID = strings.TrimSpace(parts[1])
	}

	b.WriteString(`<div class="form-group">`)
	fmt.Fprintf(b, "\n    <label>%s</label>\n", label)
	fmt.Fprintf(b, `    <select data-bind:%s`, signal)
	if required {
		b.WriteString(` required`)
	}
	if elementID != "" {
		fmt.Fprintf(b, ` id="%s"`, elementID)
	}
	b.WriteString(">\n        <option value=\"\">Loading...</option>\n    </select>\n</div>\n")
}

// sortedPropertyNames returns property names: required first, then optional, both alphabetical.
func sortedPropertyNames(schema *huma.Schema) []string {
	var req, opt []string
	for name := range schema.Properties {
		if slices.Contains(schema.Required, name) {
			req = append(req, name)
		} else {
			opt = append(opt, name)
		}
	}
	slices.Sort(req)
	slices.Sort(opt)
	return append(req, opt...)
}
