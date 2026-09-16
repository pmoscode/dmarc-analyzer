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
	testPassword = "test-passwort-123"
	testMailbox  = "INBOX"
)

// literalBytes implementiert imap.LiteralReader (io.Reader + Size() int64)
// für ein festes []byte — die Bibliothek liefert dafür keinen fertigen
// Helfer, User.Append verlangt aber genau dieses Interface.
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

// testServer bündelt den in-process IMAP-Testserver mit dem User, gegen
// den Adapter-Tests laufen.
type testServer struct {
	addr string
	mem  *imapmemserver.Server
	user *imapmemserver.User
}

// newTestServer startet einen in-process IMAP-Server (imapmemserver, die
// go-imap-eigene Testserver-Komponente) auf einem lokalen Ephemeral-Port
// und legt einen Benutzer mit leerem INBOX-Postfach an. TLS bewusst nicht
// konfiguriert — der Test prüft Protokolllogik, nicht Transportsicherheit
// (siehe account.MailAccount.UseTLS-Kommentar).
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("test-listener konnte nicht gestartet werden: %v", err)
	}

	ts := newTestServerOnAddr(t, ln.Addr().String())
	ts.serve(t, ln)
	return ts
}

// newTestServerOnAddr erzeugt einen Testserver mit Benutzer und Postfach,
// startet aber noch keinen Listener — für Tests, die den Zeitpunkt des
// tatsächlichen Verbindungsaufbaus selbst steuern wollen (siehe start()).
func newTestServerOnAddr(t *testing.T, addr string) *testServer {
	t.Helper()

	memServer := imapmemserver.New()
	user := imapmemserver.NewUser(testUsername, testPassword)
	memServer.AddUser(user)
	if err := user.Create(testMailbox, nil); err != nil {
		t.Fatalf("testpostfach konnte nicht angelegt werden: %v", err)
	}

	return &testServer{addr: addr, mem: memServer, user: user}
}

// start bindet den Listener auf ts.addr und bedient ihn. Für den
// Normalfall übernimmt newTestServer das bereits; eigenständig nützlich,
// um einen Verbindungsaufbau gezielt erst verspätet erfolgreich werden zu
// lassen (Retry-Test).
func (ts *testServer) start(t *testing.T) {
	t.Helper()

	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", ts.addr)
	if err != nil {
		t.Fatalf("test-listener konnte nicht auf %q gestartet werden: %v", ts.addr, err)
	}
	ts.serve(t, ln)
}

func (ts *testServer) serve(t *testing.T, ln net.Listener) {
	t.Helper()

	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return ts.mem.NewSession(), nil, nil
		},
		InsecureAuth: true, // Test läuft ohne TLS
		Caps:         imap.CapSet{imap.CapIMAP4rev1: struct{}{}},
	})

	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
}

// account liefert ein MailAccount, das auf diesen Testserver zeigt.
func (ts *testServer) account(t *testing.T) account.MailAccount {
	t.Helper()

	host, portStr, err := net.SplitHostPort(ts.addr)
	if err != nil {
		t.Fatalf("test-adresse %q konnte nicht zerlegt werden: %v", ts.addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("test-port %q konnte nicht geparst werden: %v", portStr, err)
	}

	acc, err := account.NewMailAccount("test-account", "Test", host, port, testUsername, testMailbox, false, time.Now())
	if err != nil {
		t.Fatalf("test-account konnte nicht erzeugt werden: %v", err)
	}
	return *acc
}

// appendMessage fügt eine Testnachricht mit fortlaufender UID ein.
//
// Übergibt bewusst &imap.AppendOptions{} statt nil: imapmemserver v2.0.0-
// beta.8 dereferenziert options ungeprüft (appendBytes in mailbox.go liest
// options.Time/.Flags ohne Nil-Check) — mit nil als Options panickt der
// Server. Bug der Beta-Bibliothek, hier umgangen statt gewartet.
func (ts *testServer) appendMessage(t *testing.T, raw string) {
	t.Helper()

	if _, err := ts.user.Append(testMailbox, newLiteralBytes([]byte(raw)), &imap.AppendOptions{}); err != nil {
		t.Fatalf("testnachricht konnte nicht angehängt werden: %v", err)
	}
}

// recreateMailbox löscht und legt das Postfach neu an — imapmemserver
// vergibt dabei eine neue, garantiert höhere UIDVALIDITY
// (siehe user.go: prevUidValidity wird bei jedem Create hochgezählt).
// Simuliert den in IMPLEMENTIERUNG.md Abschnitt 7.1 beschriebenen Fall
// eines UIDVALIDITY-Wechsels (z. B. nach Wiederherstellen des Postfachs).
func (ts *testServer) recreateMailbox(t *testing.T) {
	t.Helper()

	if err := ts.user.Delete(testMailbox); err != nil {
		t.Fatalf("testpostfach konnte nicht gelöscht werden: %v", err)
	}
	if err := ts.user.Create(testMailbox, nil); err != nil {
		t.Fatalf("testpostfach konnte nicht neu angelegt werden: %v", err)
	}
}

// testMessage baut eine minimale, aber gültige RFC822-Nachricht.
func testMessage(subject, body string) string {
	return "From: sender@example.com\r\n" +
		"To: " + testUsername + "@example.com\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n"
}
