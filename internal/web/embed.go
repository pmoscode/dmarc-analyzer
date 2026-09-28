package web

import "embed"

// embeddedTemplates and embeddedStatic embed HTML templates and static
// files (CSS/JS/vendor) into the binary (MIGRATIONSPLAN.md section 6:
// "fully contained in the binary"). With Options.Dev, the files are read
// from disk instead (see views.go, static.go).
//
//go:embed templates
var embeddedTemplates embed.FS

//go:embed static
var embeddedStatic embed.FS
