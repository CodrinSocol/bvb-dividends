#!/usr/bin/env bash
# Lints the proto API surface against the Google AIPs.
#
# api-linter resolves imports from a FileDescriptorSet rather than from proto
# paths, so buf builds the descriptor first: buf already knows how to find the
# vendored googleapis under third_party/, and this keeps one source of truth for
# how the protos compile.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

descriptor="$(mktemp -t api-linter-descriptor.XXXXXX.binpb)"
trap 'rm -f "$descriptor"' EXIT

node_modules/.bin/buf build --as-file-descriptor-set -o "$descriptor" 2>/dev/null

# Lint only this project's protos; the vendored googleapis are dependencies.
mapfile -t protos < <(cd proto && find . -name '*.proto' | sed 's|^\./||' | sort)

.bin/api-linter \
  --config api-linter.yaml \
  --descriptor-set-in "$descriptor" \
  --set-exit-status \
  --output-format="${API_LINTER_FORMAT:-summary}" \
  "${protos[@]}"
