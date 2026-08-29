#!/usr/bin/env bash
# Builds the pinned protobuf/SQL/migration tools from tools/go.mod into .bin/.
#
# The tools live in their own module so their (large) dependency graphs never
# constrain the runtime module. They are invoked from the repository root, so
# they are built to binaries rather than run via `go -C tools tool`, which would
# also change the working directory the tool sees.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin_dir="$repo_root/.bin"
mkdir -p "$bin_dir"

tools=(
  "protoc-gen-go=google.golang.org/protobuf/cmd/protoc-gen-go"
  "protoc-gen-go-grpc=google.golang.org/grpc/cmd/protoc-gen-go-grpc"
  "protoc-gen-grpc-gateway=github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
  "protoc-gen-openapi=github.com/google/gnostic/cmd/protoc-gen-openapi"
  "api-linter=github.com/googleapis/api-linter/cmd/api-linter"
  "sqlc=github.com/sqlc-dev/sqlc/cmd/sqlc"
  "goose=github.com/pressly/goose/v3/cmd/goose"
)

for entry in "${tools[@]}"; do
  name="${entry%%=*}"
  pkg="${entry#*=}"
  if [[ -x "$bin_dir/$name" && "${FORCE:-}" != "1" ]]; then
    echo "  = $name (cached)"
    continue
  fi
  echo "  + $name"
  (cd "$repo_root/tools" && go build -o "$bin_dir/$name" "$pkg")
done

echo "tools ready in .bin/"
