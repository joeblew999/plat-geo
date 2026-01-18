# ADR-011: Web UI - Viewer and Editor

## Status

Proposed

## Context

plat-geo needs web interfaces for:

1. **Viewer** - Public-facing map display for viewing PMTiles layers
2. **Editor** - Admin interface for configuring layers, styles, and triggering tile generation

The ubuntu-website project has a working PMTiles viewer using Leaflet + protomaps-leaflet. We need to port this and add an editor interface using Datastar for reactivity.

## Decision

Create a web UI with two modes:

- **Viewer**: Read-only map display with layer toggles
- **Editor**: Admin interface for layer configuration, styling, and tile generation

Use Datastar for the editor's server-driven reactivity.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Web Clients                             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────────────┐    ┌─────────────────────────────┐ │
│  │      Viewer         │    │         Editor              │ │
│  │  (Public Maps)      │    │   (Admin/Datastar)          │ │
│  │                     │    │                             │ │
│  │  - Layer toggles    │    │  - Layer config             │ │
│  │  - Legend display   │    │  - Style editor             │ │
│  │  - Read-only        │    │  - Tile generation          │ │
│  └─────────────────────┘    │  - Data source management   │ │
│                             └─────────────────────────────┘ │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    plat-geo Server                           │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                   API Endpoints                          ││
│  │  GET  /                    → Viewer HTML                 ││
│  │  GET  /editor              → Editor HTML (Datastar)      ││
│  │  GET  /api/v1/layers       → Layer list                  ││
│  │  POST /api/v1/layers       → Create layer config         ││
│  │  PUT  /api/v1/layers/:id   → Update layer config         ││
│  │  POST /api/v1/generate     → Trigger tile generation     ││
│  │  GET  /tiles/:file         → Serve PMTiles               ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  .data/pmtiles/                              │
│  ├── basemap.pmtiles                                        │
│  ├── places.pmtiles                                         │
│  └── custom.pmtiles                                         │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
web/
├── static/
│   ├── js/
│   │   └── vendor/
│   │       ├── leaflet.js          # Map library
│   │       ├── pmtiles.js          # PMTiles reader
│   │       ├── protomaps-leaflet.js # Vector tile renderer
│   │       └── datastar.js         # Reactivity (editor only)
│   └── css/
│       └── vendor/
│           └── leaflet.css
├── templates/
│   ├── viewer.html                 # Public map viewer
│   ├── editor.html                 # Admin editor (Datastar)
│   └── partials/
│       ├── layer-list.html         # Layer toggle list
│       ├── layer-form.html         # Layer edit form
│       └── style-editor.html       # Style configuration
└── embed.go                        # Go embed for static files
```

## Viewer Features

Based on ubuntu-website's airspace-demo.html:

- Full-screen map with Leaflet
- Layer toggles (checkboxes)
- Legend display per layer
- Mobile-responsive (collapsible panel)
- PMTiles loading from local server or CDN
- Multiple layer support with z-ordering

```html
<!-- Viewer: Read-only map display -->
<div id="map" class="map-container"></div>
<div id="panel" class="info-panel">
  <h3>Layers</h3>
  <div id="layer-list">
    <!-- Layer toggles generated from manifest -->
  </div>
  <div id="legend">
    <!-- Legend entries -->
  </div>
</div>
```

## Editor Features (Datastar)

Server-driven reactivity for admin operations:

### Layer Configuration
```html
<div data-signals='{"layers": []}'>
  <div data-on-load="@get('/api/v1/layers')">
    <template data-for="layer in $layers">
      <div class="layer-card">
        <input type="text" data-bind="layer.name">
        <input type="color" data-bind="layer.fill">
        <input type="range" data-bind="layer.opacity" min="0" max="1" step="0.1">
        <button data-on-click="@put('/api/v1/layers/' + layer.id)">Save</button>
      </div>
    </template>
  </div>
</div>
```

### Style Editor
```html
<div class="style-editor">
  <h4>Render Rules</h4>
  <div data-for="rule in $layer.renderRules">
    <select data-bind="rule.filterProp">
      <option value="">No filter</option>
      <!-- Property options from data -->
    </select>
    <input type="text" data-bind="rule.filterValue">
    <input type="color" data-bind="rule.fill">
    <input type="color" data-bind="rule.stroke">
    <input type="range" data-bind="rule.opacity" min="0" max="1">
  </div>
  <button data-on-click="@post('/api/v1/layers/' + $layer.id + '/rules')">
    Add Rule
  </button>
</div>
```

### Tile Generation
```html
<div class="tile-generator">
  <h4>Generate Tiles</h4>
  <select data-bind="$source">
    <option value="duckdb">DuckDB Table</option>
    <option value="geojson">GeoJSON File</option>
    <option value="url">Remote URL</option>
  </select>

  <div data-show="$source === 'duckdb'">
    <input type="text" data-bind="$table" placeholder="Table name">
  </div>

  <div data-show="$source === 'geojson'">
    <input type="file" data-bind="$file">
  </div>

  <div class="zoom-config">
    <label>Min Zoom: <input type="number" data-bind="$minZoom" value="0"></label>
    <label>Max Zoom: <input type="number" data-bind="$maxZoom" value="14"></label>
  </div>

  <button data-on-click="@post('/api/v1/generate')" data-indicator>
    Generate Tiles
  </button>

  <div data-show="$generating" class="progress">
    Generating tiles...
  </div>
