package importfiles_test

import (
	"context"
	"strings"
	stdsync "sync"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// --- report.Repository ---------------------------------------------------

type fakeReportRepository struct {
	mu      stdsync.Mutex
	byKey   map[report.Key]*report.AggregateReport
	nextID  int64
	saveErr error
}

func newFakeReportRepository() *fakeReportRepository {
	return &fakeReportRepository{byKey: make(map[report.Key]*report.AggregateReport)}
}

func (f *fakeReportRepository) Save(_ context.Context, r *report.AggregateReport) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	if _, exists := f.byKey[r.Key()]; exists {
		return report.ErrDuplicate
	}
	f.nextID++
	r.ID = report.ReportID(f.nextID)
	cp := *r
	f.byKey[r.Key()] = &cp
	return nil
}

func (f *fakeReportRepository) Exists(_ context.Context, key report.Key) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.byKey[key]
	return ok, nil
}

func (f *fakeReportRepository) FindByID(context.Context, report.ReportID) (*report.AggregateReport, error) {
	return nil, nil //nolint:nilnil // im Test nicht benötigt
}

func (f *fakeReportRepository) Query(context.Context, report.Query) (report.Page, error) {
	return report.Page{}, nil
}

func (f *fakeReportRepository) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byKey)
}

// --- domainsync.FailedImportRepository ------------------------------------

type fakeFailedImportRepository struct {
	mu      stdsync.Mutex
	entries []domainsync.FailedImport
}

func newFakeFailedImportRepository() *fakeFailedImportRepository {
	return &fakeFailedImportRepository{}
}

func (f *fakeFailedImportRepository) Record(_ context.Context, entry domainsync.FailedImport) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeFailedImportRepository) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.entries)
}

// --- domainsync.MessageDecoder ---------------------------------------------

// fakeDecoder behandelt jede .eml-Datei als genau einen Anhang mit dem
// Dateiinhalt als "Filename" (praktisch für Tests: der Inhalt steuert,
// welcher Report daraus entsteht).
type fakeDecoder struct{}

func (fakeDecoder) Decode(data []byte) ([]domainsync.RawAttachment, error) {
	return []domainsync.RawAttachment{{Filename: string(data), Data: data}}, nil
}

// --- domainsync.ReportParser -----------------------------------------------

// fakeParser akzeptiert jeden Anhang außer solche mit dem Inhalt
// "unsupported" (simuliert einen Nicht-DMARC-Anhang) und liefert einen
// Report, dessen ReportID dem Anhangsinhalt entspricht.
type fakeParser struct {
	failFor map[string]error
}

func (p fakeParser) Supports(att domainsync.RawAttachment) bool {
	return string(att.Data) != "unsupported"
}

func (p fakeParser) Parse(_ context.Context, att domainsync.RawAttachment) (*report.AggregateReport, error) {
	content := string(att.Data)
	if err, ok := p.failFor[content]; ok {
		return nil, err
	}

	domain, err := report.NewDomainName("example.com")
	if err != nil {
		return nil, err
	}
	dr, err := report.NewDateRange(fixedBegin, fixedBegin.Add(hour))
	if err != nil {
		return nil, err
	}
	policy, err := report.NewPublishedPolicy(domain, report.PolicyReject, report.PolicyReject, report.AlignmentRelaxed, report.AlignmentRelaxed, 100, "")
	if err != nil {
		return nil, err
	}

	return report.NewAggregateReport(
		report.Metadata{OrgName: "fake-org.example", ReportID: content, Range: dr},
		policy, nil, report.SourceReference{}, fixedBegin,
	)
}

// fakeMultiParser implementiert zusätzlich domainsync.MultiReportParser
// — simuliert einen Anhang (z. B. ein .zip), der mehrere Reports auf
// einmal enthält. Anhangsinhalt "multi:a,b,c" liefert drei Reports mit
// den ReportIDs a, b, c.
type fakeMultiParser struct{}

func (p fakeMultiParser) Supports(att domainsync.RawAttachment) bool {
	return strings.HasPrefix(string(att.Data), "multi:")
}

func (p fakeMultiParser) Parse(ctx context.Context, att domainsync.RawAttachment) (*report.AggregateReport, error) {
	all, err := p.ParseAll(ctx, att)
	if err != nil {
		return nil, err
	}
	return all[0], nil
}

func (p fakeMultiParser) ParseAll(_ context.Context, att domainsync.RawAttachment) ([]*report.AggregateReport, error) {
	ids := strings.Split(strings.TrimPrefix(string(att.Data), "multi:"), ",")

	domain, err := report.NewDomainName("example.com")
	if err != nil {
		return nil, err
	}
	dr, err := report.NewDateRange(fixedBegin, fixedBegin.Add(hour))
	if err != nil {
		return nil, err
	}
	policy, err := report.NewPublishedPolicy(domain, report.PolicyReject, report.PolicyReject, report.AlignmentRelaxed, report.AlignmentRelaxed, 100, "")
	if err != nil {
		return nil, err
	}

	reports := make([]*report.AggregateReport, 0, len(ids))
	for _, id := range ids {
		rep, err := report.NewAggregateReport(
			report.Metadata{OrgName: "fake-org.example", ReportID: id, Range: dr},
			policy, nil, report.SourceReference{}, fixedBegin,
		)
		if err != nil {
			return nil, err
		}
		reports = append(reports, rep)
	}
	return reports, nil
}
