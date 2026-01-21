# Architecture Overview

## The Huma + Datastar Synergy

This project combines two Go technologies for a seamless full-stack experience:

- **Huma** - REST/RPC framework with OpenAPI 3.1 generation
- **Datastar** - Reactive server-driven UI via Server-Sent Events (SSE)

## Why This Combination?

| Traditional SPA | Huma + Datastar |
|-----------------|-----------------|
| Separate frontend codebase | Single Go codebase |
| Manual API type sync | Generated from structs |
| Client-side state management | Server-side signals |
| JSON API responses | HTML fragments + signals |
| Complex build pipelines | Single `go build` |

## Data Flow

### Creating a Layer (Example)

```
1. User fills form
   └─► Datastar captures via data-bind:newlayername

2. Form submit triggers @post('/api/v1/editor/layers')
   └─► Datastar sends signals as JSON body

3. Huma receives SignalsInput
   └─► ParseLayerConfigSignals() converts JSON → LayerConfig struct

4. Service layer creates layer
   └─► LayerService.Create(config)

5. SSE response streams back
   └─► HTML fragment: updated layer list
   └─► Signals: reset form fields
   └─► Status: success message

6. Datastar applies atomically
   └─► DOM updated with new HTML
   └─► Form cleared via signal reset
   └─► Toast shown via success signal
```

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Browser                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  editor.html                                              │  │
│  │  ├─ data-signals="{ newlayername: '', ... }"             │  │
│  │  ├─ data-bind:newlayername (two-way binding)             │  │
│  │  ├─ data-on:submit → @post('/api/v1/editor/layers')      │  │
│  │  └─ data-init → @get('/api/v1/editor/layers')            │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ SSE Connection
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Go Backend                                    │
│                                                                  │
│  ┌─────────────────┐    ┌─────────────────┐                     │
│  │  Huma Routes    │    │  Editor Routes  │                     │
│  │  (JSON REST)    │    │  (SSE/HTML)     │                     │
│  │                 │    │                 │                     │
│  │  GET /layers    │    │  GET /editor/   │                     │
│  │  POST /layers   │    │    layers       │                     │
│  │  DELETE /layers │    │  POST /editor/  │                     │
│  └────────┬────────┘    │    layers       │                     │
│           │             └────────┬────────┘                     │
│           │                      │                              │
│           │    ┌─────────────────┴──────────────────┐           │
│           │    │  Signal Parsing (GENERATED)        │           │
│           │    │  ParseLayerConfigSignals()         │           │
│           │    │  JSON → service.LayerConfig        │           │
│           │    └─────────────────┬──────────────────┘           │
│           │                      │                              │
│           └──────────────┬───────┘                              │
│                          ▼                                      │
│           ┌──────────────────────────────┐                      │
│           │  Service Layer               │                      │
│           │  ├─ LayerService             │                      │
│           │  ├─ TileService              │                      │
│           │  └─ SourceService            │                      │
│           └──────────────────────────────┘                      │
└─────────────────────────────────────────────────────────────────┘
```

## Dual API Design

The same service layer powers two API styles:

### 1. JSON REST API (for external clients)

```go
// Standard Huma REST endpoint
huma.Get(api, "/api/v1/layers", func(ctx context.Context, input *struct{}) (*LayersResponse, error) {
    layers := layerService.List()
    return &LayersResponse{Body: layers}, nil
})
```

Response:
```json
{
  "layers": [
    {"id": "abc123", "name": "Buildings", "file": "buildings.geojson"}
  ]
}
```

### 2. SSE/HTML API (for Datastar frontend)

```go
// SSE endpoint returning HTML fragments
huma.Get(api, "/api/v1/editor/layers", func(ctx context.Context, input *struct{}) (*huma.StreamResponse, error) {
    return &huma.StreamResponse{
        Body: func(ctx huma.Context) {
            sse := editor.NewSSEContext(ctx)
            layers := layerService.List()
            html := templates.RenderLayerList(layers)
            sse.PatchElements("#layer-list", html)
        },
    }, nil
})
```

Response (SSE):
```
event: datastar-merge-fragments
data: fragments <div id="layer-list">...HTML...</div>
```

## Single Source of Truth

The `service.LayerConfig` struct defines everything:

```go
// internal/service/types.go
type LayerConfig struct {
    ID       string  `json:"id"`
    Name     string  `json:"name" signal:"newlayername"`
    File     string  `json:"file" signal:"newlayerfile"`
    GeomType string  `json:"geomType" signal:"newlayergeomtype"`
    Opacity  float64 `json:"opacity" signal:"newlayeropacity"`
}
```

This struct generates:
- **OpenAPI schema** via Huma (json tags)
- **Signal parsers** via our generator (signal tags)
- **TypeScript types** via openapi-typescript

## File Organization

```
internal/
├── api/
│   ├── routes.go           # JSON REST endpoints
│   └── editor/
│       ├── layers.go       # SSE handlers for layers
│       ├── signals.go      # Signal parsing utilities
│       └── signals_gen.go  # GENERATED signal helpers
├── service/
│   ├── types.go            # Domain models (source of truth)
│   ├── layer.go            # Layer business logic
│   └── tile.go             # Tile business logic
└── templates/
    └── render.go           # HTML fragment renderer

web/templates/
├── editor.html             # Main UI with Datastar
└── fragments/
    ├── layer-card.html     # Single layer component
    └── layer-list.html     # Layer list component
```

## Next Steps

- [Huma Guide](./huma.md) - Deep dive into API patterns
- [Datastar Guide](./datastar.md) - Frontend integration details
- [Code Generation](./codegen.md) - Understanding the gen phases
