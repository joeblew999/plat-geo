# ADR-002: DuckDB Storage and Caching Strategy

## Status

Proposed

ref: https://duckdb.org/docs/stable/core_extensions/httpfs/s3api.html

## Context

plat-geo uses DuckDB for analytical queries on geospatial data stored in Parquet format. When querying remote data on S3-compatible storage, we need strategies to minimize:

1. **Latency**: Cold starts on object storage can be slow
2. **Bandwidth costs**: Egress fees can dominate costs at scale
3. **Repeated fetches**: Container restarts lose in-memory cache

DuckDB provides built-in S3 support via the `httpfs` extension, with additional caching extensions available.

## Decision Drivers

- **Query performance**: Minimize time-to-first-result for analytical queries
- **Cost efficiency**: Reduce egress fees and API request costs
- **Operational simplicity**: Prefer solutions with low maintenance overhead
- **Data freshness**: Balance caching with data currency requirements

## Storage Provider Options

### Option 1: Cloudflare R2 (Recommended)

| Aspect | Details |
|--------|---------|
| Egress Fees | **None** |
| DuckDB Support | Native via `TYPE r2` secret |
| API Compatibility | Full S3 API |
| Data Catalog | R2 Data Catalog (Iceberg) |

**Configuration:**
```sql
CREATE OR REPLACE SECRET (
    TYPE r2,
    KEY_ID 'your-key',
    SECRET 'your-secret',
    ACCOUNT_ID 'your-account-id'
);
SELECT * FROM 'r2://bucket/data.parquet';
```

**Pros:**
- Zero egress fees make remote queries cost-effective
- Native DuckDB support with `r2://` URL scheme
- R2 Data Catalog provides managed Iceberg tables
- R2 SQL offers serverless distributed queries

**Cons:**
- Cloudflare ecosystem lock-in
- R2 SQL still in beta

### Option 2: Amazon S3

| Aspect | Details |
|--------|---------|
| Egress Fees | ~$0.09/GB |
| DuckDB Support | Native via `TYPE s3` secret |
| API Compatibility | Reference S3 API |
| Data Catalog | AWS Glue / Lake Formation |

**Configuration:**
```sql
CREATE OR REPLACE SECRET (
    TYPE s3,
    PROVIDER credential_chain,
    REGION 'us-east-1'
);
SELECT * FROM 's3://bucket/data.parquet';
```

**Pros:**
- Industry standard with mature ecosystem
- Multiple authentication methods (IAM, STS, SSO)
- KMS encryption support
- Requester pays buckets available

**Cons:**
- High egress fees at scale
- Complex IAM configuration

### Option 3: Google Cloud Storage

| Aspect | Details |
|--------|---------|
| Egress Fees | ~$0.12/GB |
| DuckDB Support | Native via `TYPE gcs` secret |
| API Compatibility | S3 interoperability mode |
| Data Catalog | BigLake / Dataplex |

**Configuration:**
```sql
CREATE OR REPLACE SECRET (
    TYPE gcs,
    KEY_ID 'hmac-access-id',
    SECRET 'hmac-secret-key'
);
SELECT * FROM 'gs://bucket/data.parquet';
```

**Pros:**
- Native HTTP/2 support
- Good integration with BigQuery
- HMAC keys for S3 compatibility

**Cons:**
- Higher egress fees than AWS
- Requires HMAC key setup for DuckDB

### Option 4: Tigris Data

| Aspect | Details |
|--------|---------|
| Egress Fees | **None** |
| DuckDB Support | S3-compatible |
| API Compatibility | Full S3 API |
| Distribution | Automatic multi-region |

**Pros:**
- Zero egress fees
- Automatic global distribution
- S3-compatible API

**Cons:**
- Newer provider
- Smaller community

## Caching Strategy Options

### Option A: cache_httpfs Extension (Recommended)

Community extension that adds disk/memory caching for remote Parquet files.

**Installation:**
```sql
INSTALL cache_httpfs FROM community;
LOAD cache_httpfs;
```

**Configuration:**
```sql
-- On-disk caching (survives restarts)
SET cache_httpfs_type = 'on_disk';
SET cache_httpfs_cache_directory = '/tmp/duckdb_cache';

-- In-memory caching (faster, ephemeral)
SET cache_httpfs_type = 'in_mem';

-- Disable caching
SET cache_httpfs_type = 'noop';
```

**Features:**
- Caches Parquet metadata and data blocks
- Caches glob/list operation results
- Supports `s3://`, `r2://`, `hf://` URL schemes
- Leverages Parquet bloom filters for minimal data transfer

