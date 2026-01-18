# ADR-007: HUGR GraphQL Data Mesh Layer

## Status

Proposed

ref: https://hugr-lab.github.io
ref: https://github.com/hugr-lab/hugr

## Context

plat-geo currently exposes data via REST endpoints with DuckDB as the analytical backend. As the system grows, we need:

1. **Unified API**: Single query interface across DuckDB, NornicDB, and external sources
2. **Flexible queries**: Clients request exactly the data they need
3. **Cross-source joins**: Combine data from multiple sources in one query
4. **Security**: Fine-grained access control at field and row level
5. **Performance**: Caching and optimized query execution

HUGR is an open-source Data Mesh platform that provides a GraphQL layer over DuckDB and other data sources, addressing these requirements.

## What is HUGR?

HUGR (Hyper Unified Graph Runtime) is a high-performance GraphQL backend built on DuckDB. It enables:

- **Unified GraphQL API** across distributed data sources
- **Native DuckDB integration** for OLAP workloads
- **Geospatial operations** including spatial joins and H3 clustering
- **Cross-source queries** combining databases, files, and APIs
- **Role-based access control** with field-level permissions

## Decision Drivers

- **Developer experience**: GraphQL provides typed, self-documenting APIs
- **Query flexibility**: Clients fetch exactly what they need (no over/under-fetching)
- **DuckDB native**: Uses DuckDB as calculation engine (our existing backend)
- **Geospatial support**: Built-in spatial operations match our use case
- **Open source**: Apache 2.0 licensed, can self-host

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Clients                                 │
│         (Web, Mobile, AI Agents, BI Tools)                  │
└────────────────────────┬────────────────────────────────────┘
                         │ GraphQL
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                        HUGR                                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                 GraphQL Layer                        │   │
│  │  - Schema generation from data sources              │   │
│  │  - Query parsing and optimization                   │   │
│  │  - Field-level RBAC                                 │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                 DuckDB Engine                        │   │
│  │  - OLAP query execution                             │   │
│  │  - Cross-source JOINs                               │   │
│  │  - Geospatial functions                             │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                    Caching                           │   │
│  │  - L1: In-memory (BigCache)                         │   │
│  │  - L2: Distributed (Redis/Memcached)                │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                         │
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   DuckDB    │  │ PostgreSQL  │  │  Parquet    │
│   Files     │  │  (PostGIS)  │  │  on S3/R2   │
└─────────────┘  └─────────────┘  └─────────────┘
```

## Supported Data Sources

| Source | Status | Notes |
|--------|--------|-------|
| DuckDB files | ✅ | Native support |
| PostgreSQL | ✅ | Including PostGIS, TimescaleDB |
| Parquet | ✅ | Local and cloud (S3) |
| CSV, JSON | ✅ | Via DuckDB |
| REST APIs | ✅ | OpenAPI v3 compatible |
| Iceberg | ✅ | Delta Lake supported |
| ESRI Shapefile | ✅ | Via DuckDB spatial |
| Cloudflare R2 | 🔜 | Planned |
| Azure Blob | 🔜 | Planned |
| GCS | 🔜 | Planned |

## Key Features for plat-geo

### 1. Geospatial Operations

HUGR provides native spatial operations:

```graphql
query NearbyPlaces {
  places(
    where: {
      geometry: {
        st_dwithin: {
          geometry: { type: "Point", coordinates: [-122.4, 37.8] }
          distance: 1000
        }
      }
    }
  ) {
    id
    name
    category
    geometry
  }
}
```

### 2. H3 Clustering

Built-in H3 hexagonal indexing:

```graphql
query PlacesByH3Cell {
  places_h3_aggregate(resolution: 7) {
    h3_cell
    count
    center_lat
    center_lng
  }
}
```

### 3. Cross-Source Queries

Join data from multiple sources in one query:

```graphql
query PlacesWithWeather {
  places {
    id
    name
    # From DuckDB
    geometry
    # From external API
    weather {
      temperature
      conditions
    }
  }
}
```

### 4. Role-Based Access Control

Fine-grained permissions:

```yaml
roles:
  public:
    types:
      Place:
        fields: [id, name, category]
        filter: "category != 'private'"
  admin:
    types:
      Place:
        fields: "*"
```

### 5. Two-Level Caching

```yaml
cache:
  l1:
    enabled: true
    size: 1GB
  l2:
    type: redis
    url: redis://localhost:6379
```

## Integration Options

### Option A: Standalone HUGR Service

Run HUGR as a separate service alongside plat-geo.

```yaml
# process-compose.yaml addition
hugr:
  command: hugr serve --config hugr.yaml
  working_dir: .
  availability:
    restart: on_failure
  readiness_probe:
    exec:
      command: curl -sf http://localhost:8080/health
