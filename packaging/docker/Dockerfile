# syntax=docker/dockerfile:1
# Build from the repository root:
#   docker build -f packaging/docker/Dockerfile -t haepage .

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
COPY themes/ /src/themes/
COPY LICENSE THIRD_PARTY_NOTICES.md /src/
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/haelahpx/haepage/internal/version.Version=${VERSION} -X github.com/haelahpx/haepage/internal/version.Commit=${COMMIT}" \
      -o /out/haepage ./cmd/haepage \
 && mkdir -p /out/data

# Static binary on a minimal image: no shell, CA certificates included,
# runs as the unprivileged "nonroot" user (uid 65532).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/haepage /haepage
COPY --from=build /src/LICENSE /src/THIRD_PARTY_NOTICES.md /usr/share/doc/haepage/
COPY --from=build --chown=65532:65532 /out/data /data
ENV HAEPAGE_DATA=/data \
    HAEPAGE_ADDR=:8080
VOLUME /data
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/haepage"]
