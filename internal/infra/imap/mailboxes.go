package imap

import (
	"context"
	"fmt"
	"sort"

	"github.com/emersion/go-imap/v2"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

var _ sync.MailboxLister = (*Adapter)(nil)

// ListMailboxes implementiert sync.MailboxLister: listet per IMAP LIST
// alle auf dem Server vorhandenen Postfächer/Ordner, ausgenommen solche
// mit dem Attribut \Noselect (reine Hierarchie-Knoten, die selbst keine
// Nachrichten enthalten können, siehe RFC 9051 Abschnitt 7.3.1) —
// niemand kann DMARC-Berichte in einem solchen "Ordner" ablegen, ihn in
// den Picker aufzunehmen wäre nur verwirrend. Ergebnis alphabetisch
// sortiert für eine stabile, vorhersehbare Anzeige.
func (a *Adapter) ListMailboxes(ctx context.Context) ([]string, error) {
	if a.client == nil {
		return nil, errNotConnected
	}

	var mailboxes []string
	err := runCtx(ctx, a.client, func() error {
		data, listErr := a.client.List("", "*", nil).Collect()
		if listErr != nil {
			return listErr
		}
		for _, d := range data {
			if hasNoSelectAttr(d.Attrs) {
				continue
			}
			mailboxes = append(mailboxes, d.Mailbox)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("postfächer konnten nicht aufgelistet werden: %w", err)
	}

	sort.Strings(mailboxes)
	return mailboxes, nil
}

func hasNoSelectAttr(attrs []imap.MailboxAttr) bool {
	for _, a := range attrs {
		if a == imap.MailboxAttrNoSelect {
			return true
		}
	}
	return false
}
