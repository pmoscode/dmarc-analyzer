#!/usr/bin/env bash
# Erzeugt einmalig eine lokale Test-CA + Server-Zertifikat für
# dmarc.localtest.me/idp.localtest.me (von Caddy verwendet) sowie ein
# kombiniertes CA-Bundle, dem dmarc-analyzer zusätzlich vertraut (siehe
# docker-compose.override.yml) — Ersatz für Caddys "tls internal", das
# dmarc-analyzer als Go-Binary im distroless-Image sonst nicht vertraut
# (x509: certificate signed by unknown authority).
#
# Nur für lokales Testen. Ausgabe wird bewusst nicht committet (siehe
# .gitignore) — einmalig ausführen, danach `docker compose up --build`:
#
#   docker compose build dmarc-analyzer   # Bundle wird aus diesem Image extrahiert
#   ./caddy/local-ca/generate.sh
#   docker compose up
set -euo pipefail
cd "$(dirname "$0")"

if [ -f root-ca.key ]; then
	echo "Bereits vorhanden — zum Neuerzeugen erst 'rm -f caddy/local-ca/*.key caddy/local-ca/*.crt' ausführen." >&2
	exit 0
fi

# 1) Root-CA
openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes \
	-keyout root-ca.key -out root-ca.crt \
	-subj "/CN=dmarc-analyzer local dev CA" \
	-addext basicConstraints=critical,CA:true \
	-addext keyUsage=critical,keyCertSign,cRLSign

# 2) Server-Zertifikat für beide Hostnamen
openssl req -newkey rsa:2048 -sha256 -nodes \
	-keyout server.key -out server.csr \
	-subj "/CN=dmarc.localtest.me"

cat >server.ext <<'EOF'
subjectAltName = DNS:dmarc.localtest.me, DNS:idp.localtest.me
extendedKeyUsage = serverAuth
EOF

openssl x509 -req -in server.csr -CA root-ca.crt -CAkey root-ca.key \
	-CAcreateserial -out server.crt -days 825 -sha256 -extfile server.ext

rm -f server.csr server.ext root-ca.srl

# 3) Kombiniertes Bundle für dmarc-analyzer: Debians Standard-CAs (aus dem
# gebauten Image extrahiert, damit echte HTTPS-Aufrufe z. B. gegen einen
# echten IMAP/OIDC-Server weiter funktionieren) + unsere lokale Test-CA.
IMG=$(docker compose -f ../../docker-compose.yml config --images 2>/dev/null | grep dmarc-analyzer | head -1 || true)
if [ -z "$IMG" ]; then
	echo "Kein gebautes dmarc-analyzer-Image gefunden — zuerst 'docker compose build dmarc-analyzer' ausführen." >&2
	exit 1
fi
CID=$(docker create --entrypoint="" "$IMG" /dmarc-analyzer)
docker cp "$CID:/etc/ssl/certs/ca-certificates.crt" ./debian-ca-bundle.crt
docker rm "$CID" >/dev/null
cat ./debian-ca-bundle.crt root-ca.crt >combined-ca-bundle.crt

echo "Fertig: caddy/local-ca/{root-ca.crt,server.crt,server.key,combined-ca-bundle.crt}"
