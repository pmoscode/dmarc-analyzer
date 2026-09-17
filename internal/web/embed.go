package web

import "embed"

// embeddedTemplates und embeddedStatic betten HTML-Vorlagen und
// statische Dateien (CSS/JS/vendor) in die Binärdatei ein
// (MIGRATIONSPLAN.md Abschnitt 6: "vollständig in der Binärdatei").
// Mit Options.Dev werden stattdessen die Dateien auf der Festplatte
// gelesen (siehe views.go, static.go).
//
//go:embed templates
var embeddedTemplates embed.FS

//go:embed static
var embeddedStatic embed.FS
