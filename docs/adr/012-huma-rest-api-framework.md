# ADR 012: Huma REST API Framework Integration

## Status
**Proposed**

## Context

The plat-geo server needs to serve two types of clients:

1. **External REST clients** - Mobile apps, CLI tools, third-party integrations
   - Need: JSON responses, OpenAPI documentation, request validation

2. **Internal Datastar actors** - Web UI (viewer/editor)
   - Need: SSE with HTML fragments (`datastar-patch-elements`, `datastar-patch-signals`)

Currently, the server (`internal/server/server.go`) uses custom HTTP handlers with:
- Manual route registration via `http.ServeMux`
- No OpenAPI documentation
- Manual validation logic
- Custom SSE implementation for Datastar

**Key Insight**: Huma's built-in `sse.Register` sends JSON data, but Datastar requires HTML fragments with specific event types. However, Huma's `StreamResponse` allows raw writes - we can use this with the Datastar Go SDK.

## Decision

Adopt **Huma v2** with **Go 1.22+ stdlib (`http.ServeMux`)** via the `humago` adapter as a **unified router** for all endpoints. No external router dependency needed.

### Architecture: One Router, All Endpoints

```
┌─────────────────────────────────────────────────────────────────┐
│                  http.ServeMux + Huma API (humago)               │
│                                                                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │  REST Endpoints │  │ Datastar SSE    │  │ Static/Tiles    │  │
│  │  (Huma ops)     │  │ (Huma + SDK)    │  │ (mux handlers)  │  │
│  │                 │  │                 │  │                 │  │
│  │ GET /api/layers │  │ GET /editor/*   │  │ GET /tiles/*    │  │
│  │ → JSON          │  │ → StreamResponse│  │ → Range req     │  │
│  │ → OpenAPI ✓     │  │ → Datastar SDK  │  │                 │  │
│  │ → Validation ✓  │  │ → HTML frags    │  │                 │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│                                                                  │
│                    ┌─────────────────────┐                       │
│                    │   Service Layer     │                       │
│                    │   (shared logic)    │                       │
│                    └─────────────────────┘                       │
└─────────────────────────────────────────────────────────────────┘
```

### Why This Works

| Need | Solution |
|------|----------|
| REST clients need OpenAPI | Huma generates it automatically |
| Datastar needs HTML fragments | Huma `StreamResponse` + Datastar SDK |
| Input validation for all | Huma validates inputs for both REST and SSE |
| Single router | Go stdlib `http.ServeMux` (Go 1.22+) |
| PMTiles Range requests | Raw mux handler (outside Huma) |
| Zero external router deps | `humago` adapter wraps stdlib |
| HTML stays in HTML files | Template rendering with full CSS control |

### Router Setup

```go
import (
    "net/http"
    "github.com/danielgtaylor/huma/v2"
    "github.com/danielgtaylor/huma/v2/adapters/humago"
)

mux := http.NewServeMux()
api := humago.New(mux, huma.DefaultConfig("plat-geo API", "1.0.0"))

// REST endpoints (JSON responses)
huma.Get(api, "/api/v1/layers", handlers.GetLayers)
huma.Post(api, "/api/v1/layers", handlers.CreateLayer)
huma.Delete(api, "/api/v1/layers/{id}", handlers.DeleteLayer)

// Datastar endpoints (SSE via StreamResponse + Datastar SDK)
huma.Get(api, "/api/v1/editor/layers", handlers.GetEditorLayers)
huma.Post(api, "/api/v1/editor/layers", handlers.CreateEditorLayer)

// Raw HTTP for PMTiles (Range request support)
mux.HandleFunc("GET /tiles/", handlers.ServePMTiles)
```

## Implementation Details

### 1. Service Layer (Shared Business Logic)

```go
// internal/service/layer.go
type LayerService struct {
    dataDir string
    layers  map[string]LayerConfig
}

func (s *LayerService) List() map[string]LayerConfig
func (s *LayerService) Get(id string) (LayerConfig, error)
func (s *LayerService) Create(layer LayerConfig) (string, error)
func (s *LayerService) Delete(id string) error
```

