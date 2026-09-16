package syncreports

// progressTracker verwandelt möglicherweise unsortiert abgeschlossene
// UIDs (Ergebnis nebenläufiger Verarbeitung, IMPLEMENTIERUNG.md
// Abschnitt 7.3) in eine lückenlose Fortschrittsgrenze: LastUID rückt nur
// vor, solange keine kleinere UID noch offen ist. So verliert ein Absturz
// höchstens die gerade in Bearbeitung befindlichen Nachrichten, nie eine
// bereits gespeicherte (IMPLEMENTIERUNG.md Abschnitt 7.1, Schritt 6).
//
// Nicht nebenläufigkeitssicher — wird ausschließlich von der einzelnen
// Schreiber-Goroutine in usecase.go aufgerufen, die Ergebnisse ohnehin
// seriell über einen Channel konsumiert.
type progressTracker struct {
	lastUID   uint32
	completed map[uint32]struct{}
}

func newProgressTracker(startAfter uint32) *progressTracker {
	return &progressTracker{lastUID: startAfter, completed: make(map[uint32]struct{})}
}

// markDone vermerkt uid als abgeschlossen (erfolgreich gespeichert ODER
// endgültig in die Fehlerquarantäne verschoben — beides "erledigt" im
// Sinne des Fortschritts) und liefert die neue lückenlose Obergrenze sowie,
// ob sie sich gegenüber dem letzten Aufruf verändert hat.
func (p *progressTracker) markDone(uid uint32) (lastUID uint32, advanced bool) {
	p.completed[uid] = struct{}{}

	for {
		next := p.lastUID + 1
		if _, ok := p.completed[next]; !ok {
			break
		}
		delete(p.completed, next)
		p.lastUID = next
		advanced = true
	}

	return p.lastUID, advanced
}
