# BVB Dividends

A read-only API over the dividends announced on the Bucharest Stock Exchange
(Bursa de Valori București), with a daily importer that reads them from BVB's
financials web service, and a small web client.

## What it is

- **`apps/api`** — an HTTP API defined in protobuf and following the
  [Google API Improvement Proposals](https://google.aip.dev). Compliance is
  enforced by `api-linter` in CI, not asserted.
- **`apps/importer`** — a job that refreshes companies and dividends from BVB.
  It runs once and exits, or stays resident on a daily schedule.
- **`apps/web`** — a React client whose types are generated from the API's own
  OpenAPI document.

## Getting started

Requires Go 1.25+, [Bun](https://bun.sh) 1.3+, and Docker or a local
PostgreSQL 16.

```sh
bun install
cp .env.example .env          # nothing secret; the defaults are for local use
docker compose up -d          # PostgreSQL on :5555

bunx nx run postgres:migrate-up
bunx nx run api:serve         # http://localhost:8080
bunx nx run web:serve         # http://localhost:4200
```

`http://localhost:8080/docs` renders the API's generated specification.

To fill the database, point the importer at BVB and run it once:

```sh
bunx nx run importer:serve -- --days=30
```

That needs network access to `ws.bvb.ro`; see [Reaching BVB](#reaching-bvb).

## The API

Resources are hierarchical, per [AIP-122](https://google.aip.dev/122): a
dividend belongs to the company that declared it.

```
GET /v1/companies                                  list companies
GET /v1/companies/{company}                        get one company
GET /v1/companies/{company}/dividends              list a company's dividends
GET /v1/companies/-/dividends                      list across all companies (AIP-159)
GET /v1/companies/{company}/dividends/{dividend}   get one dividend

GET /openapi.yaml   the generated specification
GET /docs           a rendering of it
GET /healthz        the process is running
GET /readyz         the database is reachable
```

Listings take `pageSize`, `pageToken`, `filter` and `orderBy`. Page tokens are
opaque and carry a checksum of the rest of the request, so reusing one under a
different filter fails rather than silently returning a page of something else.

`filter` is the full [AIP-160](https://google.aip.dev/160) expression language:

```sh
# Dividends you can still earn by buying the share today.
curl -sG localhost:8080/v1/companies/-/dividends \
  --data-urlencode 'filter=schedule.ex_dividend_date > "2026-08-29"'

# Announced, but BVB has not published an ex-dividend date yet.
curl -sG localhost:8080/v1/companies/-/dividends \
  --data-urlencode 'filter=schedule.ex_dividend_date = null'

# Recent cash dividends from one company, oldest first.
curl -sG localhost:8080/v1/companies/SNP/dividends \
  --data-urlencode 'filter=year >= 2024 AND dividend_type = "cash"' \
  --data-urlencode 'orderBy=schedule.ex_dividend_date'
```

The previous service's `/active-dividends` has no counterpart here. Under the AIPs
it is not a method but a filter over the collection, which is why this API has
one fewer endpoint and more reach: "announced but not yet scheduled" is a
question the old API could not express, because its date-range queries excluded
every dividend with no ex-dividend date.

## How it fits together

The protos are the source of truth. Everything else is generated from them:

```
proto/  ──► api-linter + buf          the AIPs, enforced in CI
        ──► protoc-gen-go + gateway   the Go server and its REST mapping
        ──► protoc-gen-openapi        the OpenAPI document, embedded and served
        ──► openapi-typescript        the web client's types
```

grpc-gateway is registered against the service implementation in-process, so
the binary serves REST without opening a gRPC port or dialling itself.

```
apps/         api, importer, web
libs/
  domain/     entities, value objects, the filter AST, the repository ports.
              No I/O, no struct tags, no dependencies on anything below.
  app/        use cases: the import run
  apiserver/  the service implementation, REST gateway, AIP-193 errors,
              and aipquery/, which compiles filters and orderings
  postgres/   repositories, migrations, sqlc output
  bvbsoap/    the BVB SOAP client; the only package that knows BVB speaks SOAP
proto/        the API definition
tools/        pinned build tools, in their own module so their dependency
              graphs never constrain the runtime module
```

Adapters depend on the domain. The domain depends on nothing.

## Working on it

Nx runs everything, and knows what depends on what:

```sh
bunx nx run-many -t build            # everything, in dependency order
bunx nx run-many -t test
bunx nx run-many -t lint
bunx nx graph                        # what depends on what

bunx nx run proto:lint               # buf + api-linter (the AIPs)
bunx nx run proto:breaking           # no breaking changes against main
bunx nx run proto:generate           # Go, the gateway, and the OpenAPI document

bunx nx run postgres:migrate-up      # also -down, -status, -create
bunx nx run postgres:sqlc-generate
```

Generated code is committed so editors work without a build step. CI
regenerates it and fails if the result differs, so it cannot go stale.

### Tests

Unit tests need nothing. The `libs/postgres` tests need a database, named by
`TEST_DATABASE_URL`, and skip without one:

```sh
createdb bvb_dividends_test
TEST_DATABASE_URL=postgres://...@localhost:5555/bvb_dividends_test go test ./...
```

They truncate between cases, so they refuse to start unless the database name
contains `test`.

## Reaching BVB

The importer reads from `https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx`,
which is a SOAP service.

BVB publishes a WSDL, and the previous service generated its request and
response classes from it at build time, so the element names were never
written down in that repository. `libs/bvbsoap/types.go` reconstructs them
from how that generated code was used, which pins every field. The one genuine
judgement is the `identityType` enum value, and it is marked as such in that
file — it is the first thing to try changing if BVB rejects a request.

Those types are covered by recorded responses in `libs/bvbsoap/testdata/`, so
the mapping is tested, but the tests cannot confirm the names BVB really uses.
**Confirm them against the live service before trusting an import**, with:

```sh
bunx nx run importer:serve -- --days=30 --dry-run
```

which fetches and maps everything and writes nothing.

## What changed from the previous service

The rewrite fixes defects rather than carrying them over:

| Before | Here |
|---|---|
| `/dividend` returned `{}` — the DTO had no accessors, so nothing could populate or serialise it | Responses are generated from the protos |
| Dividend IDs were regenerated daily by deleting and re-inserting each company's dividends, so saved links broke within a day | IDs are derived from a natural key, so a re-import updates the same row and a resource name stays valid |
| Pagination metadata was computed and then discarded | `nextPageToken` and `totalSize`, per AIP-158 |
| Amounts were an arbitrary-precision type in the entity and a floating-point type in the DTO | `numeric(20,4)` end to end, served as `google.type.Money` |
| Dividends with no ex-dividend date were invisible to every query | Reachable, and distinguishable from scheduled ones |
| `.distinct()` on companies with no `equals` deduplicated nothing | Deduplicated by symbol |
| A renamed company kept its old name forever | Upserted |
| SOAP failures were logged and returned an empty list, which read downstream as "no dividends" | Faults surface with BVB's message; transport failures retry |
| The whole import ran in one transaction, sequentially | One transaction per company, fetched concurrently, with a report of what failed |
| The database password was committed | Configuration comes from the environment |
| No indexes on `company_symbol` or `ex_dividend_date`; one test | Indexed; unit, integration, fuzz and HTTP tests |
