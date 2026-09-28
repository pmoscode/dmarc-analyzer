package imap_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	imapadapter "github.com/pmoscode/dmarc-analyzer/internal/infra/imap"
)

func TestConnect_Success(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	acc := ts.account(t)

	a := imapadapter.NewAdapter()
	secret := account.NewSecretFromString(testPassword)

	err := a.Connect(context.Background(), acc, secret)
	require.NoError(t, err)
	require.NoError(t, a.Close())
}

func TestConnect_WrongPassword_FailsWithoutHanging(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	acc := ts.account(t)

	a := imapadapter.NewAdapter()
	secret := account.NewSecretFromString("definitely-wrong")

	start := time.Now()
	err := a.Connect(context.Background(), acc, secret)
	elapsed := time.Since(start)

	require.Error(t, err)
	// A login error is not retried with backoff (see the adapter.go
	// comment) — must fail noticeably faster than three attempts with
	// backoff would take (200ms+400ms of wait time alone).
	require.Less(t, elapsed, 150*time.Millisecond,
		"a login error must not be retried (a wrong password doesn't become correct via retry)")
}

func TestFetchNew_FirstSync_ReturnsAllMessagesAndSetsUIDValidity(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Report 1", "Content 1"))
	ts.appendMessage(t, testMessage("Report 2", "Content 2"))

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	seq, baseline, err := a.FetchNew(context.Background(), sync.State{Mailbox: testMailbox})
	require.NoError(t, err)
	require.NotZero(t, baseline.UIDValidity)
	require.Zero(t, baseline.LastUID, "there is no progress yet on the first sync")

	var messages []sync.RawMessage
	for msg, err := range seq {
		require.NoError(t, err)
		messages = append(messages, msg)
	}

	require.Len(t, messages, 2)
	require.Contains(t, string(messages[0].Data), "Report 1")
	require.Contains(t, string(messages[1].Data), "Report 2")
	require.NotZero(t, messages[0].UID)
	require.Greater(t, messages[1].UID, messages[0].UID)
}

func TestFetchNew_SecondSync_ReturnsNoNewMessages(t *testing.T) {
	// The core case required by UMSETZUNGSPLAN.md AP 3: "a second sync run
	// right after the first fetches zero messages."
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Report 1", "Content 1"))

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	ctx := context.Background()
	state := sync.State{Mailbox: testMailbox}

	seq, state, err := a.FetchNew(ctx, state)
	require.NoError(t, err)
	var lastUID uint32
	for msg, err := range seq {
		require.NoError(t, err)
		lastUID = msg.UID
	}
	require.NotZero(t, lastUID)
	state.LastUID = lastUID // as the caller (SyncReports, AP4) would do

	seq, _, err = a.FetchNew(ctx, state)
	require.NoError(t, err)

	var second []sync.RawMessage
	for msg, err := range seq {
		require.NoError(t, err)
		second = append(second, msg)
	}
	require.Empty(t, second, "a second sync with an advanced state must not return anything new")
}

func TestFetchNew_OnlyFetchesMessagesAfterLastUID(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Old", "before the state"))

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	ctx := context.Background()
	seq, state, err := a.FetchNew(ctx, sync.State{Mailbox: testMailbox})
	require.NoError(t, err)
	for msg := range seq {
		state.LastUID = msg.UID
	}

	ts.appendMessage(t, testMessage("New", "after the state"))

	seq, _, err = a.FetchNew(ctx, state)
	require.NoError(t, err)

	var messages []sync.RawMessage
	for msg, err := range seq {
		require.NoError(t, err)
		messages = append(messages, msg)
	}
	require.Len(t, messages, 1)
	require.Contains(t, string(messages[0].Data), "New")
}

