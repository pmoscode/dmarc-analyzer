package queryfailedrecords_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/queryfailedrecords"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
)

var errTest = errors.New("testfehler")

type fakeFailedRecordsRepository struct {
	page failedrecords.Page
	err  error
	got  failedrecords.Query
}

func (f *fakeFailedRecordsRepository) Query(_ context.Context, q failedrecords.Query) (failedrecords.Page, error) {
	f.got = q
	if f.err != nil {
		return failedrecords.Page{}, f.err
	}
	return f.page, nil
}

func TestUseCase_List_ForwardsQueryToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeFailedRecordsRepository{}
	uc := &queryfailedrecords.UseCase{Records: repo}

	q := failedrecords.Query{Domain: "example.com", SourceIP: "203.0.113.1", Limit: 25, Cursor: "abc"}
	_, err := uc.List(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, q, repo.got)
}

func TestUseCase_List_ReturnsRepositoryPage(t *testing.T) {
	t.Parallel()

	page := failedrecords.Page{Records: []failedrecords.Record{{Count: 5}}}
	uc := &queryfailedrecords.UseCase{Records: &fakeFailedRecordsRepository{page: page}}

	got, err := uc.List(context.Background(), failedrecords.Query{})
	require.NoError(t, err)
	require.Equal(t, page, got)
}

func TestUseCase_List_RepositoryError_IsForwarded(t *testing.T) {
	t.Parallel()

	uc := &queryfailedrecords.UseCase{Records: &fakeFailedRecordsRepository{err: errTest}}

	_, err := uc.List(context.Background(), failedrecords.Query{})
	require.ErrorIs(t, err, errTest)
}
