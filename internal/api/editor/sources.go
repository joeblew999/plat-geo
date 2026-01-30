package editor

import (
	"context"
	"mime/multipart"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/plat-geo/internal/humastar"
	"github.com/joeblew999/plat-geo/internal/service"
)

type SourceHandler struct {
	humastar.Handler
	sourceService *service.SourceService
}

func NewSourceHandler(sourceService *service.SourceService, renderer *humastar.Renderer) *SourceHandler {
	return &SourceHandler{
		Handler:       humastar.Handler{Renderer: renderer},
		sourceService: sourceService,
	}
}

func (h *SourceHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/editor/sources", h.ListSources, huma.OperationTags("editor"))
	huma.Get(api, "/api/v1/editor/sources/select", h.ListSourcesSelect, huma.OperationTags("editor"))
	huma.Post(api, "/api/v1/editor/sources/upload", h.Upload, huma.OperationTags("editor"))
	huma.Delete(api, "/api/v1/editor/sources/{filename}", h.Delete, huma.OperationTags("editor"))
}

type SourceUploadInput struct {
	RawBody multipart.Form
}

func (h *SourceHandler) Upload(ctx context.Context, input *SourceUploadInput) (*huma.StreamResponse, error) {
	return h.Stream(func(sse humastar.SSE) {
		files := input.RawBody.File["file"]
		if len(files) == 0 {
			sse.Error("No file provided")
			return
		}

		fileHeader := files[0]
		file, err := fileHeader.Open()
		if err != nil {
			sse.Error("Failed to open uploaded file")
			return
		}
		defer file.Close()

		if err := h.sourceService.Save(fileHeader.Filename, file); err != nil {
			sse.Error(err.Error())
			return
		}

		sse.Success("File uploaded: " + fileHeader.Filename)
		if sources, err := h.sourceService.List(); err == nil {
			sse.Patch(h.renderSourceList(sources), "#source-list")
			sse.Patch(h.renderSourceSelect(sources), "#source-select")
		}
	}), nil
}

type SourceDeleteInput struct {
	Filename string `path:"filename" doc:"Source filename to delete"`
}

func (h *SourceHandler) Delete(ctx context.Context, input *SourceDeleteInput) (*huma.StreamResponse, error) {
	return h.Stream(func(sse humastar.SSE) {
		if err := h.sourceService.Delete(input.Filename); err != nil {
			sse.Error(err.Error())
			return
		}

		sse.Success("Deleted: " + input.Filename)
		if sources, err := h.sourceService.List(); err == nil {
			sse.Patch(h.renderSourceList(sources), "#source-list")
			sse.Patch(h.renderSourceSelect(sources), "#source-select")
		}
		sse.DispatchCustomEvent("source-changed", map[string]any{
			"action": "deleted", "filename": input.Filename,
		})
	}), nil
}

func (h *SourceHandler) ListSources(ctx context.Context, input *humastar.EmptyInput) (*huma.StreamResponse, error) {
	return h.Stream(func(sse humastar.SSE) {
		sources, err := h.sourceService.List()
		if err != nil {
			sse.Error("Failed to list sources: " + err.Error())
			return
		}
		sse.Patch(h.renderSourceList(sources), "#source-list")
		sse.Patch(h.renderSourceSelect(sources), "#source-select")
	}), nil
}

func (h *SourceHandler) ListSourcesSelect(ctx context.Context, input *humastar.EmptyInput) (*huma.StreamResponse, error) {
	return h.Stream(func(sse humastar.SSE) {
		sources, err := h.sourceService.List()
		if err != nil {
			sse.Error("Failed to list sources: " + err.Error())
			return
		}
		sse.Patch(h.renderSourceSelect(sources), "#source-select")
	}), nil
}

type SourceCardData struct {
	Name     string
	Size     string
	FileType string
}

func (h *SourceHandler) renderSourceList(sources []service.SourceFile) string {
	items := make([]any, len(sources))
	for i, s := range sources {
		items[i] = SourceCardData{Name: s.Name, Size: s.Size, FileType: s.FileType}
	}
	return h.RenderList("source-card", items, "No Source Files", "Upload GeoJSON or GeoParquet files using the form above.")
}

func (h *SourceHandler) renderSourceSelect(sources []service.SourceFile) string {
	opts := make([]humastar.SelectOptionData, len(sources))
	for i, s := range sources {
		opts[i] = humastar.SelectOptionData{Value: s.Name, Label: s.Name + " (" + s.Size + ")"}
	}
	return h.RenderSelect("-- Select a source file --", opts)
}
