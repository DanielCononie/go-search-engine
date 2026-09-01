# Search contract

The public search contract is version `v1`.

## Request

```text
GET /search?q=iron+man&mode=lexical&limit=10&offset=0&site=en.wikipedia.org&language=en
```

- `q` is required.
- `mode` defaults to `lexical`. Supported values are `lexical` and `semantic`.
- `limit` defaults to 10 and must be between 1 and 100.
- `offset` defaults to 0 and must be non-negative.
- `site` optionally filters results to an exact site value.
- `language` optionally filters results to an exact language value.
- Double quotes express an exact phrase, for example `"infinity stones"`.
- Query syntax is treated as text rather than raw Redis Search syntax.
- Exact-phrase syntax applies to lexical mode. Semantic mode embeds the query
  as natural language and does not interpret quotes as a Redis phrase query.

## Response

```json
{
  "version": "v1",
  "query": "iron man",
  "mode": "lexical",
  "used_mode": "lexical",
  "took_ms": 1,
  "next_offset": null,
  "suggestions": [
    {
      "term": "spidr",
      "candidates": ["spider"]
    }
  ],
  "results": [
    {
      "page_id": "sha256:example",
      "url": "https://en.wikipedia.org/wiki/Iron_Man",
      "title": "Iron Man",
      "score": 12,
      "matched_sections": [
        {
          "section_id": "sha256:section-example",
          "heading": "Fictional character biography",
          "snippet": "Iron Man is a superhero appearing in American comic books...",
          "score": 12
        }
      ]
    }
  ]
}
```

Results are grouped by page and include up to three matching sections. The best section supplies the page's base score; additional matching sections add a capped bonus. `next_offset` is set when another grouped page is available.

When lexical search returns no results, Redis Search may provide spelling
suggestions. Suggestions are advisory: the API never rewrites or reruns the
user's query automatically, and the field is omitted when Redis has no
candidate.

Semantic mode embeds the request query, finds the closest current section
vectors by cosine distance, groups them by page, and returns section evidence
through the same response shape. Semantic page and section scores are cosine
similarities. Semantic mode does not return lexical spelling suggestions.

The response always reports the actual executed mode in `used_mode`. Lexical
search remains the default. If semantic mode is not configured, active, and
synchronized, an explicit `mode=semantic` request returns HTTP 503; the API
never silently substitutes lexical results. Hybrid retrieval is not part of
this contract.

## Relevance baseline

`testdata/search_relevance.json` contains the initial judged query set. Run
`go run ./cmd/relevance` to measure the retained in-memory implementation, or
`go run ./cmd/relevance -backend redis` to evaluate the live lexical backend.
After configuring and backfilling embeddings, use
`go run ./cmd/relevance -backend semantic -fixture
testdata/semantic_relevance.json` to evaluate semantic retrieval against the
natural-language semantic judgment fixture.
Reports include top results, Recall@10, MRR, nDCG@10, p50/p95 latency,
zero-result rate, and backend metadata. The historical Phase 0 measurement is
checked in at `docs/relevance-baseline.json`; future field-weight and ranking
changes should compare Redis output with it.

`go run ./cmd/search-diagnostics` reports the alias, active physical index,
schema version, indexed section count, indexing errors and memory, a
content-derived corpus version, and whether page records and the search index
are currently in sync. With embedding configuration present, it also reports
the active semantic physical index, embedding profile, source/record coverage,
stale and failed semantic records, and `semantic_available`.

## Semantic index lifecycle

`go run ./cmd/embedder` derives an embedding profile from the configured
model, model version, and dimensions. Version-specific semantic records copy
the durable section metadata, source content hash, embedding profile, status,
and vector under `search:semantic-section:<index-version>:*`.

The embedder skips ready records whose source content hash and profile are
current. Writes are guarded by the source section content hash so a concurrent
re-index cannot attach an obsolete vector. A complete run waits for Redis
Search to index every current record before moving
`search_semantic_sections_current` to the configured physical index. Failed
runs leave the previous active semantic index unchanged.
