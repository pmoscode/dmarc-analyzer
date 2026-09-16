// Package settings enthält die Kontoverwaltung: ein wiederverwendbares
// Kontoformular (auch vom Ersteinrichtungs-Assistenten genutzt) und die
// Kontoübersicht (IMPLEMENTIERUNG.md Abschnitt 10.1, "Einstellungen").
package settings

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
)

const defaultIMAPPort = 993

// AccountForm ist das wiederverwendbare Kontoformular — genutzt sowohl in
// den Einstellungen (Konto hinzufügen) als auch im
// Ersteinrichtungs-Assistenten (internal/ui/onboarding).
type AccountForm struct {
	widget.BaseWidget

	displayName *widget.Entry
	host        *widget.Entry
	port        *widget.Entry
	username    *widget.Entry
	mailbox     *widget.Entry
	useTLS      *widget.Check
	password    *widget.Entry

	form *widget.Form
}

// NewAccountForm erzeugt ein leeres Formular mit sinnvollen Vorgaben
// (Port 993, verschlüsselte Verbindung an, Postfach INBOX).
func NewAccountForm() *AccountForm {
	f := &AccountForm{
		displayName: widget.NewEntry(),
		host:        widget.NewEntry(),
		port:        widget.NewEntry(),
		username:    widget.NewEntry(),
		mailbox:     widget.NewEntry(),
		useTLS:      widget.NewCheck(i18n.AccountFieldUseTLS, nil),
		password:    widget.NewPasswordEntry(),
	}
	f.port.SetText(strconv.Itoa(defaultIMAPPort))
	f.mailbox.SetText("INBOX")
	f.useTLS.SetChecked(true)
	f.host.PlaceHolder = "imap.example.com"
	f.username.PlaceHolder = "name@example.com"

	f.form = widget.NewForm(
		widget.NewFormItem(i18n.AccountFieldDisplayName, f.displayName),
		widget.NewFormItem(i18n.AccountFieldHost, f.host),
		widget.NewFormItem(i18n.AccountFieldPort, f.port),
		widget.NewFormItem(i18n.AccountFieldUsername, f.username),
		widget.NewFormItem(i18n.AccountFieldMailbox, f.mailbox),
		widget.NewFormItem("", f.useTLS),
		widget.NewFormItem(i18n.AccountFieldPassword, f.password),
	)
	f.form.Items[len(f.form.Items)-1].HintText = i18n.AccountHelpPassword

	f.ExtendBaseWidget(f)
	return f
}

// CreateRenderer erfüllt fyne.Widget.
func (f *AccountForm) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(f.form)
}

// Validate prüft die Pflichtfelder und liefert eine Klartext-Fehlermeldung
// (kein technischer Validierungsfehler) für das erste Problem, oder nil.
func (f *AccountForm) Validate() error {
	if strings.TrimSpace(f.host.Text) == "" {
		return validationError{i18n.AccountErrorHostRequired}
	}
	if strings.TrimSpace(f.username.Text) == "" {
		return validationError{i18n.AccountErrorUsernameRequired}
	}
	if _, err := f.parsedPort(); err != nil {
		return validationError{i18n.AccountErrorPortInvalid}
	}
	if f.password.Text == "" {
		return validationError{i18n.AccountErrorPasswordRequired}
	}
	return nil
}

func (f *AccountForm) parsedPort() (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(f.port.Text))
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %q ist ungültig", f.port.Text)
	}
	return port, nil
}

// BuildAccount erstellt aus den Formularwerten ein MailAccount. Ruft
// zuvor Validate() nicht selbst auf — der Aufrufer entscheidet, wann
// validiert wird (z. B. erst beim Absenden, nicht bei jedem Tastendruck).
func (f *AccountForm) BuildAccount(id account.AccountID) (*account.MailAccount, error) {
	port, err := f.parsedPort()
	if err != nil {
		return nil, validationError{i18n.AccountErrorPortInvalid}
	}

	acc, err := account.NewMailAccount(
		id,
		strings.TrimSpace(f.displayName.Text),
		strings.TrimSpace(f.host.Text),
		port,
		strings.TrimSpace(f.username.Text),
		strings.TrimSpace(f.mailbox.Text),
		f.useTLS.Checked,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

// Secret liefert das eingegebene Passwort als account.Secret.
func (f *AccountForm) Secret() account.Secret {
	return account.NewSecretFromString(f.password.Text)
}

// validationError ist eine bereits benutzerfreundlich formulierte
// Fehlermeldung — im Unterschied zu Fehlern aus tieferen Schichten
// (Domäne, Infra), die noch in Klartext übersetzt werden müssen.
type validationError struct{ message string }

func (e validationError) Error() string { return e.message }
