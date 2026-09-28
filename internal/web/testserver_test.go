package web

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/domainoverview"
	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/retention"
	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// fullDeps holds all the fakes behind a server built via
// newTestServerWithAccounts — as a struct instead of individual return
// values, so tests can name exactly the fakes they need.
type fullDeps struct {
	accounts *fakeAccountRepository
	source   *fakeMessageSource
	syncer   *fakeJobSyncer
}

// newTestServerWithAccounts builds a server with fully wired-up
// dependencies (Statistics/Accounts/SyncJob) — for tests of
// /einstellungen and /abgleich.
func newTestServerWithAccounts(t *testing.T, accounts ...account.MailAccount) (*Server, *fullDeps) {
	t.Helper()

	fd := &fullDeps{
		accounts: newFakeAccountRepository(accounts...),
		source:   &fakeMessageSource{},
		syncer:   &fakeJobSyncer{},
	}

	accountsUC := &manageaccount.UseCase{
		Accounts:  fd.accounts,
		NewSource: func() domainsync.MessageSource { return fd.source },
	}

	deps := Dependencies{
		Statistics:          &statistics.UseCase{Repository: &fakeRepository{}},
		Reports:             &queryreports.UseCase{Reports: &fakeReportRepository{}},
		Sources:             &sourcestats.UseCase{Sources: &fakeSourcesRepository{}, Enricher: &fakeEnricher{}},
		Domains:             &domainoverview.UseCase{Domains: &fakeDomainsRepository{}},
		Accounts:            accountsUC,
		SyncJob:             syncjob.NewRunner(context.Background(), fd.accounts, fd.syncer),
		Retention:           &retention.UseCase{RetentionMonths: 24, Reports: &fakePruner{}},
		SyncIntervalMinutes: 60,
	}

	provider := newFakeOIDCProvider(t)
	srv, err := New(context.Background(), deps, testOIDCOptions(provider.issuer()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	require.NoError(t, srv.Start(context.Background()))
	return srv, fd
}
