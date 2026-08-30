# Builds the single bvb-dividends binary, with the web application in it.
#
# The frontend is built first and embedded, so what ships is one artifact
# serving the API and the application from one origin, rather than a pair that
# have to be deployed and versioned together.

FROM oven/bun:1.3.11-alpine AS web

WORKDIR /src

# Manifests first, so a source-only change does not re-resolve the dependencies.
COPY package.json bun.lock ./
COPY web/package.json ./web/
COPY libs/ts/package.json ./libs/ts/
RUN bun install --frozen-lockfile

COPY tsconfig.base.json ./
COPY libs/ts ./libs/ts
COPY web ./web

RUN cd web && bunx vite build

FROM golang:1.26-alpine AS build

WORKDIR /src

# Dependencies first, for the same reason.
COPY go.mod go.sum ./
RUN go mod download

COPY api ./api
COPY cmd ./cmd
COPY internal ./internal
COPY libs/go ./libs/go
COPY web/embed.go ./web/
COPY --from=web /src/web/dist ./web/dist

# CGO is off so the result is a static binary the distroless image can run.
# The build metadata is stripped for reproducibility and size.
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/bvb-dividends ./cmd/bvb-dividends

# Distroless carries no shell and no package manager, and runs as a non-root
# user by default: the surface of a public, unauthenticated service should be
# the binary and nothing else.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/bvb-dividends /bvb-dividends
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/bvb-dividends"]
