package imap_test

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

const (
	testUsername = "dmarc-test"
	testPassword = "test-password-123"
	testMailbox  = "INBOX"
)

// literalBytes implements imap.LiteralReader (io.Reader + Size() int64) for
// a fixed []byte — the library doesn't provide a ready-made helper for
// this, but User.Append requires exactly this interface.
type literalBytes struct {
	*bytes.Reader
	size int64
}

func newLiteralBytes(data []byte) literalBytes {
	return literalBytes{Reader: bytes.NewReader(data), size: int64(len(data))}
}

func (l literalBytes) Size() int64 {
	return l.size
}

// testServer bundles the in-process IMAP test server with the user that
// Adapter tests run against.
type testServer struct {
	addr string
	mem  *imapmemserver.Server
	user *imapmemserver.User
}

// newTestServer starts an in-process IMAP server (imapmemserver, go-imap's
// own test server component) on a local ephemeral port and creates a user
// with an empty INBOX mailbox. TLS deliberately not configured — the test
// checks protocol logic, not transport security (see the
// account.MailAccount.UseTLS comment).
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}

	ts := newTestServerOnAddr(t, ln.Addr().String())
	ts.serve(t, ln)
	return ts
}

// newTestServerOnAddr creates a test server with user and mailbox, but does
// not yet start a listener — for tests that want to control the timing of
// the actual connection setup themselves (see start()).
func newTestServerOnAddr(t *testing.T, addr string) *testServer {
	t.Helper()

	memServer := imapmemserver.New()
	user := imapmemserver.NewUser(testUsername, testPassword)
	memServer.AddUser(user)
	if err := user.Create(testMailbox, nil); err != nil {
		t.Fatalf("failed to create test mailbox: %v", err)
	}

	return &testServer{addr: addr, mem: memServer, user: user}
}

// start binds the listener on ts.addr and serves it. newTestServer already
// handles this for the normal case; useful on its own to deliberately let a
// connection attempt succeed only after a delay (retry test).
func (ts *testServer) start(t *testing.T) {
	t.Helper()

	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", ts.addr)
	if err != nil {
		t.Fatalf("failed to start test listener on %q: %v", ts.addr, err)
	}
	ts.serve(t, ln)
}

func (ts *testServer) serve(t *testing.T, ln net.Listener) {
	t.Helper()

	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return ts.mem.NewSession(), nil, nil
		},
		InsecureAuth: true, // test runs without TLS
		Caps:         imap.CapSet{imap.CapIMAP4rev1: struct{}{}},
	})

	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
}

// account returns a MailAccount pointing at this test server.
func (ts *testServer) account(t *testing.T) account.MailAccount {
	t.Helper()

	host, portStr, err := net.SplitHostPort(ts.addr)
	if err != nil {
		t.Fatalf("failed to split test address %q: %v", ts.addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse test port %q: %v", portStr, err)
	}

	acc, err := account.NewMailAccount("test-account", "Test", host, port, testUsername, testMailbox, false, time.Now())
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}
	return *acc
}

// appendMessage inserts a test message with a sequential UID.
//
// Deliberately passes &imap.AppendOptions{} instead of nil: imapmemserver
// v2.0.0-beta.8 dereferences options without a nil check (appendBytes in
// mailbox.go reads options.Time/.Flags unchecked) — passing nil as Options
// panics the server. A bug in the beta library, worked around here rather
// than waited out.
func (ts *testServer) appendMessage(t *testing.T, raw string) {
	t.Helper()

	if _, err := ts.user.Append(testMailbox, newLiteralBytes([]byte(raw)), &imap.AppendOptions{}); err != nil {
		t.Fatalf("failed to append test message: %v", err)
	}
}

// recreateMailbox deletes and recreates the mailbox — imapmemserver assigns
// a new, guaranteed-higher UIDVALIDITY in the process (see user.go:
// prevUidValidity is incremented on every Create). Simulates the
// UIDVALIDITY-change case described in IMPLEMENTIERUNG.md section 7.1
// (e.g. after restoring the mailbox).
func (ts *testServer) recreateMailbox(t *testing.T) {
	t.Helper()

	if err := ts.user.Delete(testMailbox); err != nil {
		t.Fatalf("failed to delete test mailbox: %v", err)
	}
	if err := ts.user.Create(testMailbox, nil); err != nil {
		t.Fatalf("failed to recreate test mailbox: %v", err)
	}
}

// testMessage builds a minimal but valid RFC822 message.
func testMessage(subject, body string) string {
	return "From: sender@example.com\r\n" +
		"To: " + testUsername + "@example.com\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n"
}
