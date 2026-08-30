# BVB Dividends

A read-only API over the dividends announced on the Bucharest Stock Exchange
(Bursa de Valori București), a web client over that API, and a daily import that
keeps both fed from BVB's financials web service — as one binary.

This is a Go rewrite of [`CodrinSocol/bvb-dividends`](https://github.com/CodrinSocol/bvb-dividends),
which is a Spring Boot service. That service is still running and is untouched
by this repository.

## Getting started

Requires Go 1.26+, [Bun](https://bun.sh) 1.3+, and Docker or a local
PostgreSQL 16.

```sh
bun install
cp .env.example .env          # nothing secret; the defaults are for local use
docker compose up -d          # PostgreSQL on :5432

bunx nx run web:build         # the binary embeds the built application
bunx nx run bvb-dividends:serve
```

That serves the application and the API on <http://localhost:8080>, applies each
slice's schema on startup, and imports from BVB daily at noon Bucharest time.
`http://localhost:8080/api/docs` renders the API's generated specification.

To fill the database now rather than waiting for the schedule:

```sh
go run ./cmd/bvb-dividends import
```

The first import takes the whole market - a few hundred companies and a few
thousand dividends, in well under a minute. It needs network access to
`ws.bvb.ro`; see [Reaching BVB](#reaching-bvb).

While working on the frontend, `bunx nx run web:serve` runs vite on :4200 with
the API proxied, so the client uses the same origin-relative URLs it will use
once it is embedded.

## The binary

```
bvb-dividends                serve the API and the web application, and import
                             from BVB on a daily schedule
bvb-dividends import [flags] import once and exit
                             -days N  window of announcements to ask for
                             -all     every company BVB knows of
                             -dry-run fetch and map everything, write nothing
bvb-dividends migrate        apply the database migrations and exit
```

Everything is configured through the environment; `.env.example` lists every
variable with the default it takes.

## The API

Resources are hierarchical, per [AIP-122](https://google.aip.dev/122): a
dividend belongs to the company that declared it.

The service speaks two protocols over the same handlers, transcoded by
[vanguard](https://github.com/connectrpc/vanguard-go): the Connect protocol,
which is what the generated web client uses, and the REST mapping the
`google.api.http` annotations declare, which is what the OpenAPI document
describes.

```
GET /api/v1/companies                                  list companies
GET /api/v1/companies/{company}                        get one company
GET /api/v1/companies/{company}/dividends              list a company's dividends
GET /api/v1/companies/-/dividends                      list across all companies (AIP-159)
GET /api/v1/companies/{company}/dividends/{dividend}   get one dividend

GET /api/docs                the generated specification, rendered
GET /api/docs/openapi.yaml   the specification itself
GET /healthz/live            the process is running
GET /healthz/ready           the database is reachable
```

Listings take `pageSize`, `pageToken`, `filter` and `orderBy`. Pagination is
keyset rather than offset, so a page is stable while the import writes
underneath it and a deep page costs no more than a shallow one; page tokens
carry a checksum of the rest of the request, so reusing one under a different
filter fails rather than silently returning a page of something else.

`filter` is the full [AIP-160](https://google.aip.dev/160) expression language,
over the fields the resource's proto message declares:

```sh
# Dividends you can still earn by buying the share today.
curl -sG localhost:8080/api/v1/companies/-/dividends \
  --data-urlencode 'filter=state = "STATE_ANNOUNCED"'

# Announced, but BVB has not published an ex-dividend date yet.
curl -sG localhost:8080/api/v1/companies/-/dividends \
  --data-urlencode 'filter=schedule.ex_dividend_date = null'

# Recent cash dividends from one company, oldest first.
curl -sG localhost:8080/api/v1/companies/SNP/dividends \
  --data-urlencode 'filter=year >= 2024 AND dividend_type = "cash"' \
  --data-urlencode 'orderBy=schedule.ex_dividend_date'
```

The Java service's `/active-dividends` has no counterpart here. Under the AIPs
it is not a method but a filter over the collection, which is why this API has
one fewer endpoint and more reach: "announced but not yet scheduled" is a
question the old API could not express, because its date-range queries excluded
every dividend with no ex-dividend date.

## How it fits together

The service is organised as vertical slices. A slice owns its entity, the
commands and queries against it, the schema it is stored in, the import that
fills it, and the ConnectRPC interface that exposes it — so a change to a
resource is a change inside one directory.

```
cmd/bvb-dividends/   the composition root: one file per slice, saying what it
                     provides, plus the BVB client and the scheduled import
internal/
  companies/         \  entities.go, commands.go, queries.go, service.go
  dividends/          > infra/bvb/            maps what BVB reports
                     /  infra/repo/postgres/  the slice's schema and queries
                        interface/connectrpc/ the service it implements
  common/            what the slices share: the value objects, the error type,
                     and the infrastructure - fx wiring, the database, the
                     ConnectRPC server, the AIP interceptors
api/proto/           the API definition, and the OpenAPI document generated
                     from it
libs/go/bvb-client/  the BVB SOAP client, which knows nothing about this domain
libs/go/gen/         the generated Go server stubs and resource names
libs/ts/gen/         the generated TypeScript client
web/                 the React application, embedded into the binary
```

A slice may not import another slice. The two places they meet are both
explicit: the import in `cmd/` passes the symbols the companies import saw to
the dividends import, and the dividends interface declares the one question it
needs answered about a company, which `cmd/` binds the companies service to.

The protos are the source of truth. Everything else is generated from them:

```
api/proto/ ──► buf lint + api-linter      the AIPs, enforced in CI
           ──► protoc-gen-go + connect    the Go handlers
           ──► protoc-gen-go-aip          resource names, from the patterns
           ──► protobuf-es                the web client
           ──► connect-openapi            the OpenAPI document, embedded
```

That extends to what a caller may filter on: the filterable fields are derived
from the resource's proto message, so a field that is not on the resource cannot
be named in a filter, and one added to it becomes filterable without a second
edit. Each slice exposes those fields as a view, which is also where the two
values the API has and the table does not come from: a resource name, and the
dividend's lifecycle state.

## Working on it

Nx runs everything, and knows what depends on what:

```sh
bunx nx run-many -t build            # everything, in dependency order
bunx nx run bvb-dividends:test
bunx nx run bvb-dividends:lint:go
bunx nx graph                        # what depends on what

bunx nx run proto:lint:proto         # buf + api-linter (the AIPs)
bunx nx run proto:breaking           # no breaking changes against main
bunx nx run proto:build:proto        # the Go, TypeScript and OpenAPI output
bunx nx run db:build:sqlc            # the generated queries

bunx nx run web:build                # the application the binary embeds
bunx nx run web:serve                # vite on :4200, with the API proxied
```

The build tools — buf, sqlc, golangci-lint, api-linter — are pinned in
`tools/go.mod` and run through `go tool`, so CI and a developer's machine run
the same versions. `go.work` is what makes them resolve from the repository
root; the application module builds without it, which is what the container
image does.

Generated code is committed so editors work without a build step. CI
regenerates it and fails if the result differs, so it cannot go stale.

### Tests

Unit tests need nothing. The repository tests need a database, named by
`TEST_DATABASE_URL`, and skip without one:

```sh
createdb bvb_dividends_test
TEST_DATABASE_URL=postgres://...@localhost:5432/bvb_dividends_test go test ./...
```

They truncate between cases, so they refuse to start unless the database name
contains `test`, and they take an advisory lock while they hold it, because
`go test ./...` runs packages in parallel.

## Reaching BVB

The import reads from `https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx`, which
is a SOAP service, through three of its operations:

```
GetAvailableBalances   which issuers filed a balance for a year - the closest
                       the service comes to listing the companies on BVB
GetLastDividends       the companies that announced a dividend recently
GetDividends           every dividend one company has declared
```

There is no operation that enumerates companies, which is why the first import
asks for the issuers that filed in each of the last few years and unions the
answers: the current year returns nothing until the filing season reaches it,
and a company that has since delisted appears only in the years it filed. A
later run can ask for the whole market again with `import --all`.

`libs/go/bvb-client/types.go` is derived from the WSDL the service publishes at
`?WSDL`, and two of its details are worth knowing, because getting either wrong
produces no error at all - the service accepts the request, binds none of the
parameters, and answers with an empty result:

- the XML namespace is `http://www.bvb.ro`, with no trailing slash, while the
  SOAPAction header does have one;
- request parameters are PascalCase (`NoDays`, `IdentityType`, `Identity`), and
  one response element is spelled `GMS_Date`.

The recorded responses in `libs/go/bvb-client/testdata/` come from the live
service, so the mapping is tested against what it really sends. To check a
change against it without writing anything:

```sh
go run ./cmd/bvb-dividends import --dry-run
```

## What changed from the Java service

Beyond the language, the rewrite fixes defects rather than carrying them over:

| Java behaviour | Here |
|---|---|
| `/dividend` returned `{}` — the DTO had no accessors, so nothing could populate or serialise it | Responses are generated from the protos |
| Dividend IDs were regenerated daily by deleting and re-inserting each company's dividends, so saved links broke within a day | IDs are derived from a natural key, so a re-import updates the same row and a resource name stays valid |
| Pagination metadata was computed and then discarded | `nextPageToken` and `totalSize`, per AIP-158, over a keyset cursor |
| Amounts were `BigDecimal` in the entity and `Double` in the DTO | `numeric(20,4)` end to end, served as `google.type.Money` |
| Dividends with no ex-dividend date were invisible to every query | Reachable, and distinguishable from scheduled ones |
| `.distinct()` on companies with no `equals` deduplicated nothing | Deduplicated by symbol |
| A renamed company kept its old name forever | Upserted, and only when something actually changed |
| SOAP failures were logged and returned an empty list, which read downstream as "no dividends" | Faults surface with BVB's message; transport failures retry |
| The whole import ran in one transaction, sequentially | One transaction per company, fetched concurrently, with a report of what failed |
| The database password was committed | Configuration comes from the environment |
| No indexes on `company_symbol` or `ex_dividend_date`; one test | Indexed; unit, integration and HTTP tests, against a real database |

## Contributors

The Java service this replaces was written by
[Codrin Socol](https://github.com/CodrinSocol) and
[Mihnea-Andrei Bloțiu](https://github.com/mihneablotiu).