### 2. REST Transport (Huma Operations)

```go
// internal/transport/rest/layers.go
type GetLayersOutput struct {
    Body map[string]LayerConfig
}

func GetLayers(ctx context.Context, input *struct{}) (*GetLayersOutput, error) {
    layers := layerService.List()
    return &GetLayersOutput{Body: layers}, nil
}

// Registration
huma.Get(api, "/api/v1/layers", GetLayers)
```

**Benefits:**
- Auto-generated OpenAPI at `/docs`
- Request validation from struct tags
- RFC 9457 error responses
- Content negotiation

### 3. Datastar Endpoints (Huma StreamResponse + Datastar SDK)

Use Huma's `StreamResponse` to get input validation, then use `humago.Unwrap()` to get stdlib types for Datastar SDK:

```go
import (
    "context"
    "net/http"

    "github.com/danielgtaylor/huma/v2"
    "github.com/danielgtaylor/huma/v2/adapters/humago"
    "github.com/starfederation/datastar-go/datastar"
)

// Input validated by Huma
type GetEditorLayersInput struct {
    // Query params, headers, etc. validated by Huma
}

// Datastar SSE endpoint registered through Huma
func GetEditorLayers(ctx context.Context, input *GetEditorLayersInput) (*huma.StreamResponse, error) {
    return &huma.StreamResponse{
        Body: func(humaCtx huma.Context) {
            // humago.Unwrap() returns (*http.Request, http.ResponseWriter)
            r, w := humago.Unwrap(humaCtx)

            sse := datastar.NewSSE(w, r)

            layers := layerService.List()
            html := renderLayerCards(layers)

            sse.PatchElements(
                datastar.WithSelector("#layer-list"),
                datastar.WithMergeMode(datastar.MergeModeInner),
                datastar.WithFragments(html),
            )
        },
    }, nil
}

// POST with input validation + Datastar response
type CreateEditorLayerInput struct {
    Body struct {
        Name     string `json:"newlayername" required:"true" minLength:"1"`
        File     string `json:"newlayerfile" required:"true"`
        GeomType string `json:"newlayergeomtype" enum:"polygon,line,point"`
    }
}

func CreateEditorLayer(ctx context.Context, input *CreateEditorLayerInput) (*huma.StreamResponse, error) {
    // Huma already validated input.Body
    return &huma.StreamResponse{
        Body: func(humaCtx huma.Context) {
            // humago.Unwrap() returns (*http.Request, http.ResponseWriter)
            r, w := humago.Unwrap(humaCtx)

            // Create layer using validated input
            layer := LayerConfig{
                Name:     input.Body.Name,
                File:     input.Body.File,
                GeomType: input.Body.GeomType,
            }
            id, err := layerService.Create(layer)

            sse := datastar.NewSSE(w, r)
            if err != nil {
                sse.PatchSignals(map[string]any{"error": err.Error()})
                return
            }

            sse.PatchSignals(map[string]any{
                "success":       "Layer created",
                "_editingLayer": false,
            })

            // Refresh layer list
            html := renderLayerCards(layerService.List())
            sse.PatchElements(
                datastar.WithSelector("#layer-list"),
                datastar.WithFragments(html),
            )
        },
    }, nil
}

// Registration - all through Huma
huma.Get(api, "/api/v1/editor/layers", GetEditorLayers)
huma.Post(api, "/api/v1/editor/layers", CreateEditorLayer)
```

**Key Integration Point**: `humago.Unwrap(humaCtx)` bridges Huma and Datastar by extracting the underlying `*http.Request` and `http.ResponseWriter` from the Huma context, allowing the Datastar SDK to work seamlessly.

**Benefits of this approach:**
- **Unified router**: All endpoints registered through Huma
- **Input validation**: Huma validates inputs before handler runs
- **Datastar output**: SDK handles SSE format correctly
- **OpenAPI**: Datastar endpoints appear in docs (as StreamResponse)
- **Type safety**: Go structs for both input and Datastar signals

