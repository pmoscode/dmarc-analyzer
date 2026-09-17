package web

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/retention"
	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// fullDeps sind alle Fakes hinter einem per newTestServerWithAccounts
// gebauten Server — als Struct statt einzelner Rückgabewerte, damit
// Tests gezielt genau die Fakes benennen können, die sie brauchen.
type fullDeps struct {
	accounts    *fakeAccountRepository
	credentials *fakeCredentialStore
	source      *fakeMessageSource
	syncer      *fakeJobSyncer
	settings    *fakeSettingsRepository
}

// newTestServerWithAccounts baut einen Server mit vollständig verdrahteten
// Dependencies (Statistics/Accounts/Credentials/SyncJob) — für Tests von
// /einstellungen, /einrichtung, /entsperren und /abgleich, die alle
// mindestens Accounts brauchen (schon die Anmeldung leitet über "/" auf
// /einrichtung um, wenn Accounts nil oder leer ist, siehe
// handlers_dashboard.go).
func newTestServerWithAccounts(t *testing.T, accounts ...account.MailAccount) (*Server, *fullDeps) {
	t.Helper()
	isolateConfigDir(t)

	fd := &fullDeps{
		accounts:    newFakeAccountRepository(accounts...),
		credentials: newFakeCredentialStore(),
		source:      &fakeMessageSource{},
		syncer:      &fakeJobSyncer{},
		settings:    &fakeSettingsRepository{},
	}

	accountsUC := &manageaccount.UseCase{
		Accounts:    fd.accounts,
		Credentials: fd.credentials,
		NewSource:   func() domainsync.MessageSource { return fd.source },
	}

	deps := Dependencies{
		Statistics:  &statistics.UseCase{Repository: &fakeRepository{}},
		Reports:     &queryreports.UseCase{Reports: &fakeReportRepository{}},
		Sources:     &sourcestats.UseCase{Sources: &fakeSourcesRepository{}, Enricher: &fakeEnricher{}},
		Accounts:    accountsUC,
		Credentials: fd.credentials,
		SyncJob:     syncjob.NewRunner(context.Background(), fd.accounts, fd.syncer),
		Retention:   &retention.UseCase{Settings: fd.settings, Reports: &fakePruner{}},
	}

	srv, err := New(deps, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	_, err = srv.Start(context.Background())
	require.NoError(t, err)
	return srv, fd
}
