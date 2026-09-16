package report

import (
	"context"
	"errors"
)

// SaveIfNew prüft die fachliche Identität von r gegen repo und speichert
// ihn nur, wenn er noch nicht existiert — die gemeinsame
// Deduplizierungslogik für alle Importwege (IMAP-Sync, Datei-Import),
// hier einmal implementiert statt in jedem Use Case wiederholt. Liefert
// true, wenn r tatsächlich neu gespeichert wurde.
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
			// Race zwischen Exists und Save (z. B. derselbe Report über
			// zwei parallele Importwege) — kein Fehler, nur kein Neuimport.
			return false, nil
		}
		return false, err
	}
	return true, nil
}