### 4. Template Rendering (HTML stays in HTML files)

**Key Principle**: HTML stays in `.html` template files for full CSS control. No HTML embedded in Go code.

```
┌─────────────────────────────────────────────────────────────────┐
│  Huma                      │  Templates        │  Datastar SDK  │
│  (validation, OpenAPI)     │  (HTML/CSS)       │  (SSE output)  │
├────────────────────────────┼───────────────────┼────────────────┤
│  Input struct validation   │  layer-card.html  │  PatchElements │
│  Query params              │  tile-card.html   │  PatchSignals  │
│  Body parsing              │  empty-state.html │  SSE format    │
│  StreamResponse            │  Full CSS control │                │
└────────────────────────────┴───────────────────┴────────────────┘
```

Template files in `web/templates/fragments/`:
- `layer-card.html` - Pure HTML with CSS classes
- `tile-card.html` - Designer-friendly, no Go code
- `source-card.html` - Hot-reloadable in dev
- `empty-state.html` - Full control over styling
- `select-option.html`

**Template rendering helper**:

```go
// internal/templates/render.go
package templates

import (
    "bytes"
    "html/template"
)

var tmpl = template.Must(template.ParseGlob("web/templates/fragments/*.html"))

func Render(name string, data any) string {
    var buf bytes.Buffer
    tmpl.ExecuteTemplate(&buf, name, data)
    return buf.String()
}
```

**Usage in handlers**:

```go
func GetEditorLayers(ctx context.Context, input *GetEditorLayersInput) (*huma.StreamResponse, error) {
    return &huma.StreamResponse{
        Body: func(humaCtx huma.Context) {
            r, w := humago.Unwrap(humaCtx)
            sse := datastar.NewSSE(w, r)

            layers := layerService.List()

            // HTML stays in template files - full CSS control
            html := templates.Render("layer-card.html", layers)

            sse.PatchElements(
                datastar.WithSelector("#layer-list"),
                datastar.WithFragments(html),
            )
        },
    }, nil
}
```

**Benefits**:
- **Separation of concerns**: Go handles logic, HTML handles presentation
- **Designer-friendly**: Edit HTML/CSS without touching Go code
- **Hot-reload**: Templates can be reloaded in dev without recompiling
- **Full CSS control**: Use any CSS framework (Tailwind, etc.)

### 5. OpenAPI Configuration

```go
config := huma.DefaultConfig("plat-geo API", "1.0.0")
config.Info.Description = "Geospatial data platform API"
config.Servers = []*huma.Server{
    {URL: "http://localhost:8086"},
}
config.Tags = []*huma.Tag{
    {Name: "layers", Description: "Layer management"},
    {Name: "tiles", Description: "Tile operations"},
    {Name: "sources", Description: "Source file management"},
    {Name: "query", Description: "DuckDB queries"},
}
```

## File Structure

```
internal/
├── service/                    # Business logic (NEW)
│   ├── layer.go               # LayerService
│   ├── tile.go                # TileService
│   ├── source.go              # SourceService
│   └── types.go               # Shared types (LayerConfig, etc.)
│
├── api/                        # Huma API handlers (NEW)
│   ├── rest/                  # JSON REST operations
│   │   ├── layers.go          # GET/POST/DELETE /api/v1/layers
│   │   ├── tiles.go
│   │   ├── sources.go
│   │   └── query.go
│   │
│   ├── editor/                # Datastar SSE operations (via StreamResponse)
│   │   ├── layers.go          # GET/POST /api/v1/editor/layers
│   │   ├── tiles.go
│   │   └── sources.go
│   │
│   └── static/                # Raw HTTP handlers
│       └── tiles.go           # PMTiles serving with Range requests
│
├── server/
│   └── server.go              # http.ServeMux + Huma setup (MODIFIED)
│
├── templates/                 # HTML fragment rendering (NEW)
│   └── render.go              # Template helpers for Datastar
│
└── db/
    └── duckdb.go              # (unchanged)
```

