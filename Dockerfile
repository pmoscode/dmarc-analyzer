# syntax=docker/dockerfile:1

# Build stage: CGO_ENABLED=0 has worked for the whole module since the
# move to the embedded web UI (ADR 0001) — modernc.org/sqlite is pure
# Go, no native toolchain needed.
FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# COMMIT must come in as a build arg: .git is excluded via
# .dockerignore, so "go build" can't embed the revision itself here.
ARG VERSION=dev
ARG COMMIT=""
RUN CGO_ENABLED=0 go build \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/dmarc-analyzer \
    ./cmd/dmarc-analyzer

# Create the empty /data directory here, not in the runtime stage: that
# stage has no shell (distroless), but COPY --chown can take over an
# already-created directory with the right ownership.
RUN mkdir -p /data-empty

# Runtime stage: distroless instead of Alpine — no shell/package manager
# in the final image, already runs as non-root (nonroot:nonroot, UID
# 65532), which reduces the attack surface for a service reachable over
# the network (see docs/features/auth.md).
FROM gcr.io/distroless/static-debian12:nonroot

# Version/commit also as OCI labels, so they can be read via
# "docker image inspect" (or "task docker:version") without a running
# container too. In CI, docker/metadata-action overwrites these labels
# with the same values.
ARG VERSION=dev
ARG COMMIT=""
LABEL org.opencontainers.image.title="dmarc-analyzer" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}"

COPY --from=build /out/dmarc-analyzer /dmarc-analyzer

# DMARC_DATA_DIR (see internal/infra/envconfig) — the database lives
# here; mount it as a volume so it survives a container restart.
# --chown=nonroot:nonroot is needed because a volume/directory Docker
# creates automatically would otherwise be owned by root:root — but the
# process runs as nonroot (UID 65532) and couldn't create a database
# otherwise (SQLITE_CANTOPEN, actually reproduced via docker run).
COPY --from=build --chown=nonroot:nonroot /data-empty /data
VOLUME ["/data"]
ENV DMARC_DATA_DIR=/data
ENV DMARC_LISTEN_ADDR=:8080

EXPOSE 8080

# No curl/wget in the image (distroless) — the healthcheck is its own
# subcommand of the same binary (see cmd_healthcheck.go).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/dmarc-analyzer", "healthcheck"]

ENTRYPOINT ["/dmarc-analyzer"]
CMD ["web"]
