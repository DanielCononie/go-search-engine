# Go Search Engine

A Go/Fiber search-engine project that crawls configured pages, parses their content, and serves ranked search results.

## Current architecture

- `internal/crawler` fetches configured URLs concurrently.
- `internal/parser` extracts page titles and searchable content.
- `internal/index` builds the current in-memory inverted index.
- `internal/search` defines the versioned search contract and backend boundary.
- `internal/storage/redis` provides Redis JSON repositories, health checks, and versioned Redis Search index management.
- `internal/handlers` exposes search and health endpoints through Fiber.

The in-memory backend remains active during the Redis migration. Redis is now required at API startup and is the persistence/search foundation for the next ingestion and lexical-search phases.

## Redis requirements

Provision Redis outside this repository with Redis Search and JSON support. The API creates the `search_sections_v1` index and the `search_sections_current` alias if they do not already exist.

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

## Run

```bash
go run ./cmd/api
```

The API listens on port 3000.

## Endpoints

```text
GET /search?q=iron+man&mode=lexical&limit=10&offset=0
GET /health/live
GET /health/ready
```

The search contract is documented in `docs/search-contract.md`. The initial judged relevance queries are stored in `testdata/search_relevance.json`.

## Tests

```bash
go test ./...
```

Generate a relevance report for the current in-memory baseline:

```bash
go run ./cmd/relevance
```

The captured Phase 0 report is stored in `docs/relevance-baseline.json`.

Redis integration tests are skipped by default. To run them against a disposable Redis instance that includes Search and JSON:

```bash
REDIS_INTEGRATION_ADDR=localhost:6379 go test ./internal/storage/redis -run Integration
```
