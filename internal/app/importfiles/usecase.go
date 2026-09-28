// Package importfiles imports DMARC reports from local files (.eml,
// .xml, .xml.gz, .zip) — FEATURES.md proposal 11.1: "read file/folder
// import via drag & drop. Indispensable for development already, enables
// offline operation and migration of legacy data."
//
// Deliberate deviation from the original design ("comes almost for free,
// since MessageSource is already abstracted", IMPLEMENTIERUNG.md
// proposal 11.1): sync.MessageSource.Connect requires account.MailAccount
// and a Secret — a local file import has no IMAP credentials, going
// through this interface would be forced rather than natural. This use
// case instead shares the MIME decomposition (sync.MessageDecoder) and
// parsing (sync.ReportParser) with syncreports, without implementing
// MessageSource itself.
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

// UseCase orchestrates the file import.
type UseCase struct {
	Reports       report.Repository
	FailedImports domainsync.FailedImportRepository
	Decoder       domainsync.MessageDecoder
	Parsers       []domainsync.ReportParser
}

// Result summarizes an import run, analogous to syncreports.Result.
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

// ImportPaths imports each given file. A single broken file doesn't
// abort the whole run (IMPLEMENTIERUNG.md section 7.2: error quarantine
// instead of abort).
func (uc *UseCase) ImportPaths(ctx context.Context, paths []string) (Result, error) {
	total := Result{}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return total, err
		}

		res, err := uc.ImportFile(ctx, path)
		if err != nil {
			total.Failed++
			total.Errors = append(total.Errors, fmt.Errorf("file %q: %w", path, err))
			continue
		}
		total.add(res)
	}
	return total, nil
}

// ImportFile reads a single file and imports all reports contained in it
// (possibly several attachments for .eml, possibly several reports in
// one attachment for .zip — see dmarcxml.Parser.ParseAll).
func (uc *UseCase) ImportFile(ctx context.Context, path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("file could not be read: %w", err)
	}

	return uc.ImportData(ctx, filepath.Base(path), data)
}

// ImportData imports a file that already exists as bytes in memory,
// instead of being read from disk — the basis for the web UI's file
// upload (MIGRATIONSPLAN.md extension 9.3: "import from bytes"), which
// has no local file path (only the filename sent by the browser and the
// request body). Same logic as ImportFile from the point where the file
// has already been read.
func (uc *UseCase) ImportData(ctx context.Context, filename string, data []byte) (Result, error) {
	attachments, err := uc.extractAttachments(filename, data)
	if err != nil {
		result := Result{Failed: 1, Errors: []error{err}}
		uc.recordFailure(ctx, filename, data, err)
		return result, nil
	}

	return uc.importAttachments(ctx, attachments), nil
}

// extractAttachments returns the MIME attachments of an .eml file, or
// the file itself as the only attachment otherwise — the same
// distinction that separates an IMAP message (always MIME) from an
// already extracted attachment.
func (uc *UseCase) extractAttachments(filename string, data []byte) ([]domainsync.RawAttachment, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".eml") {
		attachments, err := uc.Decoder.Decode(data)
		if err != nil {
			return nil, fmt.Errorf("message could not be decoded: %w", err)
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
			continue // not a DMARC report, e.g. the text body of an .eml
		}

		// ParseAttachment uses ParseAll if the parser additionally
		// implements it (e.g. dmarcxml.Parser for a .zip with multiple
		// XML files) — a single attachment can thus yield multiple
		// reports (see domainsync.ParseAttachment documentation).
		reports, err := domainsync.ParseAttachment(ctx, parser, att)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("attachment %q: %w", att.Filename, err))
			uc.recordFailure(ctx, att.Filename, att.Data, err)
			continue
		}

		for _, rep := range reports {
			imported, err := report.SaveIfNew(ctx, uc.Reports, rep)
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Errorf("attachment %q: report could not be saved: %w", att.Filename, err))
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
