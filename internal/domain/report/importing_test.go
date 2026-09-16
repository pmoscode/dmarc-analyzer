package report_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

type fakeSaveIfNewRepo struct {
	report.Repository // eingebettet, nur die drei benötigten Methoden überschrieben
	saved             map[report.Key]bool
	existsErr         error
	saveErr           error
}

func (f *fakeSaveIfNewRepo) Exists(_ context.Context, key report.Key) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.saved[key], nil
}

func (f *fakeSaveIfNewRepo) Save(_ context.Context, r *report.AggregateReport) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if f.saved == nil {
		f.saved = make(map[report.Key]bool)
	}
	f.saved[r.Key()] = true
	return nil
}

func testAggregateReport(t *testing.T) *report.AggregateReport {
	t.Helper()
	dr, err := report.NewDateRange(time.Now(), time.Now().Add(time.Hour))
	require.NoError(t, err)
	r, err := report.NewAggregateReport(
		report.Metadata{OrgName: "org.example", ReportID: "r1", Range: dr},
		report.PublishedPolicy{}, nil, report.SourceReference{}, time.Now(),
	)
	require.NoError(t, err)
	return r
}

func TestSaveIfNew_NewReport_SavesAndReturnsTrue(t *testing.T) {
	t.Parallel()

	repo := &fakeSaveIfNewRepo{}
	imported, err := report.SaveIfNew(context.Background(), repo, testAggregateReport(t))
	require.NoError(t, err)
	require.True(t, imported)
}

func TestSaveIfNew_ExistingReport_SkipsAndReturnsFalse(t *testing.T) {
	t.Parallel()

	repo := &fakeSaveIfNewRepo{}
	r := testAggregateReport(t)
	_, err := report.SaveIfNew(context.Background(), repo, r)
	require.NoError(t, err)

	imported, err := report.SaveIfNew(context.Background(), repo, r)
	require.NoError(t, err)
	require.False(t, imported)
}

func TestSaveIfNew_ExistsError_IsForwarded(t *testing.T) {
	t.Parallel()

	repo := &fakeSaveIfNewRepo{existsErr: errors.New("boom")}
	_, err := report.SaveIfNew(context.Background(), repo, testAggregateReport(t))
	require.Error(t, err)
}

func TestSaveIfNew_SaveErrDuplicate_TreatedAsSkip(t *testing.T) {
	t.Parallel()

	repo := &fakeSaveIfNewRepo{saveErr: report.ErrDuplicate}
	imported, err := report.SaveIfNew(context.Background(), repo, testAggregateReport(t))
	require.NoError(t, err, "ErrDuplicate ist kein Fehlerfall für den Aufrufer, nur kein Neuimport")
	require.False(t, imported)
}

func TestSaveIfNew_SaveOtherError_IsForwarded(t *testing.T) {
	t.Parallel()

	repo := &fakeSaveIfNewRepo{saveErr: errors.New("disk full")}
	_, err := report.SaveIfNew(context.Background(), repo, testAggregateReport(t))
	require.Error(t, err)
	require.NotErrorIs(t, err, report.ErrDuplicate)
}
