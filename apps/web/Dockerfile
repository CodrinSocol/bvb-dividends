# Builds the client and serves the static result.
#
# The build runs from the repository root so it can read the OpenAPI document
# the client's types are generated from:
#   docker build -f apps/web/Dockerfile .

FROM oven/bun:1.3-alpine AS build
WORKDIR /src

COPY package.json bun.lock ./
COPY apps/web/package.json ./apps/web/package.json
RUN bun install --frozen-lockfile

COPY apps/web ./apps/web
COPY libs/apiserver/openapi ./libs/apiserver/openapi

# Regenerated rather than trusted from the build context, so an image can never
# ship client types that disagree with the specification beside them.
RUN bunx openapi-typescript libs/apiserver/openapi/openapi.yaml -o apps/web/src/api/schema.d.ts \
    && cd apps/web && bunx tsc --noEmit && bunx vite build

FROM nginx:1.27-alpine
COPY --from=build /src/dist/apps/web /usr/share/nginx/html
COPY apps/web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
