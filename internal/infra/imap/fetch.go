package imap

import (
	"context"
	"fmt"
	"iter"

	"github.com/emersion/go-imap/v2"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// FetchNew selects the mailbox (EXAMINE, not SELECT — read-only, see
// Adapter documentation) and returns an iterator over all messages from
// state.LastUID+1 onward. If UIDValidity differs from state (or
// state.UIDValidity is 0, i.e. a first sync), it rescans from UID 1
// instead — duplicates are caught by the UNIQUE index in persistence
// (IMPLEMENTIERUNG.md section 7.1).
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
		// Peek: true → BODY.PEEK[], the \Seen flag stays untouched
		// (IMPLEMENTIERUNG.md section 7.1, step 4).
		BodySection: []*imap.FetchItemBodySection{{Peek: true}},
	}
	cmd := a.client.Fetch(imap.UIDSet{{Start: startUID, Stop: 0}}, fetchOptions)

	// A cancelled context closes the connection — this lets a currently
	// blocking cmd.Next() (network I/O, not context-aware itself) return
	// with an error instead of blocking forever. stopWatch ends the
	// watcher once the iterator has been fully consumed.
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
				yield(sync.RawMessage{}, fmt.Errorf("failed to cleanly complete fetch: %w", err))
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
				if !yield(sync.RawMessage{}, fmt.Errorf("failed to read message: %w", err)) {
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
