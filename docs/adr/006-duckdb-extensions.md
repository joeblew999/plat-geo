# ADR-006: DuckDB Extensions Strategy

## Status

Proposed

ref: https://duckdb.org/community_extensions/list_of_extensions

## Context

DuckDB provides a rich ecosystem of extensions that expand its capabilities beyond core SQL analytics. For plat-geo's geospatial workloads, we need to evaluate and select extensions that enhance:

1. Spatial data processing
2. Cloud storage connectivity
3. Data format support
4. Analytics capabilities

Extensions come in two categories:
- **Core extensions**: Maintained by DuckDB Labs, shipped with DuckDB
- **Community extensions**: Third-party, installed from community repository

## Decision Drivers

- **Stability**: Core extensions preferred for critical functionality
- **Functionality**: Must solve real problems in our stack
- **Maintenance**: Active development and updates
- **Compatibility**: Works with current DuckDB version
- **Performance**: Doesn't degrade query performance

## Extension Categories

### 1. Geospatial Extensions (Essential)

#### spatial (Core)
| Aspect | Details |
|--------|---------|
| Type | Core extension |
| Purpose | Basic spatial types and functions |
| Install | `INSTALL spatial; LOAD spatial;` |

**Capabilities:**
- GEOMETRY types (Point, LineString, Polygon, etc.)
- ST_* functions (ST_Distance, ST_Contains, ST_Intersects)
- WKT/WKB parsing
- Coordinate transformations

**Use in plat-geo:** Core spatial queries, geometry operations.

#### geography (Community - Recommended)
| Aspect | Details |
|--------|---------|
| Type | Community extension |
| Purpose | Global spatial processing on the sphere |
| Install | `INSTALL geography FROM community; LOAD geography;` |

**Capabilities:**
- Spherical geometry (great circle distances)
- Global coordinate handling
- More accurate for large-scale geo data

**Use in plat-geo:** Accurate distance calculations, global data processing.

#### h3 (Community)
| Aspect | Details |
|--------|---------|
| Type | Community extension |
| Purpose | Hierarchical hexagonal spatial indexing |
| Install | `INSTALL h3 FROM community; LOAD h3;` |

**Capabilities:**
- H3 cell encoding/decoding
- Hierarchical aggregation
- Neighbor finding
- Efficient spatial joins

**Use in plat-geo:** Spatial indexing, aggregation by hexagonal cells.

```sql
-- Example: Count places per H3 cell
SELECT h3_cell_to_lat_lng(h3_lat_lng_to_cell(lat, lng, 7)) as center,
       count(*) as place_count
FROM places
GROUP BY 1;
```

#### valhalla_routing (Community)
| Aspect | Details |
|--------|---------|
| Type | Community extension |
| Purpose | Routing and travel time calculations |
| Install | `INSTALL valhalla_routing FROM community;` |

**Capabilities:**
- Route calculation
- Travel time estimation
- Turn-by-turn directions

**Use in plat-geo:** Routing queries without external API calls.

### 2. Cloud Storage Extensions (Essential)

#### httpfs (Core)
| Aspect | Details |
|--------|---------|
| Type | Core extension |
| Purpose | HTTP/S3 remote file access |
| Install | `INSTALL httpfs; LOAD httpfs;` |

**Capabilities:**
- Read remote Parquet/CSV files
- S3, GCS, Azure Blob support
- HTTP Range requests

**Use in plat-geo:** Query Overture Maps data directly from S3.

#### aws (Core)
| Aspect | Details |
|--------|---------|
| Type | Core extension |
| Purpose | AWS credential management |
| Install | `INSTALL aws; LOAD aws;` |

**Use in plat-geo:** Authenticate to S3 for Overture Maps, R2.

### 3. Data Format Extensions

#### parquet (Core - Essential)
| Aspect | Details |
|--------|---------|
| Type | Core extension |
| Purpose | Parquet file support |
| Install | `INSTALL parquet; LOAD parquet;` |

**Use in plat-geo:** All GeoParquet data ingestion.

#### json (Core)
| Aspect | Details |
|--------|---------|
| Type | Core extension |
| Purpose | JSON parsing and querying |
| Install | Built-in |

**Use in plat-geo:** Parse JSON properties in geo data.

#### excel (Core)
| Aspect | Details |
|--------|---------|
| Type | Core extension |
| Purpose | Read Excel files |
| Install | `INSTALL excel; LOAD excel;` |

**Use in plat-geo:** Import user-provided spreadsheets with locations.

### 4. Analytics Extensions

#### cache_httpfs (Community - Recommended)
| Aspect | Details |
|--------|---------|
| Type | Community extension |
| Purpose | Caching for remote data |
| Install | `INSTALL cache_httpfs FROM community; LOAD cache_httpfs;` |

**Capabilities:**
- Disk/memory caching for remote Parquet
- Metadata caching
- Glob result caching

**Use in plat-geo:** Speed up repeated queries on remote data.

