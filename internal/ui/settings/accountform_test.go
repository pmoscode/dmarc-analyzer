package settings

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validForm() *AccountForm {
	f := NewAccountForm()
	f.host.SetText("imap.example.com")
	f.username.SetText("user@example.com")
	f.password.SetText("app-passwort")
	return f
}

func TestAccountForm_DefaultValues(t *testing.T) {
	f := NewAccountForm()
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
