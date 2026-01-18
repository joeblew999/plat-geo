package editor

import (
	"bytes"
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/plat-geo/internal/service"
	"github.com/joeblew999/plat-geo/internal/templates"
)

// SourceHandler handles source file-related SSE endpoints.
type SourceHandler struct {
	sourceService *service.SourceService
	renderer      *templates.Renderer
}

// NewSourceHandler creates a new source handler.
func NewSourceHandler(sourceService *service.SourceService, renderer *templates.Renderer) *SourceHandler {
	return &SourceHandler{
		sourceService: sourceService,
		renderer:      renderer,
	}
}

// RegisterRoutes registers source editor routes with Huma.
func (h *SourceHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/editor/sources", h.ListSources)
	huma.Get(api, "/api/v1/editor/sources/select", h.ListSourcesSelect)
}

// ListSources streams the source list as SSE HTML fragments.
func (h *SourceHandler) ListSources(ctx context.Context, input *EmptyInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSEContext(humaCtx)

			sources, err := h.sourceService.List()
			if err != nil {
				sse.SendError("Failed to list sources: " + err.Error())
				return
			}

			html := h.renderSourceList(sources)
			sse.PatchElements(html, "#source-list")
		},
	}, nil
}

// ListSourcesSelect streams sources as select options.
func (h *SourceHandler) ListSourcesSelect(ctx context.Context, input *EmptyInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSEContext(humaCtx)

			sources, err := h.sourceService.List()
			if err != nil {
				sse.SendError("Failed to list sources: " + err.Error())
				return
			}

			html := h.renderSourceSelect(sources)
			sse.PatchElements(html, "#source-select")
		},
	}, nil
}

// SourceCardData holds data for rendering a source card template.
type SourceCardData struct {
	Name     string
	Size     string
	FileType string
}

func (h *SourceHandler) renderSourceList(sources []service.SourceFile) string {
	var buf bytes.Buffer

	if len(sources) == 0 {
		if err := h.renderer.RenderToBuffer(&buf, "empty-state", map[string]string{
			"Title":   "No Source Files",
			"Message": "Upload GeoJSON or GeoParquet files using the form above.",
		}); err != nil {
			return "<!-- template error: " + err.Error() + " -->"
		}
	} else {
		for _, source := range sources {
			if err := h.renderer.RenderToBuffer(&buf, "source-card", SourceCardData{
				Name:     source.Name,
				Size:     source.Size,
				FileType: source.FileType,
			}); err != nil {
				buf.WriteString("<!-- template error: " + err.Error() + " -->")
			}
		}
	}

	return buf.String()
}

func (h *SourceHandler) renderSourceSelect(sources []service.SourceFile) string {
	var buf bytes.Buffer

	if err := h.renderer.RenderToBuffer(&buf, "select-option", SelectOptionData{
		Value: "",
		Label: "-- Select a source file --",
	}); err != nil {
		return "<!-- template error: " + err.Error() + " -->"
	}

	for _, source := range sources {
		if err := h.renderer.RenderToBuffer(&buf, "select-option", SelectOptionData{
			Value: source.Name,
			Label: source.Name + " (" + source.Size + ")",
		}); err != nil {
			buf.WriteString("<!-- template error: " + err.Error() + " -->")
		}
	}

	return buf.String()
}