## Migration Strategy

### Phase 1: Tooling Setup (START HERE)
1. Add `taskfiles/Taskfile-huma.yml` with all codegen tasks
2. Include in main `Taskfile.yml`: `huma: ./taskfiles/Taskfile-huma.yml`
3. Add dependencies to `go.mod`:
   ```bash
   go get github.com/danielgtaylor/huma/v2@latest
   go get github.com/starfederation/datastar-go@latest
   ```
4. Install codegen tools:
   ```bash
   go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
   bun add -d openapi-typescript
   ```
5. Create `cmd/openapi/main.go` for spec export
6. Verify: `task huma:spec` exports OpenAPI spec

### Phase 2: Extract Services
1. Create `internal/service/` with shared types
2. Extract business logic from `server.go` handlers
3. Keep existing handlers working (call services)

### Phase 3: Add Huma REST Layer
1. Update `internal/server/server.go` to use `humago.New(mux, config)`
2. Create `internal/api/rest/` operations with full struct tags
3. Register REST endpoints via Huma
4. Verify: `http://localhost:8086/docs` shows API docs
5. Run `task huma:ts` to generate TypeScript types

### Phase 4: Refactor Datastar Handlers
1. Create `internal/templates/render.go` for HTML template rendering
2. Move handlers to `internal/api/editor/`
3. Use `humago.Unwrap()` + Datastar SDK
4. Call services instead of duplicating logic
5. Keep SSE format unchanged

### Phase 5: Cleanup & Verification
1. Remove old `server.go` handler code
2. Update tests
3. Run `task huma:all` to regenerate all clients
4. Verify all endpoints work

## Consequences

### Positive
- **Unified router**: All endpoints through stdlib `http.ServeMux` + Huma
- **Zero external router deps**: Uses Go 1.22+ stdlib routing (no Chi needed)
- **OpenAPI for everything**: REST and editor endpoints documented
- **Input validation for all**: Huma validates inputs for both REST and Datastar
- **Datastar output works**: `humago.Unwrap()` + Datastar SDK handles SSE format
- **Business logic shared**: Services used by both REST and editor handlers
- **Type safety**: Go structs for inputs, Datastar SDK for outputs
- **MCP Registry pattern**: Same architecture as modelcontextprotocol/registry
- **HTML in HTML files**: Full CSS control, designer-friendly, hot-reloadable templates

### Negative
- Additional dependencies (Huma, Datastar SDK)
- Learning curve for the team
- StreamResponse pattern slightly more complex than raw handlers

### Trade-offs Accepted
- Editor endpoints return StreamResponse (not pure JSON) in OpenAPI
- `humago.Unwrap()` used to bridge Huma context to Datastar SDK

## Maximizing Huma Code Generation

Huma uses a **code-first approach** (like FastAPI): define Go structs with tags → Huma auto-generates OpenAPI/JSON Schema. No separate codegen step needed.

### What Huma Generates Automatically

| Feature | Generated From | Output |
|---------|----------------|--------|
| OpenAPI 3.1 spec | Go structs + tags | `/openapi.json`, `/openapi.yaml` |
| JSON Schema | Struct field tags | Embedded in OpenAPI |
| API Documentation | OpenAPI spec | `/docs` (Stoplight Elements) |
| Request validation | Struct tags | Runtime validation |
| Error responses | RFC 9457 | Consistent error format |

### Struct Tags for Maximum Schema Generation

