# ADR-008: Basemap Strategy with Protomaps

## Status

Proposed

ref: https://docs.protomaps.com/basemaps/downloads
ref: https://github.com/protomaps/basemaps

## Context

plat-geo needs basemaps for:
- Background map rendering (roads, water, land use, labels)
- Custom data visualization overlays
- Offline/self-hosted map serving
- Low-latency tile delivery

Traditional tile servers (Mapbox, Google Maps) have usage fees and require internet connectivity. Protomaps offers a self-hosted alternative using PMTiles.

## What is Protomaps Basemap?

Protomaps provides:
- **Daily OSM builds** as PMTiles (single-file tile archives)
- **Vector tiles** covering zoom levels 0-15
- **Multiple themes** (light, dark, grayscale, etc.)
- **MapLibre GL integration** for web rendering
- **Regional extraction** via CLI

## Decision Drivers

- **Self-hosted**: No external API dependencies
- **Cost**: Zero per-request fees after download
- **Performance**: Serve tiles from local storage or CDN
- **Customization**: Style tiles with any MapLibre theme
- **Freshness**: Daily OSM updates available

## Protomaps Basemap Features

### File Sizes

| Coverage | Size | Notes |
|----------|------|-------|
| Planet (z0-z15) | ~120 GB | All zoom levels |
| Planet (z0-z10) | ~5 GB | Low zoom only |
| Country (US) | ~15 GB | Full detail |
| City | ~100 MB | Regional extract |

### Built-in Themes

| Theme | Description |
|-------|-------------|
| `light` | Light background, standard colors |
| `dark` | Dark background, light features |
| `white` | Minimal, white background |
| `black` | Minimal, black background |
| `grayscale` | Neutral gray tones |

### Extraction Examples

```bash
# Extract specific region (San Francisco Bay Area)
pmtiles extract https://build.protomaps.com/20250115.pmtiles \
  bayarea.pmtiles \
  --bbox=-122.6,37.2,-121.7,38.0

# Limit zoom levels (reduces size by ~50% per zoom removed)
pmtiles extract planet.pmtiles region.pmtiles \
  --bbox=-122.6,37.2,-121.7,38.0 \
  --maxzoom=12
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Web Client                              │
│                   (MapLibre GL JS)                          │
└────────────────────────┬────────────────────────────────────┘
                         │ Vector Tiles
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                     plat-geo API                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              PMTiles Tile Server                     │   │
│  │  - HTTP Range Request support                       │   │
│  │  - Tile coordinate → byte range lookup              │   │
│  │  - Gzip/Brotli compression                          │   │
│  └─────────────────────────────────────────────────────┘   │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  .data/pmtiles/                              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  basemap.pmtiles  (~100 MB - 120 GB)                │   │
│  │  terrain.pmtiles  (optional, RGB elevation)         │   │
│  │  custom.pmtiles   (user overlays)                   │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Integration Options

### Option A: go-pmtiles Library (Recommended)

Use the official Go library to serve tiles directly from the plat-geo server.

```go
import "github.com/protomaps/go-pmtiles/pmtiles"

func main() {
    server, _ := pmtiles.NewServer(".data/pmtiles/")

    http.HandleFunc("/tiles/", func(w http.ResponseWriter, r *http.Request) {
        server.ServeHTTP(w, r)
    })
}
```

**Pros:**
- Single binary deployment
- Low latency (no external service)
- Direct file access

**Cons:**
- Increases server memory usage
- Handles tile serving load

### Option B: Standalone pmtiles serve

Run pmtiles CLI as a separate service.

```yaml
# process-compose.yaml
pmtiles:
  command: pmtiles serve .data/pmtiles/ --port 8081
  availability:
    restart: on_failure
```

**Pros:**
- Separation of concerns
- Easy to scale independently

**Cons:**
- Additional service
- Network hop

### Option C: Cloud Storage + CDN

Store PMTiles on Cloudflare R2 with browser-side range requests.

```javascript
// Client-side PMTiles loading
import { PMTiles } from 'pmtiles';
import maplibregl from 'maplibre-gl';

const protocol = new pmtiles.Protocol();
maplibregl.addProtocol('pmtiles', protocol.tile);

const map = new maplibregl.Map({
    style: {
        sources: {
            basemap: {
                type: 'vector',
                url: 'pmtiles://https://r2.example.com/basemap.pmtiles'
            }
        }
    }
});
```

**Pros:**
- Zero server-side tile handling
- Global CDN distribution
- Infinite scale

**Cons:**
- Requires cloud storage setup
- Client must support PMTiles protocol

## Recommended Approach

**Development/Small Deployments: Option A (go-pmtiles)**

Embed tile serving in plat-geo for simplicity.

**Production/Scale: Option C (R2 + CDN)**

Store PMTiles on Cloudflare R2 with zero egress fees (per ADR-001).

## Implementation Plan

### Phase 1: Local Development

1. Download regional basemap extract
2. Integrate go-pmtiles into plat-geo server
3. Add tile serving endpoint (`/tiles/{name}/{z}/{x}/{y}.mvt`)
4. Create basic MapLibre viewer

### Phase 2: Styling

1. Use Protomaps light/dark themes
2. Customize colors for brand consistency
3. Add sprites and fonts (self-hosted)

### Phase 3: Production

1. Upload basemap to Cloudflare R2
2. Configure CORS for browser access
3. Switch clients to direct R2 access
4. Add terrain tiles (optional)

## Taskfile Integration

```yaml
# taskfiles/Taskfile-pmtiles.yml
version: "3"