```

**Pros:**
- Clean separation of concerns
- Independent scaling
- No changes to existing Go server

**Cons:**
- Additional service to operate
- Network hop between services

### Option B: Embedded in Go

HUGR can be embedded directly in Go services:

```go
import "github.com/hugr-lab/hugr/pkg/server"

func main() {
    cfg := server.Config{
        DataSources: []server.DataSource{
            {Type: "duckdb", Path: ".data/duckdb/geo.duckdb"},
        },
    }
    srv := server.New(cfg)
    srv.ListenAndServe(":8080")
}
```

**Pros:**
- Single binary deployment
- Lower latency
- Shared DuckDB connection

**Cons:**
- Tighter coupling
- Build complexity (CGO + DuckDB)

### Option C: Sidecar Pattern

Run HUGR as a sidecar in the same pod/container group.

**Pros:**
- Localhost communication
- Independent lifecycle
- Clean separation

**Cons:**
- Container orchestration required

## Recommended Approach

**Phase 1: Standalone Service (Option A)**

Start with HUGR as a separate service to evaluate the GraphQL layer without modifying the existing Go server.

```
┌─────────────────────────────────────────────────────────────┐
│                    process-compose                           │
├─────────────┬─────────────┬─────────────┬──────────────────┤
│    geo      │  duckdb-ui  │  nornicdb   │      hugr        │
│   :8086     │   :4213     │   :7474     │     :8080        │
└─────────────┴─────────────┴─────────────┴──────────────────┘
```

**Phase 2: Evaluate Embedding**

If latency is critical, consider embedding HUGR in the Go server.

## Configuration Example

```yaml
# hugr.yaml
server:
  port: 8080
  host: 0.0.0.0

datasources:
  - name: geo
    type: duckdb
    path: .data/duckdb/geo.duckdb

  - name: overture
    type: parquet
    path: .data/parquet/places.parquet

  - name: tiles
    type: file
    path: .data/pmtiles/

security:
  anonymous:
    enabled: true
    role: public
  api_keys:
    enabled: true

cache:
  l1:
    enabled: true
    max_size: 512MB
```

## GraphQL Schema Generation

HUGR auto-generates GraphQL schema from data sources:

```graphql
type Place {
  id: ID!
  name: String!
  category: String
  lat: Float!
  lng: Float!
  geometry: Geometry
}

type Query {
  places(where: PlaceFilter, limit: Int, offset: Int): [Place!]!
  places_aggregate(where: PlaceFilter): PlaceAggregate!
  place(id: ID!): Place
}

type Mutation {
  insert_place(input: PlaceInput!): Place!
  update_place(id: ID!, input: PlaceInput!): Place!
  delete_place(id: ID!): Boolean!
}
```

## AI/Agent Integration

HUGR supports MCP (Model Context Protocol) for AI agent integration:

```yaml
mcp:
  enabled: true
  tools:
    - query_places
    - search_nearby
    - aggregate_by_region
```

This enables AI agents to query geospatial data directly.

## Comparison with Alternatives

| Feature | HUGR | Hasura | PostGraphile |
|---------|------|--------|--------------|
| DuckDB native | ✅ | ❌ | ❌ |
| Parquet/S3 | ✅ | ❌ | ❌ |
| Geospatial | ✅ Built-in | Via PostGIS | Via PostGIS |
| Cross-source | ✅ | ❌ | ❌ |
| Open source | ✅ Apache 2.0 | Partial | ✅ |
| Self-hosted | ✅ | ✅ | ✅ |

## Consequences

### Positive
- Unified GraphQL API across all data sources
- DuckDB-native performance for analytics
- Built-in geospatial operations
- Flexible client queries
- Role-based access control
- Two-level caching

### Negative
- Additional service to operate
- Learning curve for GraphQL
- Schema management complexity
- CGO dependency (DuckDB)

### Neutral
- GraphQL is widely adopted
- Can run standalone or embedded
- Compatible with existing DuckDB setup

## Implementation Tasks

1. Add HUGR to process-compose.yaml
2. Create hugr.yaml configuration
3. Configure DuckDB data source
4. Set up authentication/authorization
5. Document GraphQL schema for clients
6. Add to Taskfile for management

## References

- [HUGR Documentation](https://hugr-lab.github.io)
- [HUGR GitHub](https://github.com/hugr-lab/hugr)
- [GraphQL Specification](https://graphql.org/)
- [DuckDB Documentation](https://duckdb.org/docs/)
