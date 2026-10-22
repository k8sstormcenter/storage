FROM --platform=$BUILDPLATFORM golang:1.26-trixie AS builder

WORKDIR /work
ARG TARGETOS TARGETARCH
ENV GO111MODULE=on CGO_ENABLED=0

RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/migration ./cmd/migration/main.go

RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/resourceprefixmigration ./cmd/resourceprefixmigration

RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/cpexport ./cmd/cpexport/main.go

RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/storage .

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder /out/storage /usr/bin/storage
COPY --from=builder /out/migration /usr/bin/migration
COPY --from=builder /out/resourceprefixmigration /usr/bin/resourceprefixmigration
COPY --from=builder /out/cpexport /usr/bin/cpexport
ARG image_version
ARG BUILD_REVISION=""
ENV RELEASE=$image_version
LABEL org.opencontainers.image.title="storage" \
      org.opencontainers.image.source="https://github.com/k8sstormcenter/storage" \
      org.opencontainers.image.version="$image_version" \
      org.opencontainers.image.revision="$BUILD_REVISION"

ENTRYPOINT ["storage"]