```sql
SET cache_httpfs_type = 'on_disk';
SET cache_httpfs_cache_directory = '.data/cache';
```

### 5. Other Useful Extensions

#### ui (Core)
| Aspect | Details |
|--------|---------|
| Type | Core extension |
| Purpose | Web-based SQL interface |
| Install | `INSTALL ui; LOAD ui;` |

**Use in plat-geo:** Interactive data exploration.

#### fts (Core)
| Aspect | Details |
|--------|---------|
| Type | Core extension |
| Purpose | Full-text search |
| Install | `INSTALL fts; LOAD fts;` |

**Use in plat-geo:** Search place names and descriptions.

```sql
-- Create FTS index
PRAGMA create_fts_index('places', 'id', 'name', 'description');

-- Search
SELECT * FROM places WHERE fts_match('places', 'coffee shop');
```

## Extension Selection Matrix

| Extension | Type | Priority | Use Case |
|-----------|------|----------|----------|
| spatial | Core | **Essential** | Basic geo operations |
| parquet | Core | **Essential** | Data ingestion |
| httpfs | Core | **Essential** | Remote data access |
| aws | Core | **Essential** | Cloud authentication |
| geography | Community | **Recommended** | Spherical calculations |
| cache_httpfs | Community | **Recommended** | Query caching |
| h3 | Community | Optional | Spatial indexing |
| ui | Core | Optional | Development |
| fts | Core | Optional | Place search |
| valhalla_routing | Community | Future | Routing |

## Decision

### Essential Extensions (Always Load)

```sql
-- Core extensions
INSTALL spatial; LOAD spatial;
INSTALL parquet; LOAD parquet;
INSTALL httpfs; LOAD httpfs;
INSTALL aws; LOAD aws;

-- Recommended community
INSTALL geography FROM community; LOAD geography;
INSTALL cache_httpfs FROM community; LOAD cache_httpfs;
```

### Extension Loading in Go

```go
// internal/db/duckdb.go
extensions := []string{
    // Core
    "spatial",
    "parquet",
    "httpfs",
    "aws",
}

communityExtensions := []string{
    "geography",
    "cache_httpfs",
}

for _, ext := range extensions {
    db.Exec(fmt.Sprintf("INSTALL %s; LOAD %s;", ext, ext))
}

for _, ext := range communityExtensions {
    db.Exec(fmt.Sprintf("INSTALL %s FROM community; LOAD %s;", ext, ext))
}
```

### Taskfile Extension Management

```yaml
# taskfiles/Taskfile-duckdb.yml additions

extensions:essential:
  desc: Install essential extensions
  cmds:
    - '{{.DUCKDB}} -c "INSTALL spatial; INSTALL parquet; INSTALL httpfs; INSTALL aws;"'
    - '{{.DUCKDB}} -c "INSTALL geography FROM community;"'
    - '{{.DUCKDB}} -c "INSTALL cache_httpfs FROM community;"'

extensions:all:
  desc: Install all recommended extensions
  cmds:
    - task: extensions:essential
    - '{{.DUCKDB}} -c "INSTALL h3 FROM community;"'
    - '{{.DUCKDB}} -c "INSTALL fts;"'
    - '{{.DUCKDB}} -c "INSTALL ui;"'
```

## Notable Community Extensions Not Selected

| Extension | Reason for Exclusion |
|-----------|---------------------|
| bigquery | We use DuckDB as primary, not BigQuery |
| snowflake | Not in our stack |
| mongo | NornicDB for graph, not MongoDB |
| pdal | Point clouds not in initial scope |
| a5 | H3 more widely adopted |

## Extension Version Compatibility

Extensions should be updated when DuckDB is updated:

```sql
-- Check installed extensions
SELECT * FROM duckdb_extensions();

-- Update extensions
UPDATE EXTENSIONS;
```

## Consequences

### Positive
- Rich geospatial capabilities via extensions
- Remote data access without downloads
- Caching improves query performance
- Community extensions add cutting-edge features

### Negative
- Community extensions may have breaking changes
- More extensions = larger memory footprint
- Extension conflicts possible (rare)
- Version compatibility requires attention

### Neutral
- Extensions loaded on-demand
- Core extensions are stable
- Can add/remove extensions as needs change

## Future Considerations

| Extension | When to Add |
|-----------|-------------|
| valhalla_routing | When implementing routing |
| duckgl | For visualization features |
| st_read_multi | For batch geospatial file loading |

## References

- [DuckDB Extensions Overview](https://duckdb.org/docs/extensions/overview)
- [DuckDB Community Extensions](https://duckdb.org/community_extensions/list_of_extensions)
- [Spatial Extension Docs](https://duckdb.org/docs/extensions/spatial/overview)
- [H3 Extension](https://github.com/isaacbrodsky/h3-duckdb)
- [cache_httpfs Extension](https://community-extensions.duckdb.org/extensions/cache_httpfs.html)
