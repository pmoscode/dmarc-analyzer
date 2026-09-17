// Package config verwaltet nicht-geheime Einstellungen (Sync-Intervall,
// Aufbewahrungsdauer) — implementiert gegen eine JSON-Datei im
// Konfigurationsverzeichnis (AP 7), analog zu internal/infra/keyring.
// FileStore für Zugangsdaten, nur ohne Verschlüsselung: hier stehen
// bewusst keine Geheimnisse.
package config
