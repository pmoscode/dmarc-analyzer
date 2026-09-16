// Package settings enthält die Kontoverwaltung: ein wiederverwendbares
// Kontoformular (auch vom Ersteinrichtungs-Assistenten genutzt) und die
// Kontoübersicht (IMPLEMENTIERUNG.md Abschnitt 10.1, "Einstellungen").
package settings

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
)

const (
	defaultIMAPPort     = 993
	mailboxProbeAccount = account.AccountID("mailbox-lookup-probe")
)

// AccountForm ist das wiederverwendbare Kontoformular — genutzt sowohl in
// den Einstellungen (Konto hinzufügen) als auch im
// Ersteinrichtungs-Assistenten (internal/ui/onboarding).
type AccountForm struct {
	widget.BaseWidget

	displayName *widget.Entry
	host        *widget.Entry
	port        *widget.Entry
	username    *widget.Entry
	mailbox     *widget.SelectEntry
	listButton  *widget.Button
	useTLS      *widget.Check
	password    *widget.Entry

	window fyne.Window
	// runBackground: siehe internal/ui/settings.View (AGENTS.md).
	runBackground func(f func())

	// ListMailboxes probt die aktuell im Formular eingetragenen
	// Verbindungsdaten und liefert die auf dem Server vorhandenen
	// Postfächer/Ordner — vom Aufrufer (settings.View / onboarding.Wizard)
	// verdrahtet, i. d. R. direkt manageaccount.UseCase.ListMailboxes. nil
	// (z. B. in Tests, die den Knopf nicht brauchen) lässt "Ordner
	// auflisten" wirkungslos.
	ListMailboxes func(ctx context.Context, acc account.MailAccount, secret account.Secret) ([]string, error)

	form *widget.Form
}

// NewAccountForm erzeugt ein leeres Formular mit sinnvollen Vorgaben
// (Port 993, verschlüsselte Verbindung an, Postfach INBOX). window wird
// für Rückmeldungen von "Ordner auflisten" gebraucht (Fehlerdialoge).
func NewAccountForm(window fyne.Window) *AccountForm {
	f := &AccountForm{
		displayName:   widget.NewEntry(),
		host:          widget.NewEntry(),
		port:          widget.NewEntry(),
		username:      widget.NewEntry(),
		useTLS:        widget.NewCheck(i18n.AccountFieldUseTLS, nil),
		password:      widget.NewPasswordEntry(),
		window:        window,
		runBackground: func(f func()) { go f() },
	}
	f.port.SetText(strconv.Itoa(defaultIMAPPort))
	f.useTLS.SetChecked(true)
	f.host.PlaceHolder = "imap.example.com"
	f.username.PlaceHolder = "name@example.com"

	// Postfach als SelectEntry: frei eintippbar wie bisher (z. B.
	// "INBOX/DMARC" von Hand), zusätzlich mit Dropdown-Optionen
	// befüllbar, sobald "Ordner auflisten" erfolgreich war.
	f.mailbox = widget.NewSelectEntry(nil)
	f.mailbox.SetText("INBOX")
	f.listButton = widget.NewButton(i18n.AccountListMailboxes, f.listMailboxes)
	mailboxRow := container.NewBorder(nil, nil, nil, f.listButton, f.mailbox)

	mailboxItem := widget.NewFormItem(i18n.AccountFieldMailbox, mailboxRow)
	mailboxItem.HintText = i18n.AccountHelpMailbox
	passwordItem := widget.NewFormItem(i18n.AccountFieldPassword, f.password)
	passwordItem.HintText = i18n.AccountHelpPassword

	f.form = widget.NewForm(
		widget.NewFormItem(i18n.AccountFieldDisplayName, f.displayName),
		widget.NewFormItem(i18n.AccountFieldHost, f.host),
		widget.NewFormItem(i18n.AccountFieldPort, f.port),
		widget.NewFormItem(i18n.AccountFieldUsername, f.username),
		mailboxItem,
		widget.NewFormItem("", f.useTLS),
		passwordItem,
	)

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

// listMailboxes probt die aktuell eingetragenen Verbindungsdaten (Host,
// Port, Benutzername, Passwort — das Postfach-Feld selbst wird bewusst
// nicht vorausgesetzt, es soll ja gerade erst befüllt werden) und
// befüllt bei Erfolg die Dropdown-Optionen des Postfach-Felds
// (UMSETZUNGSPLAN.md: DMARC-Berichte können in einem Unterordner statt
// im Wurzelpostfach liegen).
func (f *AccountForm) listMailboxes() {
	if f.ListMailboxes == nil {
		return
	}
	if strings.TrimSpace(f.host.Text) == "" || strings.TrimSpace(f.username.Text) == "" || f.password.Text == "" {
		dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.AccountErrorNeedsConnectionDetails, f.window)
		return
	}
	port, err := f.parsedPort()
	if err != nil {
		dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.AccountErrorPortInvalid, f.window)
		return
	}

	acc, err := account.NewMailAccount(
		mailboxProbeAccount, "", strings.TrimSpace(f.host.Text), port,
		strings.TrimSpace(f.username.Text), "", f.useTLS.Checked, time.Now(),
	)
	if err != nil {
		dialog.ShowInformation(i18n.ErrorGenericTitle, err.Error(), f.window)
		return
	}
	secret := f.Secret()

	f.listButton.Disable()
	f.runBackground(func() {
		mailboxes, listErr := f.ListMailboxes(context.Background(), *acc, secret)
		fyne.Do(func() {
			f.listButton.Enable()
			if listErr != nil {
				dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.AccountErrorListMailboxesFailed, f.window)
				return
			}
			f.mailbox.SetOptions(mailboxes)
		})
	})
}

// SetRunBackgroundForTest ersetzt die interne Hintergrund-Ausführung —
// für Tests aus anderen Paketen, die AccountForm einbetten. Nicht für
// Produktivcode gedacht, siehe AGENTS.md, Abschnitt zu Fyne-Tests.
func (f *AccountForm) SetRunBackgroundForTest(run func(func())) {
	f.runBackground = run
}

// validationError ist eine bereits benutzerfreundlich formulierte
// Fehlermeldung — im Unterschied zu Fehlern aus tieferen Schichten
// (Domäne, Infra), die noch in Klartext übersetzt werden müssen.
type validationError struct{ message string }

func (e validationError) Error() string { return e.message }
