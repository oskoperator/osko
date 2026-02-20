# Controls binary source: "build" (default, local) or "pre" (CI, pre-built binary in context)
ARG BIN_SOURCE=build

# Build the manager binary (skipped by BuildKit when BIN_SOURCE=pre)
FROM golang:1.23 AS go-builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o manager cmd/main.go

# Pre-built binary from build context (CI only)
FROM scratch AS bin-pre
ARG TARGETARCH
COPY manager-${TARGETARCH} /manager

# Binary from Go builder stage (local builds)
FROM scratch AS bin-build
COPY --from=go-builder /workspace/manager /manager

FROM bin-${BIN_SOURCE} AS bin

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=bin /manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
