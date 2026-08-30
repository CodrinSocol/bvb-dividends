# Builds either Go binary; pass --build-arg APP=api or --build-arg APP=importer.
#
# Both binaries share a dependency layer, so building the second one after the
# first reuses the module download and most of the compile cache.

FROM golang:1.25-alpine AS build
ARG APP=api

WORKDIR /src

# Dependencies first, so a source-only change does not re-download them.
COPY go.mod go.sum ./
RUN go mod download

COPY libs ./libs
COPY apps ./apps

# CGO is off so the result is a static binary the distroless image can run.
# The build metadata is stripped for reproducibility and size.
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/app "./apps/${APP}"

# Distroless carries no shell and no package manager, and runs as a non-root
# user by default: the surface of a public, unauthenticated service should be
# the binary and nothing else.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
