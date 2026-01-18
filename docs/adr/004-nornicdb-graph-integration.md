# ADR-004: NornicDB Graph Database Integration

## Status

Proposed

ref: https://github.com/orneryd/NornicDB

## Context

plat-geo serves geospatial data via DuckDB for analytics and PMTiles for map tiles. However, certain spatial problems are better solved with graph databases:

1. **Routing**: Finding paths between locations
2. **Relationships**: Modeling connections between places (roads, proximity, categories)
3. **Semantic search**: Finding similar places via vector embeddings
4. **Network analysis**: Understanding connectivity and centrality

NornicDB provides a Neo4j-compatible graph database with GPU acceleration and built-in vector search, making it suitable for spatial graph workloads.

## Decision Drivers

- **Query patterns**: Some queries are naturally graph-shaped (paths, relationships)
- **Performance**: Graph traversal is O(relationships) vs O(n²) for joins
- **AI integration**: Vector search enables semantic place discovery
- **Complementary**: Graph + analytical (DuckDB) covers more use cases

## Architecture Options

### Option A: POI Relationships (Recommended for Start)

Model points of interest and their relationships.

```
(:Place {id, name, lat, lng, category})
(:Place)-[:NEAR {distance}]->(:Place)
(:Place)-[:IN_CATEGORY]->(:Category)
(:Place)-[:CONNECTED_TO]->(:Place)
```

**Use cases:**
- Find nearby restaurants to a hotel
- Discover places in the same category
- Build POI networks for recommendations

**Pros:**
- Low implementation effort
- Immediate value for place discovery
- Works with existing DuckDB place data

**Cons:**
- Limited to POI-level relationships
- No road-level routing

### Option B: Semantic Search

Store place embeddings for similarity search.

```
(:Place {id, name, description, embedding})
```

**Use cases:**
- "Find places like this cozy coffee shop"
- Natural language place search
- Recommendation based on similarity

**Pros:**
- Leverages NornicDB's vector search
- AI-powered discovery
- Works with minimal graph structure

**Cons:**
- Requires embedding generation
- Quality depends on description data

### Option C: Road Network Routing

Model road network as graph for path finding.

```
(:Intersection {id, lat, lng})
(:Intersection)-[:ROAD {distance, duration, name}]->(:Intersection)
```

**Use cases:**
- Shortest path between locations
- Travel time estimation
- Route optimization

**Pros:**
- True routing capability
- Efficient path algorithms (Dijkstra, A*)
- Industry-standard approach

**Cons:**
- Requires road network data (OSM)
- Higher data volume
- More complex data pipeline

### Option D: Full Spatial Graph

Complete graph model with geometries and all relationships.

```
(:Place {id, name, geometry})
(:Road {id, name, geometry})
(:Region {id, name, geometry})
(:Place)-[:IN]->(:Region)
(:Road)-[:CONNECTS]->(:Place)
```

**Pros:**
- Complete spatial model
- Maximum flexibility

**Cons:**
- High implementation effort
- Complex data synchronization
- May duplicate DuckDB functionality

## Comparison Matrix

| Approach | Effort | Value | Data Required |
|----------|--------|-------|---------------|
| A. POI Relationships | Low | Medium | Places with coordinates |
| B. Semantic Search | Low | Medium | Places with descriptions |
| C. Road Network | Medium | High | OSM road data |
| D. Full Spatial Graph | High | High | All spatial data |

## Decision

**Phase 1: POI Relationships + Semantic Search (Options A + B)**

Start with low-effort, high-value integration:

1. Sync places from DuckDB to NornicDB as nodes
2. Create NEAR relationships based on distance
3. Enable vector search for semantic discovery

**Phase 2: Road Network Routing (Option C)**

Add routing capability once Phase 1 proves value:

1. Import OSM road data as graph
2. Implement shortest path queries
3. Integrate with place nodes

## Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│                        plat-geo API                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────┐         ┌─────────────────────────────┐   │
│  │   DuckDB    │         │         NornicDB             │   │
│  │             │  sync   │                              │   │
│  │  GeoParquet │───────▶│  Places (nodes)              │   │
│  │  Analytics  │         │  Relationships (edges)       │   │
│  │  Bulk ops   │         │  Vector embeddings           │   │
│  └─────────────┘         └─────────────────────────────┘   │
│        │                           │                        │
│        ▼                           ▼                        │
│  ┌─────────────┐         ┌─────────────────────────────┐   │
│  │  Spatial    │         │  Graph queries              │   │
│  │  queries    │         │  - Path finding             │   │
│  │  - Bounding │         │  - Nearby places            │   │
│  │    box      │         │  - Semantic search          │   │
│  │  - Distance │         │  - Network analysis         │   │
│  └─────────────┘         └─────────────────────────────┘   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## API Design

