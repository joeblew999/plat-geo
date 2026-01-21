# Datastar Guide

[Datastar](https://data-star.dev/) is a lightweight reactive frontend library that uses Server-Sent Events (SSE) for server-driven UI updates.

## Why Datastar?

1. **No Build Step** - Single JS file, no webpack/vite needed
2. **Server-Driven** - HTML rendered on server, not client
3. **Reactive Signals** - Two-way data binding without React/Vue complexity
4. **SSE Native** - Real-time updates without WebSocket complexity
5. **Progressive** - Works without JS, enhances with Datastar

## Core Concepts

### Signals

Signals are reactive state variables that sync between HTML and JavaScript:

```html
<body data-signals="{
    _activeTab: 'layers',
    newlayername: '',
    newlayerfile: '',
    newlayeropacity: 1.0
}">
```

- Prefixed with `_` = local-only (not sent to server)
- No prefix = sent with `@post()` / `@get()` requests

### Two-Way Binding

```html
<!-- Input syncs to signal -->
<input data-bind:newlayername placeholder="Layer name">

<!-- Signal displays in text -->
<span data-text="$newlayername"></span>

<!-- Conditional display -->
<div data-show="$newlayername !== ''">
    Name entered: <span data-text="$newlayername"></span>
</div>
```

### Event Handling

```html
<!-- Click handler -->
<button data-on:click="$_activeTab = 'tiles'">
    View Tiles
</button>

<!-- Form submit with SSE -->
<form data-on:submit__prevent="@post('/api/v1/editor/layers')">
    <input data-bind:newlayername>
    <button type="submit">Create</button>
</form>

<!-- Initialize with SSE fetch -->
<div id="layer-list" data-init="@get('/api/v1/editor/layers')">
    Loading...
</div>
```

## SSE Actions

### @get() - Fetch and merge

```html
<!-- Fetches HTML, merges into DOM -->
<div data-init="@get('/api/v1/editor/layers')">
```

### @post() - Submit signals

```html
<!-- Sends all non-underscore signals as JSON body -->
<form data-on:submit__prevent="@post('/api/v1/editor/layers')">
```

Request body:
```json
{
    "newlayername": "Buildings",
    "newlayerfile": "buildings.geojson",
    "newlayeropacity": 0.8
}
```

### @delete() - Delete with ID

```html
<button data-on:click="@delete('/api/v1/editor/layers/{{.ID}}')">
    Delete
</button>
```

## Server Response Format

The Go backend sends SSE events that Datastar understands:

### Patch Elements (Update DOM)

```go
sse.PatchElements("#layer-list", "<div>New HTML content</div>")
```

SSE Event:
```
event: datastar-merge-fragments
data: fragments <div id="layer-list">New HTML content</div>
```

### Send Signals (Update State)

```go
sse.SendSignals(map[string]any{
    "newlayername": "",      // Clear form
    "success": "Created!",   // Show message
})
```

SSE Event:
```
event: datastar-merge-signals
data: signals {"newlayername":"","success":"Created!"}
```

### Remove Elements

```go
sse.RemoveElements("#layer-abc123")
```

## HTML Structure

### Main Template (editor.html)

```html
<!DOCTYPE html>
<html>
<head>
    <script src="https://cdn.jsdelivr.net/npm/@sudodevnull/datastar"></script>
</head>
<body data-signals="{
    _activeTab: 'layers',
    _editingLayer: false,
    newlayername: '',
    newlayerfile: '',
    newlayergeomtype: 'polygon',
    newlayeropacity: 1.0,
    success: '',
    error: ''
}">
    <!-- Navigation -->
    <nav>
        <button data-on:click="$_activeTab = 'layers'"
                data-class:active="$_activeTab === 'layers'">
            Layers
        </button>
        <button data-on:click="$_activeTab = 'tiles'"
                data-class:active="$_activeTab === 'tiles'">
            Tiles
        </button>
    </nav>

    <!-- Tab Content -->
    <div data-show="$_activeTab === 'layers'">
        <!-- Layer List (SSE populated) -->
        <div id="layer-list" data-init="@get('/api/v1/editor/layers')">
            Loading layers...
        </div>

        <!-- Create Form -->
        <form data-on:submit__prevent="@post('/api/v1/editor/layers')">
            <input data-bind:newlayername placeholder="Name" required>
            <input data-bind:newlayerfile placeholder="File" required>
            <select data-bind:newlayergeomtype>
                <option value="point">Point</option>
                <option value="line">Line</option>
                <option value="polygon">Polygon</option>
            </select>
            <input type="range" data-bind:newlayeropacity min="0" max="1" step="0.1">
            <button type="submit">Create Layer</button>
        </form>
    </div>

    <div data-show="$_activeTab === 'tiles'">
        <div id="tile-list" data-init="@get('/api/v1/editor/tiles')">
            Loading tiles...
        </div>
    </div>

    <!-- Toast Messages -->
    <div data-show="$success !== ''" data-text="$success" class="toast success"></div>
    <div data-show="$error !== ''" data-text="$error" class="toast error"></div>
</body>
</html>
```

### Fragment Templates

Fragments are partial HTML returned by SSE:

```html
<!-- web/templates/fragments/layer-card.html -->
<div id="layer-{{.ID}}" class="layer-card">
    <h3>{{.Name}}</h3>
    <p>File: {{.File}}</p>
    <p>Type: {{.GeomType}}</p>
    <p>Opacity: {{.Opacity}}</p>
    <button data-on:click="@delete('/api/v1/editor/layers/{{.ID}}')">
        Delete
    </button>
</div>
```

```html
<!-- web/templates/fragments/layer-list.html -->
<div id="layer-list">
    {{range .Layers}}
        {{template "layer-card.html" .}}
    {{else}}
        <p>No layers yet. Create one above.</p>
    {{end}}
</div>
```

## Go Backend Integration

### SSE Helper

```go
// internal/api/editor/sse.go
type SSEContext struct {
    sse *datastar.ServerSentEventGenerator
}

func NewSSEContext(ctx huma.Context) *SSEContext {
    w := ctx.BodyWriter().(http.ResponseWriter)
    r := ctx.Context().Value("request").(*http.Request)
    return &SSEContext{
        sse: datastar.NewSSE(w, r),
    }
}

func (s *SSEContext) PatchElements(selector, html string) {
    s.sse.MergeFragments(html, datastar.WithSelector(selector))
}

func (s *SSEContext) SendSignals(signals map[string]any) {
    s.sse.MergeSignals(signals)
}

func (s *SSEContext) RemoveElements(selector string) {
    s.sse.RemoveFragments(datastar.WithSelector(selector))
}

func (s *SSEContext) SendSuccess(msg string) {
    s.SendSignals(map[string]any{"success": msg, "error": ""})
}

func (s *SSEContext) SendError(msg string) {
    s.SendSignals(map[string]any{"error": msg, "success": ""})
}
```

### Complete Handler Example

```go
// POST /api/v1/editor/layers - Create layer via Datastar
func CreateLayerHandler(svc *service.LayerService) func(context.Context, *SignalsInput) (*huma.StreamResponse, error) {
    return func(ctx context.Context, input *SignalsInput) (*huma.StreamResponse, error) {
        return &huma.StreamResponse{
            Body: func(ctx huma.Context) {
                sse := NewSSEContext(ctx)

                // Parse Datastar signals to typed struct
                signals := input.MustParse()
                config := ParseLayerConfigSignals(signals)

                // Validate
                if config.Name == "" {
                    sse.SendError("Name is required")
                    return
                }

                // Create
                layer, err := svc.Create(config)
                if err != nil {
                    sse.SendError(err.Error())
                    return
                }

                // Update UI
                layers := svc.List()
                html := templates.RenderLayerList(layers)
                sse.PatchElements("#layer-list", html)

                // Reset form + show success
                sse.SendSignals(ResetLayerConfigSignals())
                sse.SendSuccess("Layer '" + layer.Name + "' created")
            },
        }, nil
    }
}
```

## Signal Naming Convention

Signals that map to form fields follow a naming pattern:

| Go Field | Signal Name | HTML Binding |
|----------|-------------|--------------|
| `Name` | `newlayername` | `data-bind:newlayername` |
| `File` | `newlayerfile` | `data-bind:newlayerfile` |
| `GeomType` | `newlayergeomtype` | `data-bind:newlayergeomtype` |
| `Opacity` | `newlayeropacity` | `data-bind:newlayeropacity` |

This convention is enforced by the signal generator (see [Code Generation](./codegen.md)).

## Best Practices

1. **Prefix local signals with `_`** - Keeps them out of server requests
2. **Use fragments for updates** - Don't send entire pages
3. **Reset forms via signals** - Use `SendSignals()` after create/update
4. **Show loading states** - Use `data-indicator` for spinners
5. **Handle errors gracefully** - Always send error signals on failure

## Next Steps

- [Code Generation](./codegen.md) - How signal parsers are generated
- [Architecture](./architecture.md) - Full system overview
