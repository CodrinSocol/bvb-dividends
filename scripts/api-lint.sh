#!/usr/bin/env bash
# Lints the proto API surface against the Google AIPs.
#
# api-linter resolves imports from a FileDescriptorSet rather than from proto
# paths, so buf builds the descriptor first: buf already knows how to resolve
# the imports, and this keeps one source of truth for how the protos compile.
#
# Both tools are pinned in tools/go.mod and run through `go tool`, which the
# workspace in go.work resolves from the repository root.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

descriptor="$(mktemp -t api-linter-descriptor.XXXXXX.binpb)"
trap 'rm -f "$descriptor"' EXIT

go tool buf build --as-file-descriptor-set -o "$descriptor" 2>/dev/null

# Lint only this project's protos; everything imported is a dependency.
mapfile -t protos < <(cd api/proto/bvb-dividends && find . -name '*.proto' | sed 's|^\./||' | sort)

go tool api-linter \
  --config api-linter.yaml \
  --descriptor-set-in "$descriptor" \
  --set-exit-status \
  --output-format="${API_LINTER_FORMAT:-summary}" \
  "${protos[@]}"