</div>
```

## JavaScript Libraries

### Vendored (no CDN dependency)

| Library | Size | Purpose |
|---------|------|---------|
| leaflet.js | 144 KB | Map display |
| pmtiles.js | 51 KB | PMTiles format reader |
| protomaps-leaflet.js | 125 KB | Vector tile rendering |
| datastar.js | ~15 KB | Server-driven reactivity |

### Paint Rules Pattern

```javascript
function buildPaintRules(layer) {
  return layer.renderRules.map(rule => ({
    dataLayer: layer.pmtilesLayer,
    filter: rule.filterProp
      ? (z, f) => f.props[rule.filterProp] === rule.filterValue
      : undefined,
    symbolizer: layer.geomType === 'point'
      ? new protomapsL.CircleSymbolizer({
          fill: rule.fill,
          radius: rule.radius || 5,
          opacity: rule.opacity
        })
      : new protomapsL.PolygonSymbolizer({
          fill: rule.fill,
          stroke: rule.stroke,
          opacity: rule.opacity,
          width: rule.width || 1
        })
  }));
}
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Viewer HTML |
| GET | `/editor` | Editor HTML |
| GET | `/api/v1/layers` | List layer configs |
| POST | `/api/v1/layers` | Create layer config |
| GET | `/api/v1/layers/:id` | Get layer config |
| PUT | `/api/v1/layers/:id` | Update layer config |
| DELETE | `/api/v1/layers/:id` | Delete layer config |
| POST | `/api/v1/generate` | Trigger tile generation |
| GET | `/api/v1/generate/:id/status` | Generation status |
| GET | `/tiles/:file` | Serve PMTiles (range requests) |

## Layer Configuration Schema

```json
{
  "id": "places",
  "name": "Points of Interest",
  "file": "places.pmtiles",
  "pmtilesLayer": "places",
  "geomType": "point",
  "defaultVisible": true,
  "zoomRange": [0, 14],
  "renderRules": [
    {
      "filterProp": "category",
      "filterValue": "restaurant",
      "fill": "#ff6b6b",
      "stroke": "#c0392b",
      "opacity": 0.8,
      "radius": 6
    },
    {
      "filterProp": "category",
      "filterValue": "shop",
      "fill": "#4ecdc4",
      "stroke": "#1abc9c",
      "opacity": 0.8,
      "radius": 5
    }
  ],
  "legend": [
    {"label": "Restaurant", "color": "#ff6b6b"},
    {"label": "Shop", "color": "#4ecdc4"}
  ]
}
```

## PMTiles Serving

The Go server must support HTTP Range Requests for PMTiles:

```go
func (s *Server) handleTiles(w http.ResponseWriter, r *http.Request) {
    file := chi.URLParam(r, "file")
    path := filepath.Join(s.tilesDir, file)

    // Serve with range request support
    http.ServeFile(w, r, path)
}
```

## Datastar Integration

Datastar provides server-driven reactivity without heavy client-side JS:

```html
<script src="/static/js/vendor/datastar.js"></script>

<!-- Server sends HTML fragments -->
<div data-on-load="@get('/api/v1/layers')">
  <!-- Server responds with layer list HTML -->
</div>

<!-- Form submission -->
<form data-on-submit="@post('/api/v1/layers')">
  <input name="name" required>
  <button type="submit">Create Layer</button>
</form>
```

Server returns HTML fragments:
```go
func (s *Server) handleLayerList(w http.ResponseWriter, r *http.Request) {
    layers := s.db.GetLayers()
    s.templates.ExecuteTemplate(w, "layer-list.html", layers)
}
```

## Implementation Phases

### Phase 1: Basic Viewer
1. Create web/ directory structure
2. Copy vendor JS libraries
3. Create viewer.html template
4. Add tile serving endpoint
5. Add layer list endpoint

### Phase 2: Editor Foundation
1. Add Datastar library
2. Create editor.html template
3. Add layer CRUD endpoints
4. Implement style editor UI

### Phase 3: Tile Generation
1. Add generate endpoint
2. Integrate tiler package
3. Add progress tracking
4. Background job management

## Consequences

### Positive
- Self-contained web UI (no Hugo dependency)
- Datastar provides reactivity without React/Vue complexity
- Editors can configure layers without code changes
- Vendored JS = no CDN dependencies

### Negative
- More endpoints to maintain
- Need to handle PMTiles range requests
- Editor requires authentication (future)

### Neutral
- Separate viewer and editor concerns
- Can be extended with more editor features
- Compatible with existing tiler package

## References

- [Datastar](https://data-star.dev/) - Server-driven reactivity
- https://github.com/cbeauhilton/datastar-skills for claude !
- [Leaflet](https://leafletjs.com/) - Map library
- [protomaps-leaflet](https://github.com/protomaps/protomaps-leaflet) - Vector tiles
- [PMTiles](https://github.com/protomaps/PMTiles) - Tile format
- [ubuntu-website airspace-demo](https://github.com/joeblew999/ubuntu-website/blob/main/layouts/fleet/airspace-demo.html)
