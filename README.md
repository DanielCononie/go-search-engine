# Go Search Engine

A Go/Fiber search-engine project that indexes web pages as sections in Redis
and serves lexical full-text search with optional semantic section search.

## Current architecture

- `cmd/indexer` runs the standalone crawl and indexing pipeline.
- `internal/crawler` fetches configured URLs with bounded concurrency, retries, response limits, redirect limits, and per-host pacing.
- `internal/parser` converts article/main/body content into deterministic heading-bounded sections.
- `internal/ingest` atomically replaces successful page versions and records crawl failures separately.
- `internal/search` defines the versioned lexical and semantic search contract and validates user queries.
- `internal/embedding` calls an OpenAI-compatible embedding API and creates normalized section vectors.
- `internal/storage/redis` stores page, lexical-section, and versioned semantic-section JSON documents and executes Redis Search queries.
- `internal/handlers` exposes search and health endpoints through Fiber.
- `cmd/embedder` idempotently backfills section embeddings and activates a complete semantic index version.
- `cmd/search-diagnostics` reports lexical and semantic index health, corpus versions, and section-count consistency.

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

Commands load a local `.env` file when present. Existing process environment
variables take precedence. Copy `.env.example` to `.env`; the local file is
ignored by Git.

## Index

```bash
go run ./cmd/indexer
```

The indexer crawls `internal/config/sites.go`, emits a JSON report, and exits non-zero when any URL fails. Successful page replacements remove stale sections; failed crawls preserve the last successful page and store a separate failure record.

## Embed sections

Configure an OpenAI-compatible embeddings endpoint in `.env`:

| Variable | Default |
|---|---|
| `EMBEDDING_BASE_URL` | `https://api.openai.com/v1` |
| `EMBEDDING_API_KEY` | empty |
| `EMBEDDING_MODEL` | empty; semantic search disabled |
| `EMBEDDING_VERSION` | empty |
| `EMBEDDING_DIMENSIONS` | empty |
| `EMBEDDING_INDEX_VERSION` | `v1` |
| `EMBEDDING_TIMEOUT` | `30s` |
| `EMBEDDING_MAX_RETRIES` | `2` |
| `EMBEDDING_MAX_INPUT_RUNES` | `12000` |
| `EMBEDDING_INPUT_OVERLAP_RUNES` | `500` |
| `EMBEDDING_SEND_DIMENSIONS` | `false` |

`EMBEDDING_MODEL` and `EMBEDDING_DIMENSIONS` must be set to enable semantic
search. `EMBEDDING_VERSION` defaults to the model name and should be set
explicitly when the provider can change a model implementation without
changing its name. `EMBEDDING_API_KEY` may be empty for a compatible endpoint
that does not require authentication.

After lexical indexing, backfill semantic records:

```bash
go run ./cmd/embedder
```

The command skips current records, re-embeds changed sections, removes
orphaned semantic records, and exits non-zero on partial provider failures.
Each `EMBEDDING_INDEX_VERSION` uses a separate Redis key prefix and physical
vector index. The shared semantic alias moves to the new index only after all
current source sections are embedded and indexed.

## Run the API

```bash
go run ./cmd/api
```

The API reads previously indexed sections from Redis and listens on port
3000. It does not crawl or embed on startup. When embedding configuration is
present, semantic mode is enabled only if its configured vector index is
active and synchronized; run the embedder and restart the API after changing
the corpus or embedding profile.

## Endpoints

```text
GET /search?q=iron+man&mode=lexical&limit=10&offset=0
GET /search?q=%22infinity+stones%22&site=en.wikipedia.org&language=en
GET /search?q=hero+who+uses+powered+armor&mode=semantic
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

Evaluate semantic retrieval after embedding the corpus:

```bash
go run ./cmd/relevance \
  -backend semantic \
  -fixture testdata/semantic_relevance.json
```

The report includes Recall@10, MRR, nDCG@10, zero-result rate, p50/p95
latency, the backend name, and the active index alias. Compare this output
with the retained Phase 0 baseline before changing field weights or ranking.

Inspect the live index and corpus before or after indexing:

```bash
go run ./cmd/search-diagnostics
```

Lexical `in_sync` is true when indexing is idle, Redis reports no indexing failures,
and the indexed section count matches the sections referenced by all durable
page records. The corpus version is derived from sorted page IDs and content
hashes, so it changes only when the searchable page content changes.
When embeddings are configured, `semantic_available` additionally requires
the configured physical vector index to be active and every durable source
section to have a current ready semantic record.

Redis integration tests are skipped by default. To run them against a disposable Redis instance that includes Search and JSON:

```bash
REDIS_INTEGRATION_ADDR=localhost:6379 go test ./internal/storage/redis -run Integration
```