func TestFetchNew_UIDValidityChange_TriggersFullRescan(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Before the change", "..."))

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	ctx := context.Background()
	seq, state, err := a.FetchNew(ctx, sync.State{Mailbox: testMailbox})
	require.NoError(t, err)
	var firstRun []sync.RawMessage
	for msg := range seq {
		firstRun = append(firstRun, msg)
		state.LastUID = msg.UID
	}
	require.Len(t, firstRun, 1)
	oldUIDValidity := state.UIDValidity

	// Recreate the mailbox → new UIDVALIDITY, as would happen after
	// restoring the mailbox on the server.
	ts.recreateMailbox(t)
	ts.appendMessage(t, testMessage("After the change", "..."))

	seq, newBaseline, err := a.FetchNew(ctx, state)
	require.NoError(t, err)
	require.NotEqual(t, oldUIDValidity, newBaseline.UIDValidity)

	var secondRun []sync.RawMessage
	for msg, err := range seq {
		require.NoError(t, err)
		secondRun = append(secondRun, msg)
	}
	// Full rescan: the (only) message in the new mailbox is delivered
	// again, even if its UID happened to be <= the old LastUID. Duplicates
	// are caught by the UNIQUE index in persistence (AP 2).
	require.Len(t, secondRun, 1)
	require.Contains(t, string(secondRun[0].Data), "After the change")
}

func TestFetchNew_UsesPeek_DoesNotSetSeenFlag(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Stay untouched", "..."))

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	ctx := context.Background()
	seq, _, err := a.FetchNew(ctx, sync.State{Mailbox: testMailbox})
	require.NoError(t, err)
	for range seq {
		// just consume
	}

	// Check the flags with a second, independent client — BODY.PEEK[] must
	// not have set \Seen (IMPLEMENTIERUNG.md section 7.1).
	client, err := imapclient.DialInsecure(ts.addr, nil)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
	require.NoError(t, client.Login(testUsername, testPassword).Wait())
	_, err = client.Select(testMailbox, nil).Wait()
	require.NoError(t, err)

	msgs, err := client.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{Flags: true}).Collect()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.NotContains(t, msgs[0].Flags, imap.FlagSeen)
}

func TestFetchNew_NotConnected_ReturnsError(t *testing.T) {
	t.Parallel()

	a := imapadapter.NewAdapter()
	_, _, err := a.FetchNew(context.Background(), sync.State{Mailbox: testMailbox})
	require.Error(t, err)
}

func TestFetchNew_ContextAlreadyCancelled_FailsFast(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// A context already cancelled before the SELECT makes FetchNew itself
	// fail (via runCtx during the SELECT request) — no reason to first
	// return an iterator whose first step would only report the
	// cancellation anyway.
	_, _, err := a.FetchNew(ctx, sync.State{Mailbox: testMailbox})
	require.ErrorIs(t, err, context.Canceled)
}

func TestFetchNew_ContextCancelledDuringIteration_StopsPromptly(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Doesn't matter", "..."))

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	ctx, cancel := context.WithCancel(context.Background())

	// SELECT runs with a valid context; the cancellation only happens
	// between obtaining the iterator and consuming it — this exercises the
	// cancellation path within the iteration (fetch.go: ctx.Err() check at
	// the start of every loop iteration, or the connection being closed by
	// the watcher if the server is still mid-response).
	seq, _, err := a.FetchNew(ctx, sync.State{Mailbox: testMailbox})
	require.NoError(t, err)
	cancel()

	var gotErr error
	for _, iterErr := range seq {
		if iterErr != nil {
			gotErr = iterErr
			break
		}
	}
	require.ErrorIs(t, gotErr, context.Canceled)
}

func TestConnect_RetriesTransientDialFailure(t *testing.T) {
	t.Parallel()

	// Reserve an ephemeral port, release it right away — on the first
	// Connect attempt the operating system refuses the connection
	// (ECONNREFUSED), that's the transient error to be retried.
	probe, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := probe.Addr().String()
	require.NoError(t, probe.Close())

	ts := newTestServerOnAddr(t, addr)

	a := imapadapter.NewAdapter()

	done := make(chan error, 1)
	go func() {
		done <- a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword))
	}()

	// The server only starts while retry() is already waiting between the
	// 1st and 2nd attempt (backoffDelays[0] = 200ms) — the 2nd or 3rd
	// attempt must reach the then-running server.
	time.Sleep(80 * time.Millisecond)
	ts.start(t)

	select {
	case err := <-done:
		require.NoError(t, err, "Connect should still succeed via retry after a transient error")
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not return — retry is hanging or not kicking in")
	}
	require.NoError(t, a.Close())
}