```go
// internal/api/rest/layers.go
type CreateLayerInput struct {
    Body struct {
        Name     string `json:"name" required:"true" minLength:"1" maxLength:"100"
                         doc:"Layer display name" example:"Buildings"`
        File     string `json:"file" required:"true" pattern:"^[a-z0-9_]+$"
                         doc:"Source file identifier" example:"buildings_2024"`
        GeomType string `json:"geomType" required:"true" enum:"polygon,line,point"
                         doc:"Geometry type for the layer"`
        MinZoom  int    `json:"minZoom" minimum:"0" maximum:"22" default:"0"
                         doc:"Minimum zoom level for visibility"`
        MaxZoom  int    `json:"maxZoom" minimum:"0" maximum:"22" default:"22"
                         doc:"Maximum zoom level for visibility"`
        Style    *Style `json:"style,omitempty" doc:"Optional layer styling"`
    }
}

type CreateLayerOutput struct {
    Body struct {
        ID      string      `json:"id" doc:"Generated layer ID" example:"layer_abc123"`
        Layer   LayerConfig `json:"layer" doc:"Created layer configuration"`
        Message string      `json:"message" example:"Layer created successfully"`
    }
}

// Huma auto-generates: OpenAPI paths, request schema, response schema, validation
huma.Post(api, "/api/v1/layers", CreateLayer,
    huma.WithTags("layers"),
    huma.WithSummary("Create a new layer"),
    huma.WithDescription("Creates a new map layer from an existing source file"),
)
```

### Available Struct Tags

| Tag | Purpose | Example |
|-----|---------|---------|
| `required:"true"` | Field is required | `required:"true"` |
| `doc:"..."` | Field description | `doc:"Layer name"` |
| `example:"..."` | Example value | `example:"Buildings"` |
| `enum:"a,b,c"` | Allowed values | `enum:"polygon,line,point"` |
| `default:"..."` | Default value | `default:"0"` |
| `minimum/maximum` | Numeric bounds | `minimum:"0" maximum:"22"` |
| `minLength/maxLength` | String length | `minLength:"1" maxLength:"100"` |
| `pattern:"..."` | Regex validation | `pattern:"^[a-z0-9_]+$"` |
| `format:"..."` | String format | `format:"date-time"` |
| `nullable:"true"` | Allow null | `nullable:"true"` |
| `deprecated:"true"` | Mark deprecated | `deprecated:"true"` |
| `readOnly/writeOnly` | Access control | `readOnly:"true"` |

### Custom Schema Generation

Implement `SchemaProvider` for complex types:

```go
type GeoJSON struct {
    Type        string      `json:"type"`
    Coordinates interface{} `json:"coordinates"`
}

func (g GeoJSON) Schema(r huma.Registry) *huma.Schema {
    return &huma.Schema{
        Type:        "object",
        Description: "GeoJSON geometry object",
        Properties: map[string]*huma.Schema{
            "type": {Type: "string", Enum: []any{"Point", "LineString", "Polygon"}},
            "coordinates": {Type: "array"},
        },
        Required: []string{"type", "coordinates"},
    }
}
```

### Export OpenAPI Spec via CLI

```go
// cmd/geo/main.go
import "github.com/danielgtaylor/huma/v2/humacli"

func main() {
    cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
        mux := http.NewServeMux()
        api := humago.New(mux, huma.DefaultConfig("plat-geo API", "1.0.0"))
        RegisterAllRoutes(api)

        hooks.OnStart(func() { /* start server */ })
    })

    // Add openapi command
    cli.Root().AddCommand(&cobra.Command{
        Use:   "openapi",
        Short: "Export OpenAPI spec",
        Run: func(cmd *cobra.Command, args []string) {
            b, _ := api.OpenAPI().YAML()
            fmt.Println(string(b))
        },
    })

    cli.Run()
}
```

```bash
# Export spec
./geo openapi > openapi.yaml

# Or access at runtime
curl http://localhost:8086/openapi.json
curl http://localhost:8086/openapi.yaml
```

### Client SDK Generation (from OpenAPI)

```bash
# Go client
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
oapi-codegen -package client openapi.yaml > client/client.go

# TypeScript types (using Bun)
bun add -d openapi-typescript
bun x openapi-typescript openapi.json -o web/src/generated/api.ts

# Multi-language (Java, Python, etc.)
openapi-generator generate -i openapi.yaml -g python -o ./clients/python
```

