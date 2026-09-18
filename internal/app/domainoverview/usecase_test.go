package domainoverview_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/domainoverview"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
)

var errTest = errors.New("testfehler")

type fakeDomainsRepository struct {
	page domainstats.Page
	err  error
	got  domainstats.Query
}

func (f *fakeDomainsRepository) Query(_ context.Context, q domainstats.Query) (domainstats.Page, error) {
	f.got = q
	if f.err != nil {
		return domainstats.Page{}, f.err
	}
	return f.page, nil
}

func TestUseCase_List_ForwardsQueryToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeDomainsRepository{}
	uc := &domainoverview.UseCase{Domains: repo}

	q := domainstats.Query{Domain: "example.com", Limit: 25, Cursor: "abc"}
	_, err := uc.List(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, q, repo.got)
}

func TestUseCase_List_ReturnsRepositoryPage(t *testing.T) {
	t.Parallel()

	page := domainstats.Page{Stats: []domainstats.Stat{{TotalCount: 5}}}
	uc := &domainoverview.UseCase{Domains: &fakeDomainsRepository{page: page}}

	got, err := uc.List(context.Background(), domainstats.Query{})
	require.NoError(t, err)
	require.Equal(t, page, got)
}

func TestUseCase_List_RepositoryError_IsForwarded(t *testing.T) {
	t.Parallel()

	uc := &domainoverview.UseCase{Domains: &fakeDomainsRepository{err: errTest}}

	_, err := uc.List(context.Background(), domainstats.Query{})
	require.ErrorIs(t, err, errTest)
}