**Performance:**
- Query times reduced from 100s to 40s in benchmarks
- Metadata caching eliminates repeated HEAD requests
- Parallel reads with tunable request size

### Option B: DiskCache Extension

Extends DuckDB's RAM cache with SSD spill-over.

**Features:**
- Adds disk layer under RAM cache
- Data spills to SSD when RAM fills
- Reduces network fetches on cache eviction

**Use case:** Memory-constrained environments with fast local SSD.

### Option C: Quackstore Protocol

Cache-through protocol for remote files.

**Usage:**
```sql
SELECT * FROM parquet_scan('quackstore://s3://bucket/data.parquet');
SELECT * FROM iceberg_scan('quackstore://s3://bucket/iceberg/catalog');
```

**Use case:** Iceberg tables requiring consistent caching.

### Option D: enable_object_cache (Built-in)

DuckDB's built-in Parquet metadata caching.

```sql
SET enable_object_cache = true;
```

**Limitation:** Only caches file metadata, not data blocks.

## Comparison Matrix

| Caching Option | Data Cached | Persistence | Setup Complexity |
|----------------|-------------|-------------|------------------|
| cache_httpfs | Metadata + Data | Disk/Memory | Low |
| DiskCache | Data blocks | Disk | Low |
| Quackstore | Full files | Configurable | Medium |
| enable_object_cache | Metadata only | Memory | None |

## Decision

### Storage Provider
**Recommended: Cloudflare R2** for zero egress fees and native DuckDB support.

**Alternative: Tigris Data** for zero egress without Cloudflare lock-in.

### Caching Strategy
**Recommended: cache_httpfs extension** with on-disk caching for production.

**Configuration for plat-geo:**
```sql
-- Install caching extension
INSTALL cache_httpfs FROM community;
LOAD cache_httpfs;

-- Configure persistent disk cache
SET cache_httpfs_type = 'on_disk';
SET cache_httpfs_cache_directory = '.data/cache';

-- Configure R2 storage
CREATE OR REPLACE SECRET (
    TYPE r2,
    KEY_ID '${R2_ACCESS_KEY_ID}',
    SECRET '${R2_SECRET_ACCESS_KEY}',
    ACCOUNT_ID '${R2_ACCOUNT_ID}'
);
```

## Cloudflare R2 Data Platform

Cloudflare now offers a complete data platform:

| Component | Function |
|-----------|----------|
| **R2 Storage** | S3-compatible object storage, zero egress |
| **R2 Data Catalog** | Managed Apache Iceberg catalog |
| **R2 SQL** | Serverless distributed query engine |
| **Pipelines** | Event ingestion and transformation |

**Benefits:**
- Iceberg tables with ACID transactions
- Schema evolution without data rewriting
- Connect any query engine (Spark, DuckDB, PyIceberg)
- Zero egress for cross-cloud/region queries

**DuckDB Integration:**
```sql
-- Query Iceberg tables via R2 Data Catalog
-- Use PyIceberg or native Iceberg extension
```

## Cost Estimation

For 100GB dataset with 10,000 queries/month:

| Provider | Storage | Queries | Egress (10TB) | Total |
|----------|---------|---------|---------------|-------|
| Cloudflare R2 | ~$1.50 | ~$3.60 | $0 | ~$5 |
| Amazon S3 | ~$2.30 | ~$4.00 | ~$900 | ~$906 |
| With cache_httpfs | - | - | ~90% reduction | - |

## Consequences

### Positive
- Predictable costs with zero-egress storage
- Fast repeated queries via caching
- Warm starts after container restarts (disk cache)

### Negative
- Cache invalidation requires manual management
- Disk cache consumes local storage
- Extension dependencies (community extensions)

### Neutral
- All S3-compatible providers work with DuckDB
- Caching benefits depend on query patterns

## References

- [DuckDB S3 API Documentation](https://duckdb.org/docs/stable/core_extensions/httpfs/s3api.html)
- [cache_httpfs Extension](https://medium.com/@komalbaparmar007/duckdb-object-store-caching-parquet-metadata-statistics-and-millisecond-warm-starts-ce2afd9bc33f)
- [MotherDuck OLAP Caching Blog](https://motherduck.com/blog/duckdb-olap-caching/)
- [Cloudflare R2 Data Catalog](https://developers.cloudflare.com/r2/data-catalog/)
- [Cloudflare R2 SQL](https://developers.cloudflare.com/r2-sql/)
- [DuckDB Persistent Caches](https://medium.com/@hadiyolworld007/duckdb-persistent-caches-on-object-storage-warm-starts-for-petabyte-lakes-a50e91bb2033)