### Generated TypeScript Types

```typescript
// Auto-generated from openapi.json
export interface LayerConfig {
  id: string;
  name: string;
  file: string;
  geomType: "polygon" | "line" | "point";
  minZoom?: number;
  maxZoom?: number;
  style?: Style;
}

export interface CreateLayerRequest {
  name: string;
  file: string;
  geomType: "polygon" | "line" | "point";
  minZoom?: number;
  maxZoom?: number;
  style?: Style;
}

export interface CreateLayerResponse {
  id: string;
  layer: LayerConfig;
  message: string;
}
```

### Built-in API Documentation

Huma serves interactive docs automatically at `/docs`:

```go
// Default: Stoplight Elements
config := huma.DefaultConfig("plat-geo API", "1.0.0")

// Or use Scalar Docs
config.DocsPath = "/docs"
config.Docs = huma.ScalarDocs("plat-geo API")

// Or SwaggerUI
config.Docs = huma.SwaggerUI("plat-geo API")
```

### When Code Generation is Useful

| Scenario | Useful? |
|----------|---------|
| Datastar handles SSE | ❌ No - Datastar parses events automatically |
| Custom JS viewer needs typed events | ✅ Yes - Type safety |
| Mobile app consuming REST API | ✅ Yes - SDK generation |
| External developers using API | ✅ Yes - Documentation |
| Go microservices calling this API | ✅ Yes - oapi-codegen |
| CI/CD contract testing | ✅ Yes - OpenAPI spec validation |

**Recommendation**: Generate TypeScript types for any custom JS. Generate Go client for internal services. For Datastar pages, no codegen needed.

### Full Code Generation Pipeline

Everything flows from the OpenAPI 3.1 spec that Huma generates:

```
Huma API Definition (Go structs + tags)
       │
       ▼
OpenAPI 3.1 Spec (openapi.json)
       │
       ├──► Go client (oapi-codegen)
       ├──► TypeScript types (openapi-typescript)
       ├──► TypeScript client (openapi-fetch, orval)
       ├──► Dart/Flutter client (openapi-generator)
       ├──► Swift client (openapi-generator)
       ├──► Kotlin client (openapi-generator)
       ├──► Python client (openapi-generator)
       ├──► Rust client (openapi-generator)
       ├──► API docs (Scalar, Redoc, Stoplight)
       ├──► Mock server (Prism)
       ├──► Auto-tests (Schemathesis)
       ├──► Postman collection
       └──► Bruno collection
```

### Taskfile for Huma Code Generation

