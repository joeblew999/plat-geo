# ADR-001: PMTiles Cloud Storage Provider Selection

## Status

Proposed

ref: https://docs.protomaps.com/pmtiles/cloud-storage


## Context

plat-geo serves PMTiles for map tile delivery. PMTiles uses HTTP Range Requests to fetch only the needed tile data from a single archive file. This requires cloud storage providers that properly support:

1. HTTP Range Requests
2. CORS headers (for browser-based access)
3. Exposed ETag headers (for caching validation)

We need to evaluate cloud storage options based on cost, performance, and operational complexity.

## Decision Drivers

- **Cost**: Bandwidth/egress fees can dominate costs for tile serving
- **Performance**: HTTP/2 support enables multiplexed requests
- **Simplicity**: CORS configuration complexity varies by provider
- **Reliability**: CDN integration and global distribution

## Options Considered

### Option 1: Cloudflare R2 (Recommended)

| Aspect | Details |
|--------|---------|
| Egress Fees | **None** - only per-request charges |
| HTTP Version | HTTP/2 |
| CORS Config | Via `wrangler` CLI or web UI |
| CDN | Built-in global CDN |

**Pros:**
- Zero bandwidth fees make costs predictable
- HTTP/2 improves tile loading performance
- Integrated with Cloudflare's global network
- S3-compatible API

**Cons:**
- Vendor lock-in to Cloudflare ecosystem
- Requires Cloudflare account

### Option 2: Amazon S3

| Aspect | Details |
|--------|---------|
| Egress Fees | Yes - per GB transferred |
| HTTP Version | HTTP/1.1 only |
| CORS Config | Via web console or `aws s3api put-bucket-cors` |
| CDN | Optional CloudFront integration |

**Pros:**
- Industry standard, well-documented
- Extensive tooling and SDK support
- Fine-grained IAM controls

**Cons:**
- Bandwidth fees can be significant at scale
- HTTP/1.1 limits concurrent requests
- CloudFront adds complexity and cost

### Option 3: Google Cloud Storage

| Aspect | Details |
|--------|---------|
| Egress Fees | Yes - per GB transferred |
| HTTP Version | HTTP/2 |
| CORS Config | Via `gcloud storage buckets update` with JSON file |
| CDN | Optional Cloud CDN integration |

**Pros:**
- HTTP/2 support for better performance
- Strong integration with GCP services
- Multi-regional storage options

**Cons:**
- Egress fees at scale
- CORS config requires CLI with JSON files

### Option 4: Tigris Data

| Aspect | Details |
|--------|---------|
| Egress Fees | **None** |
| HTTP Version | HTTP/2 |
| CORS Config | Via bucket settings UI |
| CDN | Built-in multi-location distribution |

**Pros:**
- No egress fees
- S3-compatible API
- Automatic multi-location distribution
- Simple CORS configuration

**Cons:**
- Newer provider, less ecosystem maturity
- Smaller community

### Option 5: Bunny.net

| Aspect | Details |
|--------|---------|
| Egress Fees | Low per-GB pricing |
| HTTP Version | HTTP/2 |
| CORS Config | Via Headers section in web UI |
| CDN | Pull Zone architecture |

**Pros:**
- EU-headquartered (GDPR considerations)
- Competitive pricing
- Good global coverage

**Cons:**
- Non-S3-compatible upload API (uses custom curl headers)
- Different operational model (Pull Zones)

### Option 6: Backblaze B2

| Aspect | Details |
|--------|---------|
| Egress Fees | Yes, but lower than S3 |
| HTTP Version | HTTP/1.1 only |
| CORS Config | Via b2 CLI or web UI |
| CDN | Optional Cloudflare integration (free egress) |

**Pros:**
- Very low storage costs
- Free egress when paired with Cloudflare CDN
- S3-compatible API

**Cons:**
- HTTP/1.1 only
- Requires CLI tool for advanced CORS config

### Option 7: Self-Hosted (Caddy/Nginx)

| Aspect | Details |
|--------|---------|
| Egress Fees | Depends on hosting |
| HTTP Version | HTTP/2 (Caddy), configurable (Nginx) |
| CORS Config | Manual header configuration |
| CDN | BYO CDN |

**Pros:**
- Full control over configuration
- Caddy has built-in HTTPS via Let's Encrypt
- No vendor dependencies

**Cons:**
- Operational overhead
- Must manage scaling and reliability
- Manual CORS header configuration

## Comparison Matrix

| Provider | Egress Fees | HTTP/2 | CORS Complexity | Best For |
|----------|-------------|--------|-----------------|----------|
| Cloudflare R2 | None | Yes | Low | Production at scale |
| Tigris Data | None | Yes | Low | Cost-sensitive production |
| Amazon S3 | High | No | Medium | AWS-native environments |
| Google Cloud | High | Yes | Medium | GCP-native environments |
| Bunny.net | Low | Yes | Low | EU-focused deployments |
| Backblaze B2 | Medium | No | Medium | Budget with Cloudflare CDN |
| Self-Hosted | Variable | Yes | High | Maximum control |

## Decision

**Recommended: Cloudflare R2** for production deployments.

**Rationale:**
1. Zero egress fees eliminate the largest variable cost for tile serving
2. HTTP/2 support enables efficient multiplexed tile loading
3. Built-in global CDN reduces latency worldwide
4. S3-compatible API allows easy migration if needed

**Alternative: Tigris Data** for projects wanting zero egress fees with multi-location distribution but without Cloudflare ecosystem dependency.

**Development/Testing: Self-Hosted with Caddy** for local development and testing environments.

## CORS Configuration Requirements

All providers require these CORS settings for PMTiles:

```
Allowed Origins: * (or specific domains)
Allowed Methods: GET, HEAD
Allowed Headers: range, if-match
Exposed Headers: range, accept-ranges, etag
```

## Cost Estimation

For a site serving 1M tile requests/month with average 50KB tiles:

| Provider | Storage (10GB) | Requests (1M) | Egress (50TB) | Total |
|----------|---------------|---------------|---------------|-------|
| Cloudflare R2 | ~$0.15 | ~$0.36 | $0 | ~$0.51 |
| Amazon S3 | ~$0.23 | ~$0.40 | ~$4,500 | ~$4,500 |
| Tigris Data | ~$0.20 | ~$0.40 | $0 | ~$0.60 |

*Note: Prices are approximate and may vary by region and tier.*

## Consequences

### Positive
- Predictable costs with zero-egress providers
- Better user experience with HTTP/2 multiplexing
- Simplified architecture with built-in CDN

### Negative
- Dependency on specific cloud provider
- Migration effort if changing providers
- Learning curve for provider-specific tooling

### Neutral
- CORS configuration required regardless of provider
- All providers support HTTP Range Requests

## References

- [PMTiles Cloud Storage Documentation](https://docs.protomaps.com/pmtiles/cloud-storage)
- [Cloudflare R2 Pricing](https://developers.cloudflare.com/r2/pricing/)
- [PMTiles Specification](https://github.com/protomaps/PMTiles)
