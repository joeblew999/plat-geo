# ADR-010: Geospatial Tiling System

## Status

Proposed

## Context

plat-geo needs tile generation capabilities for any geospatial data. The ubuntu-website repository has a mature tiling system (originally built for FAA airspace data) that includes:

- Data download and sync with ETag-based change detection
- Dual tile generation (tippecanoe CLI + pure Go gotiler)
- PMTiles output (WASM-compatible writer)
- Cloudflare R2 upload via wrangler
- Manifest generation for frontend consumption
- GitHub Actions CI pipeline

Rather than rebuilding this functionality, we should port and generalize this code for plat-geo.

## Decision

Port the tiling system from ubuntu-website to plat-geo, generalizing it for any geospatial data (places, buildings, routes, boundaries, etc.) rather than just FAA airspace.

## Source Repository Structure

From `github.com/joeblew999/ubuntu-website`:

```
internal/
├── airspace/
│   ├── config.go           # Directory paths, R2 endpoints, layer names
│   ├── dataset.go          # Data source definitions
│   ├── download.go         # Direct + paginated GeoJSON downloads
│   ├── sync.go             # ETag-based change detection
│   ├── pipeline.go         # Sync → tile → manifest orchestration
│   ├── tiler.go            # Tiler interface
│   ├── manifest.go         # Manifest generation
│   ├── upload.go           # R2 upload via wrangler
│   ├── ci.go               # GitHub Actions integration
│   ├── tiler/
│   │   └── tippecanoe.go   # Tippecanoe CLI wrapper
│   ├── gotiler/
│   │   └── gotiler.go      # Pure Go tile generator (1200+ lines)
│   └── testdata/
│       └── *.geojson       # Test data
├── pmtiles/
│   └── pmtiles.go          # Custom PMTiles v3 writer (WASM-safe)
cmd/
└── airspace/
    └── main.go             # CLI with subcommands
```

## Target Structure in plat-geo

```
internal/
├── tiler/
│   ├── tiler.go            # Tiler interface (generalized)
│   ├── config.go           # TileConfig struct
│   ├── tippecanoe.go       # Tippecanoe CLI wrapper
│   └── gotiler/
│       └── gotiler.go      # Pure Go tile generator
├── pmtiles/
│   └── pmtiles.go          # PMTiles v3 writer
├── sync/
│   ├── sync.go             # ETag-based change detection
│   ├── download.go         # GeoJSON download helpers
│   └── etag.go             # ETag store
├── pipeline/
│   ├── pipeline.go         # Orchestration
│   └── manifest.go         # Manifest generation
└── upload/
    └── r2.go               # Cloudflare R2 upload
cmd/
├── geo/                    # Existing server
└── tiler/                  # New CLI for tile operations
    └── main.go
taskfiles/
├── Taskfile-tippecanoe.yml # Already exists
├── Taskfile-pmtiles.yml    # Already exists
└── Taskfile-tiler.yml      # New: pipeline tasks
```

## Key Components to Port

### 1. Tiler Interface & Implementations

**Source:** `internal/airspace/tiler.go` + `tiler/tippecanoe.go`

```go
// TileConfig holds settings for tile generation
type TileConfig struct {
    MinZoom         int
    MaxZoom         int
    Layer           string
    DropDensest     bool
    NoFeatureLimit  bool
    NoTileSizeLimit bool
    ReduceRate      int
}

// Tiler generates PMTiles from GeoJSON
type Tiler interface {
    Tile(inputPath, outputPath string, config TileConfig) error
    Name() string
    Available() bool
}
```

### 2. GoTiler (Pure Go, WASM-Compatible)

**Source:** `internal/airspace/gotiler/gotiler.go` (~1200 lines)

Key features:
- No external dependencies (no tippecanoe binary needed)
- WASM-compatible (no SQLite)
- Uses `github.com/paulmach/orb` for geometry
- Generates MVT tiles with adaptive simplification
- Outputs clustered PMTiles format

### 3. PMTiles Writer

