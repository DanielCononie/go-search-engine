# Go Search Engine

A Go/Fiber search-engine project that indexes web pages as sections in Redis and serves lexical full-text search results.

## Current architecture

- `cmd/indexer` runs the standalone crawl and indexing pipeline.
- `internal/crawler` fetches configured URLs with bounded concurrency, retries, response limits, redirect limits, and per-host pacing.
- `internal/parser` converts article/main/body content into deterministic heading-bounded sections.
- `internal/ingest` atomically replaces successful page versions and records crawl failures separately.
- `internal/search` defines the versioned lexical search contract and validates user queries.
- `internal/storage/redis` stores page and section JSON documents and executes weighted Redis Search queries.
- `internal/handlers` exposes search and health endpoints through Fiber.
- `cmd/search-diagnostics` reports the active physical index, schema version, indexing health, corpus version, and section-count consistency.

Redis is the durable source of truth. Crawling and indexing are intentionally separate from API startup.

## Redis requirements

Provision Redis outside this repository with Redis Search and JSON support. The indexer and API create the `search_sections_v1` index and the `search_sections_current` alias if they do not already exist.

Configuration:

| Variable | Default |
|---|---|
| `REDIS_ADDR` | `localhost:6379` |
| `REDIS_USERNAME` | empty |
| `REDIS_PASSWORD` | empty |
| `REDIS_DB` | `0` |
| `REDIS_TLS_ENABLED` | `false` |
| `REDIS_DIAL_TIMEOUT` | `5s` |
| `REDIS_READ_TIMEOUT` | `3s` |
| `REDIS_WRITE_TIMEOUT` | `3s` |
| `REDIS_POOL_SIZE` | `10` |

## Index

```bash
go run ./cmd/indexer
```

The indexer crawls `internal/config/sites.go`, emits a JSON report, and exits non-zero when any URL fails. Successful page replacements remove stale sections; failed crawls preserve the last successful page and store a separate failure record.

## Run the API

```bash
go run ./cmd/api
```

The API reads previously indexed sections from Redis and listens on port 3000. It does not crawl on startup.

## Endpoints

```text
GET /search?q=iron+man&mode=lexical&limit=10&offset=0
GET /search?q=%22infinity+stones%22&site=en.wikipedia.org&language=en
GET /health/live
GET /health/ready
```

The search contract is documented in `docs/search-contract.md`. The initial judged relevance queries are stored in `testdata/search_relevance.json`.

## Tests

```bash
go test ./...
```

Generate the retained Phase 0 in-memory relevance baseline:

```bash
go run ./cmd/relevance
```

The captured Phase 0 report is stored in `docs/relevance-baseline.json`.

Evaluate the current Redis lexical backend against the same judgments:

```bash
go run ./cmd/relevance -backend redis
```

The report includes Recall@10, MRR, nDCG@10, zero-result rate, p50/p95
latency, the backend name, and the active index alias. Compare this output
with the retained Phase 0 baseline before changing field weights or ranking.

Inspect the live index and corpus before or after indexing:

```bash
go run ./cmd/search-diagnostics
```

`in_sync` is true when indexing is idle, Redis reports no indexing failures,
and the indexed section count matches the sections referenced by all durable
page records. The corpus version is derived from sorted page IDs and content
hashes, so it changes only when the searchable page content changes.

Redis integration tests are skipped by default. To run them against a disposable Redis instance that includes Search and JSON:

```bash
REDIS_INTEGRATION_ADDR=localhost:6379 go test ./internal/storage/redis -run Integration
```
