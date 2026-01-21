# Huma Guide

[Huma](https://huma.rocks/) is a modern Go framework for building REST APIs with automatic OpenAPI 3.1 generation.

## Why Huma?

1. **Type-safe APIs** - Input/output defined as Go structs
2. **Auto OpenAPI** - Spec generated from code, not the other way around
3. **Validation** - Request validation from struct tags
4. **RFC 9457 Errors** - Standard problem details responses
5. **Streaming** - First-class SSE support via `StreamResponse`

## Setup

```go
// internal/server/server.go
import (
    "github.com/danielgtaylor/huma/v2"
    "github.com/danielgtaylor/huma/v2/adapters/humago"
)

func NewServer() *http.Server {
    mux := http.NewServeMux()

    // Create Huma API with stdlib router (no external dependencies)
    api := humago.New(mux, huma.DefaultConfig("Geo API", "1.0.0"))

    // Register routes
    registerRoutes(api)

    return &http.Server{Handler: mux}
}
```

## Defining Endpoints

### Basic REST Endpoint

```go
// Input struct with path parameters
type GetLayerInput struct {
    ID string `path:"id" doc:"Layer ID"`
}

// Output struct
type LayerResponse struct {
    Body service.LayerConfig
}

// Register endpoint
huma.Get(api, "/api/v1/layers/{id}", func(ctx context.Context, input *GetLayerInput) (*LayerResponse, error) {
    layer, err := layerService.Get(input.ID)
    if err != nil {
        return nil, huma.Error404NotFound("Layer not found")
    }
    return &LayerResponse{Body: layer}, nil
})
```

### POST with Request Body

```go
type CreateLayerInput struct {
    Body service.LayerConfig
}

type CreateLayerResponse struct {
    Body service.LayerConfig
}

huma.Post(api, "/api/v1/layers", func(ctx context.Context, input *CreateLayerInput) (*CreateLayerResponse, error) {
    layer, err := layerService.Create(input.Body)
    if err != nil {
        return nil, huma.Error400BadRequest(err.Error())
    }
    return &CreateLayerResponse{Body: layer}, nil
})
```

### Query Parameters

```go
type ListLayersInput struct {
    Limit  int    `query:"limit" default:"10" doc:"Max results"`
    Offset int    `query:"offset" default:"0" doc:"Skip results"`
    Filter string `query:"filter" doc:"Filter by name"`
}

huma.Get(api, "/api/v1/layers", func(ctx context.Context, input *ListLayersInput) (*LayersResponse, error) {
    layers := layerService.List(input.Limit, input.Offset, input.Filter)
    return &LayersResponse{Body: layers}, nil
})
```

## SSE Streaming (for Datastar)

The key integration point with Datastar is SSE streaming:

```go
// SSE endpoint that returns HTML fragments
huma.Get(api, "/api/v1/editor/layers", func(ctx context.Context, input *struct{}) (*huma.StreamResponse, error) {
    return &huma.StreamResponse{
        Body: func(ctx huma.Context) {
            // Get SSE helper
            sse := editor.NewSSEContext(ctx)

            // Get data
            layers := layerService.List()

            // Render HTML fragment
            html := templates.RenderLayerList(layers)

            // Send to Datastar
            sse.PatchElements("#layer-list", html)
        },
    }, nil
})
```

### Receiving Datastar Signals

When Datastar sends a `@post()`, it includes all signals as JSON:

```go
// SignalsInput captures raw JSON body from Datastar
type SignalsInput struct {
    RawBody []byte
}

func (s *SignalsInput) Resolve(ctx huma.Context) []error {
    body, _ := io.ReadAll(ctx.BodyReader())
    s.RawBody = body
    return nil
}

// Use in handler
huma.Post(api, "/api/v1/editor/layers", func(ctx context.Context, input *SignalsInput) (*huma.StreamResponse, error) {
    return &huma.StreamResponse{
        Body: func(ctx huma.Context) {
            sse := editor.NewSSEContext(ctx)

            // Parse signals to type-safe struct (GENERATED)
            signals := input.MustParse()
            config := ParseLayerConfigSignals(signals)

            // Create layer
            layer, err := layerService.Create(config)
            if err != nil {
                sse.SendError(err.Error())
                return
            }

            // Send updates
            sse.PatchElements("#layer-list", templates.RenderLayerList(layerService.List()))
            sse.SendSignals(ResetLayerConfigSignals()) // Clear form
            sse.SendSuccess("Layer created: " + layer.Name)
        },
    }, nil
})
```

## Validation

Use struct tags for validation:

```go
type CreateLayerInput struct {
    Body struct {
        Name     string  `json:"name" minLength:"1" maxLength:"100" doc:"Layer name"`
        File     string  `json:"file" pattern:".*\\.(geojson|json)$" doc:"GeoJSON file"`
        Opacity  float64 `json:"opacity" minimum:"0" maximum:"1" default:"1"`
        GeomType string  `json:"geomType" enum:"point,line,polygon" doc:"Geometry type"`
    }
}
```

Huma automatically:
- Validates incoming requests
- Returns RFC 9457 error responses
- Documents constraints in OpenAPI spec

## Error Handling

```go
// Built-in error helpers
huma.Error400BadRequest("Invalid input")
huma.Error404NotFound("Layer not found")
huma.Error500InternalServerError("Database error")

// Custom error with details
huma.NewError(422, "Validation failed",
    &huma.ErrorDetail{
        Location: "body.name",
        Message:  "Name already exists",
        Value:    input.Body.Name,
    },
)
```

## OpenAPI Generation

Huma generates OpenAPI 3.1 spec automatically:

```bash
# Export spec via CLI
task huma:spec

# Or programmatically
go run ./cmd/geo spec --output openapi.json
```

The spec includes:
- All endpoints with paths, methods, parameters
- Request/response schemas from Go types
- Validation rules from struct tags
- Documentation from `doc:` tags

## Route Organization

```go
// internal/api/routes.go
func RegisterRoutes(api huma.API, services *service.Services) {
    // JSON REST API
    registerLayerRoutes(api, services.Layer)
    registerTileRoutes(api, services.Tile)
    registerSourceRoutes(api, services.Source)

    // SSE Editor API (Datastar)
    editor.RegisterRoutes(api, services)
}

func registerLayerRoutes(api huma.API, svc *service.LayerService) {
    huma.Get(api, "/api/v1/layers", listLayers(svc))
    huma.Post(api, "/api/v1/layers", createLayer(svc))
    huma.Get(api, "/api/v1/layers/{id}", getLayer(svc))
    huma.Delete(api, "/api/v1/layers/{id}", deleteLayer(svc))
}
```

## Best Practices

1. **Separate REST from SSE** - JSON endpoints in `/api/v1/`, SSE in `/api/v1/editor/`
2. **Use StreamResponse for SSE** - Never return HTML from regular endpoints
3. **Validate at the edge** - Use struct tags, not manual checks
4. **Document everything** - Use `doc:` tags for OpenAPI descriptions
5. **Keep handlers thin** - Business logic belongs in service layer

## Next Steps

- [Datastar Guide](./datastar.md) - Frontend integration
- [Code Generation](./codegen.md) - Signal parser generation