### Graph Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/graph/nearby` | GET | Find places near a location |
| `/api/v1/graph/search` | POST | Vector similarity search |
| `/api/v1/graph/path` | GET | Find path between places |
| `/api/v1/graph/related` | GET | Get related places |

### Example Queries

**Find nearby places:**
```cypher
MATCH (p:Place)
WHERE p.lat > $minLat AND p.lat < $maxLat
  AND p.lng > $minLng AND p.lng < $maxLng
RETURN p
LIMIT 100
```

**Find places connected to a location:**
```cypher
MATCH (start:Place {id: $placeId})-[:NEAR*1..2]-(nearby:Place)
RETURN nearby
```

**Semantic search:**
```bash
POST /nornicdb/search
{
  "query": "quiet cafe with outdoor seating",
  "limit": 10
}
```

## Sync Strategy

### DuckDB → NornicDB

1. **Initial load**: Bulk export places from DuckDB, import to NornicDB
2. **Incremental sync**: Trigger on DuckDB changes (via API or polling)
3. **Relationship generation**: Calculate NEAR relationships based on distance threshold

```sql
-- DuckDB: Export places for NornicDB
SELECT id, name, ST_Y(geometry) as lat, ST_X(geometry) as lng, category
FROM places
WHERE updated_at > $last_sync;
```

```cypher
// NornicDB: Create place nodes
UNWIND $places AS place
MERGE (p:Place {id: place.id})
SET p.name = place.name, p.lat = place.lat, p.lng = place.lng
```

```cypher
// NornicDB: Create NEAR relationships (within 1km)
MATCH (a:Place), (b:Place)
WHERE a.id < b.id
  AND point.distance(
    point({latitude: a.lat, longitude: a.lng}),
    point({latitude: b.lat, longitude: b.lng})
  ) < 1000
MERGE (a)-[:NEAR]->(b)
```

## NornicDB-Specific Features

### GPU Acceleration

NornicDB uses Metal on Apple Silicon for:
- Fast graph traversal
- Parallel vector operations
- Large-scale relationship processing

### Vector Search with BGE

The `bge-heimdall` image includes:
- BGE embedding model
- Automatic text embedding on node creation
- Cosine similarity search

```cypher
// Create node with auto-embedding
CREATE (p:Place {name: "Cozy Coffee Shop", description: "Quiet cafe with wifi"})

// Search by similarity
CALL nornic.vectorSearch("outdoor seating with view", 10)
YIELD node, score
RETURN node.name, score
```

### Memory Decay

NornicDB supports temporal relevance:
- Nodes can decay over time
- Fresher data gets higher priority
- Useful for trending places

## Implementation Tasks

### Phase 1 Tasks

1. Add NornicDB Go client to plat-geo
2. Create sync job: DuckDB → NornicDB places
3. Generate NEAR relationships
4. Implement `/api/v1/graph/nearby` endpoint
5. Implement `/api/v1/graph/search` endpoint

### Phase 2 Tasks

1. Import OSM road network
2. Build intersection graph
3. Implement shortest path API
4. Add turn-by-turn directions

## Consequences

### Positive

- Graph queries for relationship-based problems
- Vector search for semantic discovery
- GPU-accelerated performance on Apple Silicon
- Neo4j-compatible tooling (drivers, visualization)

### Negative

- Additional service to operate
- Data synchronization complexity
- Learning curve for Cypher
- Memory requirements for large graphs

### Neutral

- Complements rather than replaces DuckDB
- Standard Neo4j Bolt protocol
- Docker-based deployment

## Technology Comparison

| Aspect | DuckDB | NornicDB |
|--------|--------|----------|
| **Best for** | Analytics, bulk ops | Relationships, paths |
| **Query language** | SQL | Cypher |
| **Spatial** | ST_* functions | Point distance |
| **Storage** | Columnar (Parquet) | Graph (nodes/edges) |
| **Scaling** | Single node | Single node (GPU) |

## References

- [NornicDB GitHub](https://github.com/orneryd/NornicDB)
- [Neo4j Cypher Manual](https://neo4j.com/docs/cypher-manual/)
- [Neo4j Go Driver](https://github.com/neo4j/neo4j-go-driver)
- [BGE Embeddings](https://huggingface.co/BAAI/bge-base-en-v1.5)
