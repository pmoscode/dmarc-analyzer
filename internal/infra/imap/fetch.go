package imap

import (
	"context"
	"fmt"
	"iter"

	"github.com/emersion/go-imap/v2"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// FetchNew wählt das Postfach aus (EXAMINE, nicht SELECT — read-only, siehe
// Adapter-Dokumentation) und liefert einen Iterator über alle Nachrichten
// ab state.LastUID+1. Ändert sich UIDValidity gegenüber state (oder ist
// state.UIDValidity 0, also ein erster Sync), wird stattdessen ab UID 1
// neu gescannt — Duplikate fängt der UNIQUE-Index in der Persistenz ab
// (IMPLEMENTIERUNG.md Abschnitt 7.1).
func (a *Adapter) FetchNew(ctx context.Context, state sync.State) (iter.Seq2[sync.RawMessage, error], sync.State, error) {
	if a.client == nil {
		return nil, state, errNotConnected
	}

	var selectData *imap.SelectData
	selectErr := runCtx(ctx, a.client, func() error {
		data, err := a.client.Select(state.Mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
		selectData = data
		return err
	})
	if selectErr != nil {
		return nil, state, fmt.Errorf("postfach %q konnte nicht ausgewählt werden: %w", state.Mailbox, selectErr)
	}

	rescan := state.UIDValidity == 0 || state.UIDValidity != selectData.UIDValidity
	baseline := sync.State{
		AccountID:   state.AccountID,
		Mailbox:     state.Mailbox,
		UIDValidity: selectData.UIDValidity,
		LastUID:     state.LastUID,
	}

	startUID := imap.UID(state.LastUID) + 1
	if rescan {
		baseline.LastUID = 0
		startUID = 1
	}

	fetchOptions := &imap.FetchOptions{
		UID: true,
		// Peek: true → BODY.PEEK[], das \Seen-Flag bleibt unberührt
		// (IMPLEMENTIERUNG.md Abschnitt 7.1, Schritt 4).
		BodySection: []*imap.FetchItemBodySection{{Peek: true}},
	}
	cmd := a.client.Fetch(imap.UIDSet{{Start: startUID, Stop: 0}}, fetchOptions)

	// Ein abgebrochener Kontext schließt die Verbindung — das lässt ein
	// gerade blockierendes cmd.Next() (Netzwerk-I/O, nicht selbst
	// context-fähig) mit einem Fehler zurückkehren, statt auf ewig zu
	// blockieren. stopWatch beendet den Watcher, sobald der Iterator
	// fertig konsumiert wurde.
	stopWatch := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = a.client.Close()
		case <-stopWatch:
		}
	}()

	seq := func(yield func(sync.RawMessage, error) bool) {
		defer close(stopWatch)
		defer func() {
			if err := cmd.Close(); err != nil && ctx.Err() == nil {
				yield(sync.RawMessage{}, fmt.Errorf("fetch konnte nicht sauber abgeschlossen werden: %w", err))
			}
		}()

		for {
			if err := ctx.Err(); err != nil {
				yield(sync.RawMessage{}, err)
				return
			}

			msgData := cmd.Next()
			if msgData == nil {
				return
			}

			buf, err := msgData.Collect()
			if err != nil {
				if !yield(sync.RawMessage{}, fmt.Errorf("nachricht konnte nicht gelesen werden: %w", err)) {
					return
				}
				continue
			}

			var body []byte
			if len(buf.BodySection) > 0 {
				body = buf.BodySection[0].Bytes
			}

			if !yield(sync.RawMessage{UID: uint32(buf.UID), Data: body}, nil) {
				return
			}
		}
	}

	return seq, baseline, nil
}
