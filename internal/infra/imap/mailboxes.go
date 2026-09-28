package imap

import (
	"context"
	"fmt"
	"sort"

	"github.com/emersion/go-imap/v2"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

var _ sync.MailboxLister = (*Adapter)(nil)

// ListMailboxes implements sync.MailboxLister: lists all mailboxes/folders
// present on the server via IMAP LIST, excluding those with the \Noselect
// attribute (pure hierarchy nodes that cannot themselves contain messages,
// see RFC 9051 section 7.3.1) — nobody can put DMARC reports in such a
// "folder", so including it in the picker would only be confusing. Result
// sorted alphabetically for a stable, predictable display.
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
		return nil, fmt.Errorf("failed to list mailboxes: %w", err)
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
