package main

import (
	"context"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// --- report.Repository -----------------------------------------------

type fakeReportRepository struct {
	byKey  map[report.Key]*report.AggregateReport
	nextID int64
}

func newFakeReportRepository() *fakeReportRepository {
	return &fakeReportRepository{byKey: make(map[report.Key]*report.AggregateReport)}
}

func (f *fakeReportRepository) Save(_ context.Context, r *report.AggregateReport) error {
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
	_, ok := f.byKey[key]
	return ok, nil
}

func (f *fakeReportRepository) FindByID(context.Context, report.ReportID) (*report.AggregateReport, error) {
	return nil, nil //nolint:nilnil // im Test nicht benötigt
}

func (f *fakeReportRepository) Query(context.Context, report.Query) (report.Page, error) {
	return report.Page{}, nil
}

// --- domainsync.MessageDecoder -----------------------------------------

// fakeDecoder behandelt jede Nachricht als genau einen Anhang mit dem
// Nachrichteninhalt als Dateiname.
type fakeDecoder struct{}

func (fakeDecoder) Decode(data []byte) ([]domainsync.RawAttachment, error) {
	return []domainsync.RawAttachment{{Filename: string(data), Data: data}}, nil
}

// --- domainsync.ReportParser --------------------------------------------

// fakeParser akzeptiert jeden Anhang und liefert einen Report, dessen
// ReportID dem Anhangsinhalt entspricht.
type fakeParser struct{}

func (fakeParser) Supports(domainsync.RawAttachment) bool { return true }

func (fakeParser) Parse(_ context.Context, att domainsync.RawAttachment) (*report.AggregateReport, error) {
	domain, err := report.NewDomainName("example.com")
	if err != nil {
		return nil, err
	}
	dr, err := report.NewDateRange(fixedBegin, fixedBegin.Add(hourDuration))
	if err != nil {
		return nil, err
	}
	policy, err := report.NewPublishedPolicy(domain, report.PolicyReject, report.PolicyReject, report.AlignmentRelaxed, report.AlignmentRelaxed, 100, "")
	if err != nil {
		return nil, err
	}
	return report.NewAggregateReport(
		report.Metadata{OrgName: "fake-org.example", ReportID: string(att.Data), Range: dr},
		policy, nil, report.SourceReference{}, fixedBegin,
	)
}

// --- account.Repository -------------------------------------------------

type fakeAccountRepository struct {
	accounts map[account.AccountID]account.MailAccount
}

func newFakeAccountRepository(accs ...account.MailAccount) *fakeAccountRepository {
	m := make(map[account.AccountID]account.MailAccount, len(accs))
	for _, a := range accs {
		m[a.ID] = a
	}
	return &fakeAccountRepository{accounts: m}
}

func (f *fakeAccountRepository) Save(_ context.Context, a *account.MailAccount) error {
	f.accounts[a.ID] = *a
	return nil
}

func (f *fakeAccountRepository) FindByID(_ context.Context, id account.AccountID) (*account.MailAccount, error) {
	a, ok := f.accounts[id]
	if !ok {
		return nil, errNotFound
	}
	return &a, nil
}

func (f *fakeAccountRepository) FindAll(context.Context) ([]account.MailAccount, error) {
	all := make([]account.MailAccount, 0, len(f.accounts))
	for _, a := range f.accounts {
		all = append(all, a)
	}
	return all, nil
}

var errNotFound = fakeErr("nicht gefunden")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }
