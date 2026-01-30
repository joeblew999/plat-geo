# Huma Guide

[Huma](https://huma.rocks/) is the Go REST framework at the center of this project. We use nearly every major feature — not just route registration, but auto-discovery, auto-patch, auto-links, SDK generation, CLI integration, streaming, content negotiation, and OpenAPI-driven code generation.

This document walks through each Huma feature we exploit and how it connects to the rest of the system.

## What Huma gives us (summary)

| Feature | What it does here |
|---------|-------------------|
| **AutoRegister** | Discovers `Register*` methods — zero boilerplate route wiring |
| **AutoPatch** | Generates PATCH from GET+PUT — JSON Merge Patch, JSON Patch, Shorthand |
| **Struct tag validation** | `required`, `minLength`, `enum`, `minimum`, `maximum` → validated + documented |
| **OpenAPI 3.1** | Full spec generated from code — feeds docs UI, TypeScript types, client SDK |
| **Transformers** | Our `LinkTransformer` injects RFC 8288 Link headers on every response |
| **StreamResponse** | First-class SSE for Datastar — HTML fragments + signal updates |
| **humacli** | CLI with flags, env vars, subcommands (`spec`, `gen-client`) |
| **humaclient** | Auto-generated Go SDK from the OpenAPI spec |
| **CBOR** | Content negotiation — JSON by default, CBOR if the client asks |
| **RFC 9457 errors** | Problem Details responses with field-level error locations |
| **Scalar docs** | Interactive API reference at `/docs` powered by the generated spec |

## AutoRegister — zero-wiring route discovery

Instead of manually calling `huma.Get(...)` for every route in a central place, we define handler structs with `Register*` methods:

```go
type APIHandler struct { svc *Services }

func (h *APIHandler) RegisterLayers(api huma.API) {
    huma.Get(api, "/api/v1/layers", h.GetLayers, huma.OperationTags("layers"))
    huma.Post(api, "/api/v1/layers", h.CreateLayer, huma.OperationTags("layers"))
    huma.Get(api, "/api/v1/layers/{id}", h.GetLayer, huma.OperationTags("layers"))
    huma.Put(api, "/api/v1/layers/{id}", h.PutLayer, huma.OperationTags("layers"))
    huma.Delete(api, "/api/v1/layers/{id}", h.DeleteLayer, huma.OperationTags("layers"))
    huma.Post(api, "/api/v1/layers/{id}/publish", h.PublishLayer, huma.OperationTags("layers"))
    huma.Post(api, "/api/v1/layers/{id}/unpublish", h.UnpublishLayer, huma.OperationTags("layers"))
    // ... styles sub-resource, duplicate, etc.
}
```

At startup, one call discovers everything:

```go
huma.AutoRegister(s.humaAPI, api.NewAPIHandler(s.services))
huma.AutoRegister(s.humaAPI, api.NewInfoHandler(s.config.DataDir, s.db != nil))
huma.AutoRegister(s.humaAPI, api.NewDBHandler(s.db))
huma.AutoRegister(s.humaAPI, editor.NewLayerHandler(s.services.Layer, s.renderer))
huma.AutoRegister(s.humaAPI, editor.NewTileHandler(...))
huma.AutoRegister(s.humaAPI, editor.NewSourceHandler(...))
huma.AutoRegister(s.humaAPI, editor.NewEventHandler(...))
```

Add a new handler struct → its routes appear automatically. No central registry to maintain.

## AutoPatch — free PATCH endpoints

One line after route registration:

```go
autopatch.AutoPatch(s.humaAPI)
```

For every resource that has both GET and PUT, Huma generates a PATCH endpoint supporting three RFC standards:

- **JSON Merge Patch** (RFC 7386) — `Content-Type: application/merge-patch+json`
- **JSON Patch** (RFC 6902) — `Content-Type: application/json-patch+json`
- **Shorthand Patch** — Huma's own concise format

So `PUT /api/v1/layers/{id}` automatically gets `PATCH /api/v1/layers/{id}` — no handler code needed.

## Struct tags — validation, docs, and codegen from one source

A single struct definition drives everything:

```go
type LayerConfig struct {
    ID             string       `json:"id"`
    Name           string       `json:"name" required:"true" minLength:"1" maxLength:"100"
                                 doc:"Display name" example:"Buildings"`
    File           string       `json:"file" required:"true" doc:"Source file name"
                                 example:"buildings.pmtiles"
                                 input:"sse" sse:"/api/v1/editor/tiles/select,pmtiles-select"`
    GeomType       string       `json:"geomType" required:"true" enum:"polygon,line,point"
                                 doc:"Geometry type" default:"polygon"`
    Opacity        float64      `json:"opacity,omitempty" minimum:"0" maximum:"1" default:"0.7"
                                 doc:"Layer opacity (0-1)"`
    Published      bool         `json:"published" default:"false" doc:"Whether layer is published"`
    DefaultVisible bool         `json:"defaultVisible" default:"true" signal:"visible"`
    Fill           string       `json:"fill,omitempty" default:"#3388ff" input:"color"`
    Stroke         string       `json:"stroke,omitempty" default:"#2266cc" input:"color"`
    Styles         []Style      `json:"styles,omitempty" doc:"Named style variants"`
}
```

What Huma reads from these tags:

| Tag | Huma uses it for |
|-----|------------------|
| `json` | Field name in JSON + OpenAPI schema |
| `required` | OpenAPI `required` + request validation |
| `minLength`, `maxLength` | OpenAPI string constraints + validation |
| `minimum`, `maximum` | OpenAPI numeric constraints + validation |
| `enum` | OpenAPI enum + validation |
| `default` | OpenAPI default value |
| `doc` | OpenAPI field description |
| `example` | OpenAPI example value |

What our codegen reads from the same tags:

| Tag | Our codegen uses it for |
|-----|-------------------------|
| `signal` | Datastar signal name suffix override |
| `input` | HTML input type: `color`, `sse`, `checkbox`, `select` |
| `sse` | SSE endpoint URL + element ID for dynamic selects |
| `card` | Card layout role in the editor: `title`, `meta`, `badge` |

One struct → OpenAPI spec → validation → docs → Datastar signals → HTML forms → TypeScript types.

## Transformers — injecting Link headers

Huma transformers run on every response. We register one:

```go
humaConfig.Transformers = append(humaConfig.Transformers, humastar.LinkTransformer())
```

This transformer checks every response body for:

1. **Static links** — pre-computed from `AutoLinks()` at startup
2. **`Pager` interface** — emits `first`/`prev`/`next`/`last` rels
3. **`Actor` interface** — emits state-dependent action links

The result: every response gets RFC 8288 Link headers automatically. See [Hypermedia & HATEOAS](./hypermedia.md) for the full design.

## StreamResponse — SSE for Datastar

Huma's `StreamResponse` gives us first-class server-sent events. The editor uses this for real-time UI updates:

```go
func (h *LayerHandler) RegisterRoutes(api huma.API) {
    huma.Get(api, "/api/v1/editor/layers", h.ListLayers, huma.OperationTags("editor"))
    huma.Post(api, "/api/v1/editor/layers", h.CreateLayer, huma.OperationTags("editor"))
    huma.Delete(api, "/api/v1/editor/layers/{id}", h.DeleteLayer, huma.OperationTags("editor"))
}

func (h *LayerHandler) CreateLayer(ctx context.Context, input *humastar.SignalsInput) (*huma.StreamResponse, error) {
    return &huma.StreamResponse{
        Body: func(ctx huma.Context) {
            sse := humastar.SSE(ctx)

            signals := input.MustParse()
            config := editor.ParseLayerConfigSignals(signals)

            layer, err := h.svc.Create(config)
            if err != nil {
                sse.Error(err.Error())
                return
            }

            // Update DOM with new layer list
            sse.Patch("#layer-list", h.renderer.RenderLayerList(h.svc.List()))
            // Reset form
            sse.Signals(editor.ResetLayerConfigSignals())
            // Show success toast
            sse.Success("Layer '" + layer.Name + "' created")
        },
    }, nil
}
```

The `StreamResponse.Body` callback writes SSE events that Datastar processes:
- `datastar-merge-fragments` — HTML fragment updates
- `datastar-merge-signals` — reactive state updates
- `datastar-remove-fragments` — element removal

## humacli — CLI with env vars and subcommands

The binary is a full CLI powered by Huma + Cobra:

```go
type Options struct {
    Host    string `doc:"Host to bind to" default:"0.0.0.0"`
    Port    int    `doc:"Port to listen on" short:"p" default:"8086"`
    DataDir string `doc:"Directory for geo data files" default:".data"`
    WebDir  string `doc:"Path to web/ directory" default:"web"`
}

func main() {
    cli := humacli.New(func(hooks humacli.Hooks, opts *Options) {
        srv := newServer(opts)
        hooks.OnStart(func() {
            http.ListenAndServe(addr, srv)
        })
    })

    cli.Root().AddCommand(specCmd)      // geo spec [--yaml]
    cli.Root().AddCommand(genClientCmd) // geo gen-client [-o dir]
    cli.Run()
}
```

Usage:

```bash
geo                          # start server (default port 8086)
geo --port 9000              # custom port
geo spec                     # export OpenAPI JSON to stdout
geo spec --yaml              # export OpenAPI YAML
geo gen-client -o pkg/geo    # generate Go client SDK
SERVICE_PORT=9000 geo        # env var binding (SERVICE_ prefix)
```

Huma auto-binds `Options` struct fields to flags and `SERVICE_*` environment variables.

## humaclient — auto-generated Go SDK

At startup:

```go
humaclient.Register(s.humaAPI)
```

Then from the CLI:

```bash
geo gen-client -o pkg/geoclient
```

This generates a type-safe Go client in `pkg/geoclient/client.go` — fully typed request/response structs matching the OpenAPI spec. External Go services can import and call the API without manual HTTP code.

## Content negotiation — JSON + CBOR

One import enables CBOR:

```go
import _ "github.com/danielgtaylor/huma/v2/formats/cbor"
```

Clients sending `Accept: application/cbor` get CBOR responses. JSON remains the default. No handler changes needed.

## OpenAPI 3.1 — auto-generated, fully customized

The spec is generated from code, not written manually:

```go
humaConfig := huma.DefaultConfig("plat-geo API", "1.0.0")
humaConfig.Info.Description = "Geospatial data platform API..."
humaConfig.Servers = []*huma.Server{
    {URL: "http://localhost:8086", Description: "Local server"},
}

humaAPI.OpenAPI().Tags = append(humaAPI.OpenAPI().Tags,
    &huma.Tag{Name: "layers", Description: "Layer management operations"},
    &huma.Tag{Name: "editor", Description: "Editor SSE endpoints (Datastar)"},
    // ...
)
```

The spec includes:
- All endpoints with paths, methods, parameters
- Request/response schemas from Go struct tags
- Validation constraints (required, min/max, enum)
- Documentation from `doc:` tags
- Examples from `example:` tags
- RFC 8288 Link relations (injected by `AutoLinks`)
- Operation tags for grouping

The spec powers:
- **Scalar docs UI** at `/docs`
- **TypeScript types** via `openapi-typescript` → `web/src/generated/api.ts`
- **Go client SDK** via `humaclient` → `pkg/geoclient/client.go`
- **Explorer schema-driven forms** — reads the spec to build create/edit forms dynamically

## Error handling — RFC 9457 Problem Details

```go
huma.Error400BadRequest("Invalid input")
huma.Error404NotFound("Layer not found")
huma.Error503ServiceUnavailable("Database not configured")
```

Response:

```json
{
  "type": "https://httpproblems.com/http-status/400",
  "title": "Bad Request",
  "status": 400,
  "detail": "Invalid input"
}
```

Validation failures include field-level error locations:

```json
{
  "status": 422,
  "title": "Unprocessable Entity",
  "errors": [
    {
      "message": "expected string length >= 1",
      "location": "body.name",
      "value": ""
    }
  ]
}
```

## Operation tags — organizing the API

Every route gets a tag:

```go
huma.Get(api, "/api/v1/layers", h.GetLayers, huma.OperationTags("layers"))
huma.Post(api, "/api/v1/editor/layers", h.CreateLayer, huma.OperationTags("editor"))
```

Tags serve three purposes:
1. **Docs UI** — groups endpoints in Scalar
2. **AutoLinks** — cross-links collections sharing a tag (layers ↔ sources ↔ tiles)
3. **Filtering** — editor SSE endpoints are tagged `editor` so `AutoLinks` skips them

## The startup sequence

```go
func New(cfg Config) *Server {
    // 1. Configure Huma with our transformer
    humaConfig := huma.DefaultConfig("plat-geo API", "1.0.0")
    humaConfig.Transformers = append(humaConfig.Transformers, humastar.LinkTransformer())
    humaAPI := humago.New(mux, humaConfig)

    // 2. Auto-discover all routes
    huma.AutoRegister(humaAPI, api.NewAPIHandler(services))
    huma.AutoRegister(humaAPI, editor.NewLayerHandler(...))
    // ...

    // 3. Auto-generate PATCH from GET+PUT
    autopatch.AutoPatch(humaAPI)

    // 4. Auto-generate hypermedia Link headers from OpenAPI paths
    humastar.AutoLinks(humaAPI)

    // 5. Register for SDK generation
    humaclient.Register(humaAPI)
}
```

Five lines. The result: a fully discoverable hypermedia API with auto-patch, auto-links, streaming SSE, content negotiation, validation, OpenAPI docs, and a generated client SDK.

## What we get from one Go struct

Starting from `service.LayerConfig`:

```
LayerConfig struct tags
    │
    ├─► Huma: OpenAPI schema + validation + docs
    │     ├─► /openapi.json (auto-generated spec)
    │     ├─► /docs (Scalar interactive reference)
    │     ├─► /explorer (schema-driven forms)
    │     ├─► autopatch (PATCH from GET+PUT)
    │     ├─► pkg/geoclient/ (humaclient Go SDK)
    │     └─► web/src/generated/api.ts (TypeScript types)
    │
    ├─► humastargen: signal + form codegen
    │     ├─► signals_gen.go (Datastar signal parsers)
    │     └─► layer-form-gen.html (Datastar form template)
    │
    └─► humastar: hypermedia
          ├─► AutoLinks (structural RFC 8288 links)
          ├─► Actor (state-dependent actions)
          └─► PageBody[T] (pagination links)
```

One struct. Everything else is derived.