vars:
  DATA_DIR: '{{.DATA_DIR | default ".data"}}'
  PMTILES_DIR: '{{.DATA_DIR}}/pmtiles'
  BASEMAP_URL: 'https://build.protomaps.com'

tasks:
  install:
    desc: Install pmtiles CLI
    cmds:
      - |
        curl -L -o /tmp/pmtiles.tar.gz \
          https://github.com/protomaps/go-pmtiles/releases/latest/download/go-pmtiles_$(uname -s)_$(uname -m).tar.gz
        tar -xzf /tmp/pmtiles.tar.gz -C {{.BIN_DIR}}

  download:planet:
    desc: Download full planet basemap (~120GB)
    cmds:
      - mkdir -p {{.PMTILES_DIR}}
      - curl -o {{.PMTILES_DIR}}/planet.pmtiles {{.BASEMAP_URL}}/$(date +%Y%m%d).pmtiles

  extract:
    desc: Extract region (BBOX="-122.6,37.2,-121.7,38.0" NAME="bayarea")
    vars:
      BBOX: '{{.BBOX}}'
      NAME: '{{.NAME | default "region"}}'
      MAXZOOM: '{{.MAXZOOM | default "15"}}'
    cmds:
      - mkdir -p {{.PMTILES_DIR}}
      - |
        pmtiles extract {{.BASEMAP_URL}}/$(date +%Y%m%d).pmtiles \
          {{.PMTILES_DIR}}/{{.NAME}}.pmtiles \
          --bbox={{.BBOX}} \
          --maxzoom={{.MAXZOOM}}

  serve:
    desc: Serve PMTiles locally
    cmds:
      - pmtiles serve {{.PMTILES_DIR}} --port 8081

  info:
    desc: Show PMTiles file info
    vars:
      FILE: '{{.FILE | default "basemap.pmtiles"}}'
    cmds:
      - pmtiles show {{.PMTILES_DIR}}/{{.FILE}}

  verify:
    desc: Verify PMTiles file integrity
    vars:
      FILE: '{{.FILE | default "basemap.pmtiles"}}'
    cmds:
      - pmtiles verify {{.PMTILES_DIR}}/{{.FILE}}
```

## Client Integration

### MapLibre GL JS

```javascript
import maplibregl from 'maplibre-gl';
import { Protocol } from 'pmtiles';
import layers from 'protomaps-themes-base';

// Register PMTiles protocol
const protocol = new Protocol();
maplibregl.addProtocol('pmtiles', protocol.tile);

const map = new maplibregl.Map({
    container: 'map',
    style: {
        version: 8,
        glyphs: '/fonts/{fontstack}/{range}.pbf',
        sprite: '/sprites/light',
        sources: {
            protomaps: {
                type: 'vector',
                url: 'pmtiles:///tiles/basemap.pmtiles',
                attribution: '© OpenStreetMap'
            }
        },
        layers: layers('protomaps', 'light')
    }
});
```

### React / deck.gl

```javascript
import { PMTilesSource } from '@loaders.gl/pmtiles';
import { MVTLayer } from '@deck.gl/geo-layers';

const layer = new MVTLayer({
    data: new PMTilesSource({ url: '/tiles/basemap.pmtiles' }),
    // ... layer options
});
```

## Terrain Tiles (Optional)

Protomaps Mapterhorn provides RGB terrain tiles:

```bash
# Download terrain
pmtiles extract https://mapterhorn.protomaps.com/planet.pmtiles \
  .data/pmtiles/terrain.pmtiles \
  --bbox=-122.6,37.2,-121.7,38.0
```

Used for:
- 3D terrain visualization
- Hillshading
- Elevation queries

## Cost Comparison

| Approach | Storage | Bandwidth | Per-Request |
|----------|---------|-----------|-------------|
| Protomaps + R2 | ~$0.015/GB | $0 | $0 |
| Mapbox | N/A | N/A | $0.50/1000 |
| Google Maps | N/A | N/A | $2.00/1000 |

**Example**: 1M tile requests/month
- Protomaps + R2: ~$2 (storage only)
- Mapbox: ~$500
- Google Maps: ~$2,000

## Consequences

### Positive
- Zero per-request tile costs
- Self-hosted, no external dependencies
- Full control over styling
- Daily OSM updates available
- Works offline

### Negative
- Initial download large (~120GB for planet)
- Must manage updates manually
- Requires storage infrastructure
- No street-level imagery (OSM limitation)

### Neutral
- PMTiles is an open standard
- Can switch to other providers if needed
- Same data as other OSM-based maps

## References

- [Protomaps Basemap Downloads](https://docs.protomaps.com/basemaps/downloads)
- [Protomaps Getting Started](https://docs.protomaps.com/guide/getting-started)
- [Basemap Flavors/Themes](https://docs.protomaps.com/basemaps/flavors)
- [go-pmtiles Library](https://github.com/protomaps/go-pmtiles)
- [MapLibre GL JS](https://maplibre.org/maplibre-gl-js/docs/)
- [PMTiles Specification](https://github.com/protomaps/PMTiles)
- [Protomaps Basemaps Styles](https://github.com/protomaps/basemaps)
