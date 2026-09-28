package report

import (
	"context"
	"errors"
)

// SaveIfNew checks r's business identity against repo and only saves it
// if it doesn't already exist — the shared deduplication logic for all
// import paths (IMAP sync, file import), implemented once here instead of
// repeated in every use case. Returns true if r was actually newly saved.
func SaveIfNew(ctx context.Context, repo Repository, r *AggregateReport) (imported bool, err error) {
	exists, err := repo.Exists(ctx, r.Key())
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	if err := repo.Save(ctx, r); err != nil {
		if errors.Is(err, ErrDuplicate) {
			// Race between Exists and Save (e.g. the same report via two
			// parallel import paths) — not an error, just no new import.
			return false, nil
		}
		return false, err
	}
	return true, nil
}
