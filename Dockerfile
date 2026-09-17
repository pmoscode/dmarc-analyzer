# syntax=docker/dockerfile:1

# Build-Stage: CGO_ENABLED=0 funktioniert seit dem Umstieg auf die
# eingebettete Web-Oberfläche (ADR 0001) für das gesamte Modul —
# modernc.org/sqlite ist reines Go, keine native Toolchain nötig.
FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -ldflags "-X main.version=${VERSION}" \
    -o /out/dmarc-analyzer \
    ./cmd/dmarc-analyzer

# Leeres /data-Verzeichnis hier anlegen, nicht in der Runtime-Stage: die
# hat keine Shell (distroless), aber COPY --chown kann ein bereits
# angelegtes Verzeichnis mit der richtigen Eigentümerschaft übernehmen.
RUN mkdir -p /data-empty

# Runtime-Stage: distroless statt Alpine — kein Shell/Paketmanager im
# fertigen Image, läuft bereits als nicht-root (nonroot:nonroot, UID
# 65532), reduziert die Angriffsfläche für einen Dienst, der übers
# Netzwerk erreichbar ist (siehe docs/features/auth.md).
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/dmarc-analyzer /dmarc-analyzer

# DMARC_DATA_DIR (siehe internal/infra/envconfig) — Datenbank liegt hier,
# als Volume zu mounten, damit sie einen Container-Neustart übersteht.
# --chown=nonroot:nonroot ist nötig, weil ein von Docker automatisch
# angelegtes Volume/Verzeichnis sonst root:root gehört — der Prozess
# läuft aber als nonroot (UID 65532) und könnte sonst keine Datenbank
# anlegen (SQLITE_CANTOPEN, per docker run tatsächlich reproduziert).
COPY --from=build --chown=nonroot:nonroot /data-empty /data
VOLUME ["/data"]
ENV DMARC_DATA_DIR=/data
ENV DMARC_LISTEN_ADDR=:8080

EXPOSE 8080

# Kein curl/wget im Image (distroless) — healthcheck ist ein eigener
# Unterbefehl derselben Binärdatei (siehe cmd_healthcheck.go).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/dmarc-analyzer", "healthcheck"]

ENTRYPOINT ["/dmarc-analyzer"]
CMD ["web"]
