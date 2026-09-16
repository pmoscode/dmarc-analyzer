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
	secret := account.NewSecretFromString("definitiv-falsch")

	start := time.Now()
	err := a.Connect(context.Background(), acc, secret)
	elapsed := time.Since(start)

	require.Error(t, err)
	// Ein Anmeldefehler wird nicht mit Backoff wiederholt (siehe
	// adapter.go-Kommentar) — muss deutlich schneller scheitern, als drei
	// Versuche mit Backoff bräuchten (200ms+400ms allein an Wartezeit).
	require.Less(t, elapsed, 150*time.Millisecond,
		"Anmeldefehler darf nicht wiederholt werden (falsches Passwort wird durch Retry nicht richtig)")
}

func TestFetchNew_FirstSync_ReturnsAllMessagesAndSetsUIDValidity(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Report 1", "Inhalt 1"))
	ts.appendMessage(t, testMessage("Report 2", "Inhalt 2"))

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	seq, baseline, err := a.FetchNew(context.Background(), sync.State{Mailbox: testMailbox})
	require.NoError(t, err)
	require.NotZero(t, baseline.UIDValidity)
	require.Zero(t, baseline.LastUID, "beim ersten Sync gibt es noch keinen Fortschritt")

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
	// Der in UMSETZUNGSPLAN.md AP 3 geforderte Kernfall: "Zweiter Sync-Lauf
	// direkt nach dem ersten holt null Nachrichten."
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Report 1", "Inhalt 1"))

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
	state.LastUID = lastUID // wie es der Aufrufer (SyncReports, AP4) tun würde

	seq, _, err = a.FetchNew(ctx, state)
	require.NoError(t, err)

	var second []sync.RawMessage
	for msg, err := range seq {
		require.NoError(t, err)
		second = append(second, msg)
	}
	require.Empty(t, second, "zweiter Sync mit fortgeschriebenem State darf nichts Neues liefern")
}

func TestFetchNew_OnlyFetchesMessagesAfterLastUID(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Alt", "vor dem State"))

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	ctx := context.Background()
	seq, state, err := a.FetchNew(ctx, sync.State{Mailbox: testMailbox})
	require.NoError(t, err)
	for msg := range seq {
		state.LastUID = msg.UID
	}

	ts.appendMessage(t, testMessage("Neu", "nach dem State"))

	seq, _, err = a.FetchNew(ctx, state)
	require.NoError(t, err)

	var messages []sync.RawMessage
	for msg, err := range seq {
		require.NoError(t, err)
		messages = append(messages, msg)
	}
	require.Len(t, messages, 1)
	require.Contains(t, string(messages[0].Data), "Neu")
}

func TestFetchNew_UIDValidityChange_TriggersFullRescan(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Vor dem Wechsel", "..."))

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

	// Postfach neu anlegen → neue UIDVALIDITY, wie nach einem
	// Wiederherstellen des Postfachs auf dem Server.
	ts.recreateMailbox(t)
	ts.appendMessage(t, testMessage("Nach dem Wechsel", "..."))

	seq, newBaseline, err := a.FetchNew(ctx, state)
	require.NoError(t, err)
	require.NotEqual(t, oldUIDValidity, newBaseline.UIDValidity)

	var secondRun []sync.RawMessage
	for msg, err := range seq {
		require.NoError(t, err)
		secondRun = append(secondRun, msg)
	}
	// Voller Rescan: die (einzige) Nachricht im neuen Postfach wird erneut
	// geliefert, auch wenn ihre UID zufällig <= der alten LastUID wäre.
	// Duplikate fängt der UNIQUE-Index in der Persistenz ab (AP 2).
	require.Len(t, secondRun, 1)
	require.Contains(t, string(secondRun[0].Data), "Nach dem Wechsel")
}

func TestFetchNew_UsesPeek_DoesNotSetSeenFlag(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Unberührt bleiben", "..."))

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	ctx := context.Background()
	seq, _, err := a.FetchNew(ctx, sync.State{Mailbox: testMailbox})
	require.NoError(t, err)
	for range seq {
		// nur konsumieren
	}

	// Mit einem zweiten, unabhängigen Client die Flags prüfen — BODY.PEEK[]
	// darf \Seen nicht gesetzt haben (IMPLEMENTIERUNG.md Abschnitt 7.1).
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

	// Bereits vor dem SELECT abgebrochener Kontext lässt FetchNew selbst
	// scheitern (über runCtx bei der SELECT-Anfrage) — kein Grund, erst
	// einen Iterator zurückzugeben, dessen erster Schritt sowieso nur den
	// Abbruch melden würde.
	_, _, err := a.FetchNew(ctx, sync.State{Mailbox: testMailbox})
	require.ErrorIs(t, err, context.Canceled)
}

func TestFetchNew_ContextCancelledDuringIteration_StopsPromptly(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.appendMessage(t, testMessage("Egal", "..."))

	a := imapadapter.NewAdapter()
	require.NoError(t, a.Connect(context.Background(), ts.account(t), account.NewSecretFromString(testPassword)))
	defer func() { _ = a.Close() }()

	ctx, cancel := context.WithCancel(context.Background())

	// SELECT läuft mit gültigem Kontext, der Abbruch passiert erst
	// zwischen dem Erhalt des Iterators und seinem Konsum — das übt den
	// Abbruchpfad innerhalb der Iteration aus (fetch.go: ctx.Err()-Prüfung
	// zu Beginn jeder Schleifenrunde bzw. Schließen der Verbindung durch
	// den Watcher, falls der Server noch mitten in der Antwort ist).
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

	// Ephemeral-Port reservieren, sofort wieder freigeben — beim ersten
	// Connect-Versuch lehnt das Betriebssystem die Verbindung ab
	// (ECONNREFUSED), das ist der zu wiederholende, transiente Fehler.
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

	// Der Server startet erst, während retry() bereits zwischen dem 1.
	// und 2. Versuch wartet (backoffDelays[0] = 200ms) — der 2. oder 3.
	// Versuch muss den dann laufenden Server erreichen.
	time.Sleep(80 * time.Millisecond)
	ts.start(t)

	select {
	case err := <-done:
		require.NoError(t, err, "Connect sollte nach einem transienten Fehler über Retry doch noch gelingen")
	case <-time.After(5 * time.Second):
		t.Fatal("Connect kehrte nicht zurück — Retry hängt oder greift nicht")
	}
	require.NoError(t, a.Close())
}