**Source:** `internal/pmtiles/pmtiles.go`

Custom minimal implementation:
- PMTiles v3 spec compliant
- Hilbert curve tile ID encoding
- No SQLite dependency (unlike go-pmtiles)
- WASM-safe for Cloudflare Workers

### 4. Sync System

**Source:** `internal/airspace/sync.go` + `download.go`

- ETag-based change detection
- Paginated API support (ArcGIS FeatureServer, Overture, etc.)
- Sync history tracking (rolling 20-run window)
- Idempotent pipeline execution

### 5. Upload System

**Source:** `internal/airspace/upload.go`

- Wrangler CLI wrapper for R2 uploads
- Manifest-driven uploads (only files in manifest)
- Configurable bucket and paths

## Migration Steps

### Phase 1: Copy Core Packages

```bash
# From ubuntu-website repo root
cp -r internal/pmtiles /path/to/plat-geo/internal/
cp internal/airspace/tiler.go /path/to/plat-geo/internal/tiler/
cp -r internal/airspace/tiler/* /path/to/plat-geo/internal/tiler/
cp -r internal/airspace/gotiler /path/to/plat-geo/internal/tiler/
```

### Phase 2: Update Import Paths

Change all imports from:
```go
import "github.com/joeblew999/ubuntu-website/internal/..."
```
To:
```go
import "github.com/joeblew999/plat-geo/internal/..."
```

### Phase 3: Add Dependencies

```bash
go get github.com/paulmach/orb@v0.12.0
```

### Phase 4: Generalize Dataset Configuration

Create a generic dataset configuration system:

```go
// Dataset represents any geospatial data source
type Dataset struct {
    Name       string
    SourceURL  string
    SourceType string // "geojson", "geoparquet", "arcgis", "overture"
    TileConfig TileConfig
}
```

### Phase 5: Create CLI

New `cmd/tiler/main.go` with subcommands:
- `tiler generate` - GeoJSON → PMTiles
- `tiler sync` - Download with change detection
- `tiler upload` - Upload to R2
- `tiler pipeline` - Full workflow

### Phase 6: Add Taskfile

```yaml
# taskfiles/Taskfile-tiler.yml
version: "3"

tasks:
  generate:
    desc: Generate tiles from GeoJSON (FILE=input.geojson OUTPUT=output.pmtiles)
    cmds:
      - go run ./cmd/tiler generate -input {{.FILE}} -output {{.OUTPUT}}

  pipeline:
    desc: Run full sync → tile → upload pipeline
    cmds:
      - go run ./cmd/tiler pipeline

  upload:
    desc: Upload tiles to R2
    cmds:
      - go run ./cmd/tiler upload
```

## Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Data Sources                              │
├─────────────────┬─────────────────┬─────────────────────────┤
│   DuckDB        │   External API  │      GeoJSON            │
│   GeoParquet    │   (Overture)    │      Uploads            │
└────────┬────────┴────────┬────────┴────────┬────────────────┘
         │                 │                 │
         ▼                 ▼                 ▼
