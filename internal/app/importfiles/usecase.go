// Package importfiles importiert DMARC-Reports aus lokalen Dateien
// (.eml, .xml, .xml.gz, .zip) — FEATURES.md Vorschlag 11.1: "Datei-/
// Ordner-Import per Drag & Drop einlesen. Schon für die Entwicklung
// unverzichtbar, ermöglicht Offline-Betrieb und Migration von
// Altbeständen."
//
// Bewusste Abweichung vom ursprünglichen Entwurf ("fällt fast nebenbei
// ab, weil MessageSource bereits abstrahiert ist", IMPLEMENTIERUNG.md
// Vorschlag 11.1): sync.MessageSource.Connect verlangt account.MailAccount
// und ein Secret — ein lokaler Dateiimport hat keine IMAP-Zugangsdaten,
// durch dieses Interface zu gehen wäre erzwungen statt natürlich. Dieser
// Use Case teilt sich stattdessen die MIME-Zerlegung
// (sync.MessageDecoder) und das Parsen (sync.ReportParser) mit
// syncreports, ohne MessageSource selbst zu implementieren.
package importfiles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// UseCase orchestriert den Datei-Import.
type UseCase struct {
	Reports       report.Repository
	FailedImports domainsync.FailedImportRepository
	Decoder       domainsync.MessageDecoder
	Parsers       []domainsync.ReportParser
}

// Result fasst einen Importlauf zusammen, analog zu syncreports.Result.
type Result struct {
	New     int
	Skipped int
	Failed  int
	Errors  []error
}

func (r *Result) add(other Result) {
	r.New += other.New
	r.Skipped += other.Skipped
	r.Failed += other.Failed
	r.Errors = append(r.Errors, other.Errors...)
}

// ImportPaths importiert jede angegebene Datei. Eine einzelne kaputte
// Datei bricht den gesamten Lauf nicht ab (IMPLEMENTIERUNG.md
// Abschnitt 7.2: Fehlerquarantäne statt Abbruch).
func (uc *UseCase) ImportPaths(ctx context.Context, paths []string) (Result, error) {
	total := Result{}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return total, err
		}

		res, err := uc.ImportFile(ctx, path)
		if err != nil {
			total.Failed++
			total.Errors = append(total.Errors, fmt.Errorf("datei %q: %w", path, err))
			continue
		}
		total.add(res)
	}
	return total, nil
}

// ImportFile liest eine einzelne Datei und importiert alle darin
// enthaltenen Reports (bei .eml ggf. mehrere Anhänge, bei .zip ggf.
// mehrere Reports in einem Anhang — siehe dmarcxml.Parser.ParseAll).
func (uc *UseCase) ImportFile(ctx context.Context, path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("datei konnte nicht gelesen werden: %w", err)
	}

	return uc.ImportData(ctx, filepath.Base(path), data)
}

// ImportData importiert eine Datei, die bereits als Bytes im Speicher
// vorliegt, statt von der Festplatte gelesen zu werden — Grundlage für
// den Datei-Upload der Web-Oberfläche (MIGRATIONSPLAN.md Erweiterung
// 9.3: "Import aus Bytes"), die keinen lokalen Dateipfad hat (nur den
// vom Browser mitgeschickten Dateinamen und den Anfrage-Body). Dieselbe
// Logik wie ImportFile ab dem Punkt, an dem die Datei bereits gelesen
// ist.
func (uc *UseCase) ImportData(ctx context.Context, filename string, data []byte) (Result, error) {
	attachments, err := uc.extractAttachments(filename, data)
	if err != nil {
		result := Result{Failed: 1, Errors: []error{err}}
		uc.recordFailure(ctx, filename, data, err)
		return result, nil
	}

	return uc.importAttachments(ctx, attachments), nil
}

// extractAttachments liefert bei einer .eml-Datei deren MIME-Anhänge,
// sonst die Datei selbst als einzigen Anhang — dieselbe Unterscheidung,
// die eine IMAP-Nachricht (immer MIME) von einem bereits extrahierten
// Anhang unterscheidet.
func (uc *UseCase) extractAttachments(filename string, data []byte) ([]domainsync.RawAttachment, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".eml") {
		attachments, err := uc.Decoder.Decode(data)
		if err != nil {
			return nil, fmt.Errorf("nachricht konnte nicht zerlegt werden: %w", err)
		}
		return attachments, nil
	}
	return []domainsync.RawAttachment{{Filename: filename, Data: data}}, nil
}

func (uc *UseCase) importAttachments(ctx context.Context, attachments []domainsync.RawAttachment) Result {
	result := Result{}
	for _, att := range attachments {
		parser := uc.findParser(att)
		if parser == nil {
			continue // kein DMARC-Report, z. B. der Textkörper einer .eml
		}

		// ParseAttachment nutzt ParseAll, falls der Parser das zusätzlich
		// implementiert (z. B. dmarcxml.Parser bei einem .zip mit
		// mehreren XML-Dateien) — ein einzelner Anhang kann so mehrere
		// Reports liefern (siehe domainsync.ParseAttachment-Dokumentation).
		reports, err := domainsync.ParseAttachment(ctx, parser, att)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("anhang %q: %w", att.Filename, err))
			uc.recordFailure(ctx, att.Filename, att.Data, err)
			continue
		}

		for _, rep := range reports {
			imported, err := report.SaveIfNew(ctx, uc.Reports, rep)
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Errorf("anhang %q: report konnte nicht gespeichert werden: %w", att.Filename, err))
				continue
			}
			if imported {
				result.New++
			} else {
				result.Skipped++
			}
		}
	}
	return result
}

func (uc *UseCase) findParser(att domainsync.RawAttachment) domainsync.ReportParser {
	for _, p := range uc.Parsers {
		if p.Supports(att) {
			return p
		}
	}
	return nil
}

func (uc *UseCase) recordFailure(ctx context.Context, filename string, raw []byte, cause error) {
	if uc.FailedImports == nil {
		return
	}
	_ = uc.FailedImports.Record(ctx, domainsync.FailedImport{
		Filename: filename,
		Error:    cause.Error(),
		Raw:      raw,
	})
}
