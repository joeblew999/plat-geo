# ADR-003: DuckLake Catalog and Storage Configuration

## Status

Proposed

ref: https://ducklake.select/docs/stable/duckdb/usage/choosing_a_catalog_database

## Context

DuckLake provides a data lakehouse format that separates metadata catalog from data storage. For plat-geo's analytical workloads, we need to decide:

1. **Catalog database**: Where to store table metadata and schema
2. **Data storage**: Where to store Parquet data files
3. **Access patterns**: Single-user vs multi-user, local vs remote

DuckLake stores data as Parquet files with a log-structured approach where updates are modeled as "deletes followed by inserts."

## Decision Drivers

- **Concurrency**: Number of simultaneous clients
- **Access location**: Local vs remote access requirements
- **Operational complexity**: Infrastructure to maintain
- **Cost**: Storage and compute costs

## Catalog Database Options

### Option 1: DuckDB File (Single Client)

| Aspect | Details |
|--------|---------|
| Concurrency | Single client only |
| Setup | Minimal - just a `.ducklake` file |
| Use Case | Local development, single-user analytics |

**Configuration:**
```sql
ATTACH 'ducklake:my_ducklake.ducklake' AS my_ducklake;
```

**Pros:**
- Zero infrastructure required
- Native DuckDB integration
- Simple backup (single file)

**Cons:**
- Single client limitation
- No remote access

### Option 2: SQLite (Multiple Local Clients)

| Aspect | Details |
|--------|---------|
| Concurrency | Multiple local clients |
| Setup | SQLite file with retry logic |
| Use Case | Multi-process local analytics |

**Configuration:**
```sql
ATTACH 'ducklake:sqlite:my_catalog.db' AS my_ducklake (
    DATA_PATH './my_data/'
);
```

**Pros:**
- Supports multiple concurrent processes
- Automatic attach/detach with retry timeouts
- No server required

**Cons:**
- Local access only
- Limited to file system concurrency

### Option 3: PostgreSQL (Recommended for Production)

| Aspect | Details |
|--------|---------|
| Concurrency | Full multi-user |
| Setup | PostgreSQL 12+ server |
| Use Case | Multi-user lakehouse, remote clients |

**Configuration:**
```sql
ATTACH 'ducklake:postgres:host=localhost dbname=ducklake' AS my_ducklake (
    DATA_PATH 's3://my-bucket/my-data/'
);
```

**Pros:**
- Full client-server architecture
- Battle-tested concurrency
- Remote access support
- Integrates with existing Postgres infrastructure

**Cons:**
- Requires PostgreSQL server
- Additional operational overhead

### Option 4: MySQL (Not Recommended)

| Aspect | Details |
|--------|---------|
| Concurrency | Multi-user (theoretical) |
| Setup | MySQL 8+ server |
| Use Case | Organizations with MySQL infrastructure |

**Warning:** DuckLake documentation states there are "known issues with MySQL as a catalog" due to connector limitations.

## Data Storage Options

### Option 1: Local File System

**Configuration:**
```sql
ATTACH 'ducklake:my_ducklake.ducklake' AS my_ducklake (
    DATA_PATH './data/'
);
```

**Use case:** Development, single-machine analytics.

### Option 2: Cloudflare R2 (Recommended)

**Configuration:**
```sql
-- Create R2 secret
CREATE OR REPLACE SECRET r2_secret (
    TYPE r2,
    KEY_ID '${R2_ACCESS_KEY_ID}',
    SECRET '${R2_SECRET_ACCESS_KEY}',
    ACCOUNT_ID '${R2_ACCOUNT_ID}'
);

-- Attach DuckLake with R2 storage
ATTACH 'ducklake:postgres:host=localhost dbname=ducklake' AS my_ducklake (
    DATA_PATH 'r2://my-bucket/ducklake-data/'
);
```

**Pros:**
- Zero egress fees
- Native DuckDB support
- Works with R2 Data Catalog for Iceberg

**Cons:**
- Cloudflare ecosystem dependency

### Option 3: Amazon S3

**Configuration:**
```sql
-- Create S3 secret
CREATE OR REPLACE SECRET s3_secret (
    TYPE s3,
    PROVIDER credential_chain,
    REGION 'us-east-1'
);

-- Attach DuckLake with S3 storage
ATTACH 'ducklake:postgres:host=localhost dbname=ducklake' AS my_ducklake (
    DATA_PATH 's3://my-bucket/ducklake-data/'
);
```

**Pros:**
- Industry standard
- Mature ecosystem

**Cons:**
- High egress fees (~$0.09/GB)

### Option 4: Tigris Data

**Configuration:**
```sql
CREATE OR REPLACE PERSISTENT SECRET tigris (
    TYPE s3,
    PROVIDER config,
    KEY_ID '${TIGRIS_ACCESS_KEY_ID}',
    SECRET '${TIGRIS_SECRET_ACCESS_KEY}',
    REGION 'auto',
    ENDPOINT 't3.storage.dev',
    URL_STYLE 'vhost',
    SCOPE 's3://my-bucket'
);

ATTACH 'ducklake:postgres:host=localhost dbname=ducklake' AS my_ducklake (
    DATA_PATH 's3://my-bucket/ducklake-data/'
);
```