```yaml
# taskfiles/Taskfile-huma.yml
version: '3'

vars:
  OPENAPI_JSON: openapi.json
  OPENAPI_YAML: openapi.yaml

tasks:
  # Export OpenAPI spec from Huma
  spec:
    desc: Export OpenAPI spec (JSON and YAML)
    cmds:
      - go run cmd/openapi/main.go

  spec:json:
    desc: Export OpenAPI spec as JSON
    cmds:
      - go run cmd/openapi/main.go --format json > {{.OPENAPI_JSON}}

  spec:yaml:
    desc: Export OpenAPI spec as YAML
    cmds:
      - go run cmd/openapi/main.go --format yaml > {{.OPENAPI_YAML}}

  # TypeScript generation (using Bun)
  ts:
    desc: Generate TypeScript types from OpenAPI
    deps: [spec:json]
    cmds:
      - bun x openapi-typescript {{.OPENAPI_JSON}} -o web/src/generated/api.ts

  ts:client:
    desc: Generate TypeScript fetch client
    deps: [spec:json]
    cmds:
      - bun x openapi-typescript {{.OPENAPI_JSON}} -o web/src/generated/api.ts
      - echo "Use openapi-fetch with generated types"

  # Go client generation
  go:client:
    desc: Generate Go client from OpenAPI
    deps: [spec:json]
    cmds:
      - oapi-codegen -generate types,client -package apiclient {{.OPENAPI_JSON}} > internal/apiclient/client.go

  go:types:
    desc: Generate Go types only
    deps: [spec:json]
    cmds:
      - oapi-codegen -generate types -package apitypes {{.OPENAPI_JSON}} > internal/apitypes/types.go

  # Mobile clients (optional)
  swift:
    desc: Generate Swift client (iOS)
    deps: [spec:json]
    cmds:
      - openapi-generator-cli generate -i {{.OPENAPI_JSON}} -g swift5 -o generated/swift

  kotlin:
    desc: Generate Kotlin client (Android)
    deps: [spec:json]
    cmds:
      - openapi-generator-cli generate -i {{.OPENAPI_JSON}} -g kotlin -o generated/kotlin

  dart:
    desc: Generate Dart client (Flutter)
    deps: [spec:json]
    cmds:
      - openapi-generator-cli generate -i {{.OPENAPI_JSON}} -g dart -o generated/dart

  # Development tools
  mock:
    desc: Start mock API server (Prism)
    deps: [spec:json]
    cmds:
      - bun x @stoplight/prism-cli mock {{.OPENAPI_JSON}}

  docs:
    desc: Serve API docs locally (Scalar)
    cmds:
      - echo "API docs available at http://localhost:8086/docs"

  # Testing
  test:api:
    desc: Run auto-generated API tests (Schemathesis)
    deps: [spec:json]
    cmds:
      - schemathesis run {{.OPENAPI_JSON}} --base-url http://localhost:8086

  # Convenience
  all:
    desc: Generate all (TypeScript + Go client)
    deps: [ts, go:client]

  clean:
    desc: Remove generated files
    cmds:
      - rm -f {{.OPENAPI_JSON}} {{.OPENAPI_YAML}}
      - rm -rf web/src/generated/
      - rm -rf internal/apiclient/
      - rm -rf generated/
```

Include in main Taskfile:

```yaml
# Taskfile.yml
includes:
  huma: ./taskfiles/Taskfile-huma.yml
```

Usage:

```bash
task huma:spec        # Export OpenAPI spec
task huma:ts          # Generate TypeScript types
task huma:go:client   # Generate Go client
task huma:mock        # Start mock server
task huma:all         # Generate everything
```

### What You Get

| Generated | Use For |
|-----------|---------|
| TypeScript types | Type checking in web UI |
| TypeScript client | Typed fetch calls (openapi-fetch) |
| Go client | Workers calling API, CLI tools |
| Swift client | iOS app |
| Kotlin client | Android app |
| Dart client | Flutter app |
| Scalar/Redoc | Interactive API docs |
| Prism mock | Frontend dev without backend |
| Schemathesis | Automated API contract testing |
| Postman/Bruno | Manual testing, API sharing |

**Summary**: One Huma definition → OpenAPI spec → Everything else generated.

## Verification

1. **REST API**: `curl http://localhost:8086/api/v1/layers` returns JSON
2. **OpenAPI**: Visit `http://localhost:8086/docs` shows Swagger UI
3. **Datastar**: Editor UI at `/editor` still works with SSE updates
4. **PMTiles**: Viewer at `/viewer` loads tiles via Range requests

## Dependencies

```go
// go.mod additions
require (
    github.com/danielgtaylor/huma/v2 v2.34.1
    github.com/starfederation/datastar-go v0.x.x
)
// Note: No external router dependency - uses Go 1.22+ stdlib http.ServeMux
```

## References

- Huma GitHub: https://github.com/danielgtaylor/huma
- Huma humago adapter: `github.com/danielgtaylor/huma/v2/adapters/humago`
- MCP Registry (reference impl): https://github.com/modelcontextprotocol/registry
- Datastar: https://data-star.dev/
- Datastar Go SDK: https://github.com/starfederation/datastar-go
- Datastar Go Docs: https://pkg.go.dev/github.com/starfederation/datastar-go/datastar
- Local Huma source: `.src/huma/`
- Local MCP Registry source: `.src/mcp-registry/`
