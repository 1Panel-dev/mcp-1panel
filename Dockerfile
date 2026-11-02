# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.27.3 AS build
WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . ./

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION
# Build on the builder's native platform and reuse the Linux binary builder.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -eu; \
    sh scripts/validate-version.sh "$VERSION"; \
    test "$TARGETOS" = linux; \
    target_arch="$TARGETARCH"; \
    if [ "$TARGETARCH" = arm ]; then \
        case "$TARGETVARIANT" in v7|'') target_arch=armv7 ;; \
            *) echo "unsupported ARM variant: $TARGETVARIANT" >&2; exit 2 ;; esac; \
    fi; \
    VERSION="$VERSION" sh scripts/build-linux.sh /out "$target_arch"; \
    cp "/out/mcp-1panel-linux-$target_arch" /out/mcp-1panel

FROM alpine:3.21.3
RUN apk add --no-cache ca-certificates
WORKDIR /server
ARG VERSION
LABEL org.opencontainers.image.title="1Panel MCP Server" \
      org.opencontainers.image.source="https://github.com/1Panel-dev/mcp-1panel" \
      org.opencontainers.image.version="$VERSION"
COPY --from=build --chmod=0755 /out/mcp-1panel /usr/local/bin/mcp-1panel
ENTRYPOINT ["/usr/local/bin/mcp-1panel"]
CMD ["--mode=legacy", "--transport=stdio"]