┌─────────────────────────────────────────────────────────────┐
│                    Sync Layer                                │
│  - ETag-based change detection                              │
│  - Paginated downloads                                      │
│  - Sync history tracking                                    │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Tiler                                     │
│  ┌─────────────────┐  ┌─────────────────┐                   │
│  │   Tippecanoe    │  │    GoTiler      │                   │
│  │   (CLI)         │  │   (Pure Go)     │                   │
│  └─────────────────┘  └─────────────────┘                   │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  .data/pmtiles/                              │
│  ├── basemap.pmtiles      (Protomaps OSM - ADR-008)         │
│  ├── places.pmtiles       (Generated from DuckDB)           │
│  ├── buildings.pmtiles    (Generated from Overture)         │
│  └── custom.pmtiles       (User uploads)                    │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Upload                                    │
│  - Cloudflare R2 via wrangler                               │
│  - Manifest-driven                                          │
│  - Public CDN URLs                                          │
└─────────────────────────────────────────────────────────────┘
```

## Integration with Existing ADRs

| ADR | Integration |
|-----|-------------|
| ADR-001 (PMTiles Cloud Storage) | Upload system uses R2 |
| ADR-002 (DuckDB Storage) | Export DuckDB → GeoJSON → tiles |
| ADR-005 (Geospatial Data Sources) | Sync layer downloads from Overture, OSM |
| ADR-006 (DuckDB Extensions) | Spatial extension for geometry export |
| ADR-008 (Basemap Strategy) | Generated tiles overlay basemaps |
| ADR-009 (Tippecanoe) | Tippecanoe wrapper implementation |

## Tiler Selection Strategy

```go
func SelectTiler() Tiler {
    // Try tippecanoe first (faster for large datasets)
    tip := tippecanoe.New()
    if tip.Available() {
        return tip
    }
    // Fallback to pure Go (no external deps)
    return gotiler.New()
}
```

| Tiler | Pros | Cons |
|-------|------|------|
| **Tippecanoe** | Fast, battle-tested | External binary required |
| **GoTiler** | Pure Go, WASM-ready, no deps | Slower for large datasets |

## Cloudflare R2 Configuration

```yaml
# Environment variables
CLOUDFLARE_API_TOKEN: <token with R2 permissions>
R2_BUCKET: plat-geo-assets
R2_PUBLIC_URL: https://pub-xxx.r2.dev

# Upload paths
/tiles/places.pmtiles
/tiles/buildings.pmtiles
/manifests/manifest.json
```

## Use Cases

### 1. Places from DuckDB

```bash
# Export from DuckDB to GeoJSON
task duckdb:query -- -c "COPY (SELECT ...) TO '/tmp/places.geojsonl'"

# Generate tiles
task tiler:generate FILE=/tmp/places.geojsonl OUTPUT=places
```

### 2. Buildings from Overture

```bash
# Sync Overture buildings data
task tiler:sync DATASET=overture-buildings

# Generate tiles
task tiler:generate FILE=.data/geojson/buildings.geojson OUTPUT=buildings
```

### 3. Custom User Upload

```bash
# Generate tiles from uploaded GeoJSON
task tiler:generate FILE=uploads/custom.geojson OUTPUT=custom
```

## Consequences

### Positive

- Reuse battle-tested tile generation code
- Dual tiler strategy (CLI + pure Go)
- WASM-compatible for edge deployment
- Smart sync with ETag caching
- Seamless R2 integration
- Works with any geospatial data source

### Negative

- ~1500 lines of code to maintain
- GoTiler slower than tippecanoe for large datasets
- Wrangler CLI dependency for uploads

### Neutral

- Same PMTiles format as basemaps
- Generalizes to any GeoJSON source
- Can add domain-specific modules (airspace, transit, etc.) later

## Files to Copy

```bash
# Essential (must copy)
internal/airspace/tiler.go           → internal/tiler/tiler.go
internal/airspace/tiler/tippecanoe.go → internal/tiler/tippecanoe.go
internal/airspace/gotiler/gotiler.go → internal/tiler/gotiler/gotiler.go
internal/pmtiles/pmtiles.go          → internal/pmtiles/pmtiles.go

# Sync system (recommended)
internal/airspace/sync.go            → internal/sync/sync.go
internal/airspace/download.go        → internal/sync/download.go

# Upload system (recommended)
internal/airspace/upload.go          → internal/upload/r2.go

# Tests
internal/airspace/testdata/          → internal/tiler/testdata/
```

## References

- [ubuntu-website tiler code](https://github.com/joeblew999/ubuntu-website/tree/main/internal/airspace)
- [paulmach/orb](https://github.com/paulmach/orb) - Geometry library
- [PMTiles Spec](https://github.com/protomaps/PMTiles/blob/main/spec/v3/spec.md)
- [Tippecanoe](https://github.com/felt/tippecanoe)
- [Wrangler R2](https://developers.cloudflare.com/r2/api/workers/workers-api-usage/)
