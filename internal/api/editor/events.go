package editor

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/plat-geo/internal/humastar"
	"github.com/joeblew999/plat-geo/internal/service"
)

// EventHandler streams resource change events to the Datastar UI via SSE.
type EventHandler struct {
	humastar.Handler
	layerService *service.LayerService
}

// NewEventHandler creates a new event handler.
func NewEventHandler(layerService *service.LayerService, renderer *humastar.Renderer) *EventHandler {
	return &EventHandler{
		Handler:      humastar.Handler{Renderer: renderer},
		layerService: layerService,
	}
}

func (h *EventHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/editor/events", h.Events,
		huma.OperationTags("editor"),
	)
}

func (h *EventHandler) Events(ctx context.Context, input *humastar.EmptyInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := humastar.NewSSE(humaCtx)
			ch := service.DefaultBus.Subscribe()
			defer service.DefaultBus.Unsubscribe(ch)

			for {
				select {
				case <-ctx.Done():
					return
				case ev := <-ch:
					switch ev.Resource {
					case "layers":
						lh := &LayerHandler{
							Handler:      humastar.Handler{Renderer: h.Renderer},
							layerService: h.layerService,
						}
						sse.Patch(lh.renderLayerList(h.layerService.List()), "#layer-list")
					}
					sse.DispatchCustomEvent("resource-changed", map[string]any{
						"resource": ev.Resource,
						"action":   ev.Action,
						"id":       ev.ID,
					})
				}
			}
		},
	}, nil
}
