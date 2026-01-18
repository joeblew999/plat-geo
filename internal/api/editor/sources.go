package editor

import (
	"bytes"
	"context"
	"mime/multipart"

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
	huma.Get(api, "/api/v1/editor/sources/list", h.ListSources) // Alias for Datastar compatibility
	huma.Get(api, "/api/v1/editor/sources/select", h.ListSourcesSelect)
	huma.Post(api, "/api/v1/editor/sources/upload", h.Upload)
	huma.Delete(api, "/api/v1/editor/sources/{filename}", h.Delete)
}

// SourceUploadInput is the input for file upload.
type SourceUploadInput struct {
	RawBody multipart.Form
}

// Upload handles source file uploads.
func (h *SourceHandler) Upload(ctx context.Context, input *SourceUploadInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSEContext(humaCtx)

			// Get the file from the multipart form
			files := input.RawBody.File["file"]
			if len(files) == 0 {
				sse.SendError("No file provided")
				return
			}

			fileHeader := files[0]
			file, err := fileHeader.Open()
			if err != nil {
				sse.SendError("Failed to open uploaded file")
				return
			}
			defer file.Close()

			// Validate and save
			if err := h.sourceService.Save(fileHeader.Filename, file); err != nil {
				sse.SendError(err.Error())
				return
			}

			sse.SendSuccess("File uploaded: " + fileHeader.Filename)

			// Refresh source list
			sources, err := h.sourceService.List()
			if err == nil {
				html := h.renderSourceList(sources)
				sse.PatchElements(html, "#source-list")

				// Also refresh source select dropdown
				selectHtml := h.renderSourceSelect(sources)
				sse.PatchElements(selectHtml, "#source-select")
			}
		},
	}, nil
}

// SourceDeleteInput is the input for deleting a source file.
type SourceDeleteInput struct {
	Filename string `path:"filename" doc:"Source filename to delete"`
}

// Delete removes a source file.
func (h *SourceHandler) Delete(ctx context.Context, input *SourceDeleteInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSEContext(humaCtx)

			if err := h.sourceService.Delete(input.Filename); err != nil {
				sse.SendError(err.Error())
				return
			}

			sse.SendSuccess("Deleted: " + input.Filename)

			// Refresh source list
			sources, err := h.sourceService.List()
			if err == nil {
				html := h.renderSourceList(sources)
				sse.PatchElements(html, "#source-list")

				// Also refresh source select dropdown
				selectHtml := h.renderSourceSelect(sources)
				sse.PatchElements(selectHtml, "#source-select")
			}
		},
	}, nil
}

// ListSources streams the source list as SSE HTML fragments.
// Also updates the source-select dropdown for consistency.
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

			// Also update the source select dropdown
			selectHtml := h.renderSourceSelect(sources)
			sse.PatchElements(selectHtml, "#source-select")
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
