# Search contract

The public search contract is version `v1`.

## Request

```text
GET /search?q=iron+man&mode=lexical&limit=10&offset=0
```

- `q` is required.
- `mode` defaults to `lexical`. Phase 0 supports only `lexical`.
- `limit` defaults to 10 and must be between 1 and 100.
- `offset` defaults to 0 and must be non-negative.

## Response

```json
{
  "version": "v1",
  "query": "iron man",
  "mode": "lexical",
  "used_mode": "lexical",
  "took_ms": 1,
  "next_offset": null,
  "results": [
    {
      "url": "https://en.wikipedia.org/wiki/Iron_Man",
      "title": "Iron Man",
      "score": 12
    }
  ]
}
```

`matched_sections` is omitted until section-aware indexing is introduced. Future semantic or hybrid modes must report the actual executed mode in `used_mode`; fallback must never be silent.

## Relevance baseline

`testdata/search_relevance.json` contains the initial judged query set. Run `go run ./cmd/relevance` to measure the current in-memory implementation and record its top results, Recall@10, MRR, p50/p95 latency, zero-result rate, and indexing duration. The Phase 0 measurement is checked in at `docs/relevance-baseline.json`; future ranking changes should compare their report with it. Redis memory per page and section becomes measurable after external Redis provisioning and ingestion are available.