**Pros:**
- Zero egress fees
- Automatic multi-region distribution
- S3-compatible

### Option 5: MotherDuck (Managed)

**Configuration:**
```sql
-- MotherDuck as catalog, your S3 for storage
ATTACH 'ducklake:md:' AS my_ducklake (
    DATA_PATH 's3://my-bucket/ducklake-data/'
);
```

**Pros:**
- Managed catalog service
- Bring your own storage
- Hybrid local/cloud compute

**Cons:**
- MotherDuck subscription required
- Additional service dependency

## Comparison Matrix

| Configuration | Catalog | Storage | Concurrency | Best For |
|---------------|---------|---------|-------------|----------|
| Local DuckDB | DuckDB file | Local | Single | Development |
| SQLite + Local | SQLite | Local | Multi-process | CI/Testing |
| Postgres + R2 | PostgreSQL | R2 | Multi-user | Production |
| Postgres + S3 | PostgreSQL | S3 | Multi-user | AWS shops |
| MotherDuck + R2 | MotherDuck | R2 | Multi-user | Managed solution |

## Decision

### Development Environment
**DuckDB file catalog + Local storage**
```sql
ATTACH 'ducklake:.data/geo.ducklake' AS geo_lake;
```

### Production Environment
**PostgreSQL catalog + Cloudflare R2 storage**
```sql
-- Configure R2
CREATE OR REPLACE SECRET r2_secret (
    TYPE r2,
    KEY_ID '${R2_ACCESS_KEY_ID}',
    SECRET '${R2_SECRET_ACCESS_KEY}',
    ACCOUNT_ID '${R2_ACCOUNT_ID}'
);

-- Attach production DuckLake
ATTACH 'ducklake:postgres:host=${POSTGRES_HOST} dbname=ducklake' AS geo_lake (
    DATA_PATH 'r2://plat-geo-data/ducklake/'
);
```

**Rationale:**
1. PostgreSQL provides robust multi-user concurrency
2. R2 eliminates egress fees for analytical queries
3. Clear separation enables independent scaling

## DuckLake vs Iceberg Comparison

| Feature | DuckLake | Apache Iceberg |
|---------|----------|----------------|
| Native DuckDB | Yes | Via extension |
| Catalog options | DuckDB/SQLite/Postgres | REST/Glue/Hive |
| Maturity | 0.3 (maturing 2025) | Production-ready |
| Ecosystem | Growing | Extensive |
| R2 Data Catalog | No (use DuckLake direct) | Yes |

**Note:** For Iceberg tables in R2, use R2 Data Catalog. For DuckDB-native lakehouse, use DuckLake.

## Required Extensions

```sql
-- Core extensions
INSTALL httpfs;
INSTALL parquet;
INSTALL ducklake;

-- For S3/R2 access
INSTALL aws;

-- For PostgreSQL catalog
INSTALL postgres;

-- Load all
LOAD httpfs;
LOAD parquet;
LOAD ducklake;
LOAD aws;
LOAD postgres;
```

## Consequences

### Positive
- Separation of catalog and storage enables flexibility
- PostgreSQL catalog works with existing infrastructure
- R2 storage eliminates query egress costs
- ACID transactions on lakehouse data

### Negative
- PostgreSQL adds operational overhead
- DuckLake still maturing (v0.3)
- No primary key/foreign key constraints
- No indexes in DuckLake tables

### Neutral
- Data stored as immutable Parquet files
- Log-structured updates (deletes + inserts)
- Schema evolution supported

## DuckLake Limitations

DuckLake does not support:
- Indexes
- Primary keys
- Foreign keys
- UNIQUE constraints
- CHECK constraints

These reflect the data lake design philosophy - optimization happens via Parquet column statistics and partition pruning rather than traditional indexes.

## Migration Path

1. **Start local**: DuckDB file catalog + local storage
2. **Scale out**: Migrate catalog to PostgreSQL
3. **Go cloud**: Move data to R2/S3
4. **Multi-user**: Add authentication at PostgreSQL layer

## References

- [DuckLake Choosing a Catalog Database](https://ducklake.select/docs/stable/duckdb/usage/choosing_a_catalog_database)
- [DuckLake Choosing Storage](https://ducklake.select/docs/stable/duckdb/usage/choosing_storage)
- [DuckLake FAQ](https://ducklake.select/faq)
- [Getting Started with DuckLake (MotherDuck)](https://motherduck.com/blog/getting-started-ducklake-table-format/)
- [DuckLake with Tigris](https://www.tigrisdata.com/blog/ducklake/)
- [Frozen DuckLakes for Serverless Access](https://ducklake.select/2025/10/24/frozen-ducklake/)
