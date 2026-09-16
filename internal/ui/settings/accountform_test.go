package settings

import (
	"context"
	"errors"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

func validForm() *AccountForm {
	f := NewAccountForm(nil)
	f.host.SetText("imap.example.com")
	f.username.SetText("user@example.com")
	f.password.SetText("app-passwort")
	return f
}

func TestAccountForm_DefaultValues(t *testing.T) {
	f := NewAccountForm(nil)
	require.Equal(t, "993", f.port.Text)
	require.Equal(t, "INBOX", f.mailbox.Text)
	require.True(t, f.useTLS.Checked)
}

func TestAccountForm_Validate_MissingHost(t *testing.T) {
	f := validForm()
	f.host.SetText("")
	require.Error(t, f.Validate())
}

func TestAccountForm_Validate_MissingUsername(t *testing.T) {
	f := validForm()
	f.username.SetText("")
	require.Error(t, f.Validate())
}

func TestAccountForm_Validate_MissingPassword(t *testing.T) {
	f := validForm()
	f.password.SetText("")
	require.Error(t, f.Validate())
}

func TestAccountForm_Validate_InvalidPort(t *testing.T) {
	f := validForm()
	f.port.SetText("not-a-number")
	require.Error(t, f.Validate())

	f.port.SetText("70000")
	require.Error(t, f.Validate())

	f.port.SetText("0")
	require.Error(t, f.Validate())
}

func TestAccountForm_Validate_AllFieldsPresent_NoError(t *testing.T) {
	f := validForm()
	require.NoError(t, f.Validate())
}

func TestAccountForm_BuildAccount_UsesEnteredValues(t *testing.T) {
	f := validForm()
	f.displayName.SetText("Mein Postfach")
	f.port.SetText("143")
	f.mailbox.SetText("Archive")
	f.useTLS.SetChecked(false)

	acc, err := f.BuildAccount("acc-1")
	require.NoError(t, err)
	require.Equal(t, "Mein Postfach", acc.DisplayName)
	require.Equal(t, "imap.example.com", acc.Host)
	require.Equal(t, 143, acc.Port)
	require.Equal(t, "user@example.com", acc.Username)
	require.Equal(t, "Archive", acc.Mailbox)
	require.False(t, acc.UseTLS)
}

func TestAccountForm_BuildAccount_InvalidPort_ReturnsError(t *testing.T) {
	f := validForm()
	f.port.SetText("kaputt")

	_, err := f.BuildAccount("acc-1")
	require.Error(t, err)
}

func TestAccountForm_Secret_ReturnsEnteredPassword(t *testing.T) {
	f := validForm()
	require.Equal(t, "app-passwort", string(f.Secret().Expose()))
}

func newSyncTestFormWithWindow(t *testing.T) (*AccountForm, func()) {
	t.Helper()
	w := test.NewWindow(nil)
	f := NewAccountForm(w)
	f.runBackground = func(fn func()) { fn() }
	f.host.SetText("imap.example.com")
	f.username.SetText("user@example.com")
	f.password.SetText("app-passwort")
	w.SetContent(f)
	return f, w.Close
}

func TestAccountForm_ListMailboxes_Success_PopulatesMailboxOptions(t *testing.T) {
	f, closeWin := newSyncTestFormWithWindow(t)
	defer closeWin()

	var gotAcc account.MailAccount
	f.ListMailboxes = func(_ context.Context, acc account.MailAccount, _ account.Secret) ([]string, error) {
		gotAcc = acc
		return []string{"INBOX", "INBOX/DMARC"}, nil
	}

	require.NotPanics(t, f.listMailboxes)

	require.Equal(t, "imap.example.com", gotAcc.Host)
	require.Equal(t, "user@example.com", gotAcc.Username)
}

func TestAccountForm_ListMailboxes_MissingConnectionDetails_DoesNotCall(t *testing.T) {
	f, closeWin := newSyncTestFormWithWindow(t)
	defer closeWin()
	f.password.SetText("")

	called := false
	f.ListMailboxes = func(context.Context, account.MailAccount, account.Secret) ([]string, error) {
		called = true
		return nil, nil
	}

	require.NotPanics(t, f.listMailboxes)
	require.False(t, called, "ohne vollständige Verbindungsdaten darf nicht probeweise verbunden werden")
}

func TestAccountForm_ListMailboxes_Failure_DoesNotPanicOrPopulateOptions(t *testing.T) {
	f, closeWin := newSyncTestFormWithWindow(t)
	defer closeWin()

	called := false
	f.ListMailboxes = func(context.Context, account.MailAccount, account.Secret) ([]string, error) {
		called = true
		return nil, errors.New("verbindung fehlgeschlagen")
	}

	require.NotPanics(t, f.listMailboxes)
	require.True(t, called)
}

func TestAccountForm_ListMailboxes_NilCallback_DoesNothing(t *testing.T) {
	f, closeWin := newSyncTestFormWithWindow(t)
	defer closeWin()

	require.NotPanics(t, f.listMailboxes)
}
