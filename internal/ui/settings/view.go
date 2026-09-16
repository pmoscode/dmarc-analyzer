package settings

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"

	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/components"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
)

// View zeigt die Kontoübersicht: Liste, Hinzufügen, Testen, Löschen
// (IMPLEMENTIERUNG.md Abschnitt 10.1, "Einstellungen"). I/O läuft nie im
// UI-Thread — jeder Use-Case-Aufruf läuft über runBackground, die
// Aktualisierung der Widgets über fyne.Do() zurück (UMSETZUNGSPLAN.md AP 5).
type View struct {
	widget.BaseWidget

	accounts *manageaccount.UseCase
	window   fyne.Window
	// OnAccountsChanged wird nach jeder erfolgreichen Änderung (Anlegen,
	// Löschen) aufgerufen — der Aufrufer (z. B. das Hauptfenster) kann so
	// z. B. von der Ersteinrichtung in die normale Ansicht wechseln.
	OnAccountsChanged func()

	// runBackground führt f asynchron aus — im Produktivbetrieb eine echte
	// Goroutine. Injizierbar, weil Fynes Test-Treiber fyne.Do() synchron
	// auf der aufrufenden Goroutine ausführt (keine echte Marshalling wie
	// beim echten Treiber) — ohne diese Umschaltbarkeit würden Tests, die
	// währenddessen denselben Zustand lesen, unter -race einen echten
	// Data Race melden. Siehe AGENTS.md, Abschnitt zu Fyne-Tests.
	runBackground func(f func())

	data []account.MailAccount
	list *widget.List

	container *fyne.Container
}

// NewView erzeugt die Kontoübersicht mit leerem Zustand. Der Aufrufer
// muss anschließend Reload() aufrufen, um die Kontenliste zu laden — das
// passiert bewusst nicht schon im Konstruktor, damit z. B. Tests
// runBackground vorher umstellen können.
func NewView(accounts *manageaccount.UseCase, window fyne.Window) *View {
	v := &View{
		accounts:      accounts,
		window:        window,
		runBackground: func(f func()) { go f() },
	}

	v.list = widget.NewList(
		func() int { return len(v.data) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil,
				container.NewHBox(
					widget.NewButton(i18n.SettingsTestAccount, nil),
					widget.NewButton(i18n.SettingsDeleteAccount, nil),
				),
				widget.NewLabel(""),
			)
		},
		v.updateRow,
	)

	addButton := widget.NewButton(i18n.SettingsAddAccount, v.showAddDialog)
	addButton.Importance = widget.HighImportance

	v.container = container.NewBorder(
		container.NewHBox(widget.NewLabelWithStyle(i18n.SettingsTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), addButton),
		nil, nil, nil,
		v.list,
	)

	v.ExtendBaseWidget(v)
	return v
}

// CreateRenderer erfüllt fyne.Widget.
func (v *View) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.container)
}

func (v *View) updateRow(id widget.ListItemID, obj fyne.CanvasObject) {
	if id < 0 || id >= len(v.data) {
		return
	}
	acc := v.data[id]

	border := obj.(*fyne.Container)
	label := border.Objects[0].(*widget.Label)
	label.SetText(fmt.Sprintf("%s — %s@%s:%d", acc.DisplayName, acc.Username, acc.Host, acc.Port))

	buttons := border.Objects[1].(*fyne.Container)
	testButton := buttons.Objects[0].(*widget.Button)
	testButton.OnTapped = func() { v.testAccount(acc) }
	deleteButton := buttons.Objects[1].(*widget.Button)
	deleteButton.OnTapped = func() { v.confirmDelete(acc) }
}

// Reload lädt die Kontenliste neu — I/O läuft über runBackground, nie im
// UI-Thread.
func (v *View) Reload() {
	v.runBackground(func() {
		accounts, err := v.accounts.List(context.Background())
		fyne.Do(func() {
			if err != nil {
				dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.ErrorLoadFailed, v.window)
				return
			}
			v.data = accounts
			v.refreshContent()
		})
	})
}

// refreshContent zeigt entweder den Leerzustand oder die Liste, je
// nachdem ob Konten vorhanden sind (UMSETZUNGSPLAN.md Abschnitt 3.1:
// Leerzustände mit Handlungsaufforderung).
//
// v.container ist container.NewBorder(topBar, nil, nil, nil, v.list) — bei
// NewBorder landet das variadic "objects"-Argument (hier: v.list) IMMER an
// Index 0, die nicht-nil top/bottom/left/right-Objekte werden erst danach
// angehängt (hier: topBar an Index 1). Der Center-Slot ist also Index 0,
// nicht der letzte Index — mit Objects[len-1] wurde stattdessen die
// Kopfzeile (Titel + Hinzufügen-Button) überschrieben.
func (v *View) refreshContent() {
	if len(v.data) == 0 {
		empty := components.NewEmptyState(i18n.SettingsEmptyTitle, i18n.SettingsEmptyDetail, i18n.SettingsEmptyAction, v.showAddDialog)
		v.container.Objects[0] = empty
	} else {
		v.list.Refresh()
		v.container.Objects[0] = v.list
	}
	v.container.Refresh()
}

func (v *View) showAddDialog() {
	form := NewAccountForm(v.window)
	form.ListMailboxes = v.accounts.ListMailboxes
	dialog.ShowCustomConfirm(i18n.SettingsAddAccount, i18n.ButtonSave, i18n.ButtonBack, form, func(ok bool) {
		if !ok {
			return
		}
		v.submitNewAccount(form)
	}, v.window)
}

func (v *View) submitNewAccount(form *AccountForm) {
	if err := form.Validate(); err != nil {
		dialog.ShowInformation(i18n.ErrorGenericTitle, err.Error(), v.window)
		return
	}

	acc, err := form.BuildAccount(account.AccountID(uuid.NewString()))
	if err != nil {
		dialog.ShowInformation(i18n.ErrorGenericTitle, err.Error(), v.window)
		return
	}
	secret := form.Secret()

	v.runBackground(func() {
		createErr := v.accounts.Create(context.Background(), acc, secret)
		secret.Zero()

		fyne.Do(func() {
			if createErr != nil {
				dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.ErrorConnectFailed, v.window)
				return
			}
			v.Reload()
			if v.OnAccountsChanged != nil {
				v.OnAccountsChanged()
			}
		})
	})
}

func (v *View) testAccount(acc account.MailAccount) {
	// dialog.NewProgressInfinite ist deprecated (fyne v2.8.1) — Ersatz laut
	// Deprecation-Hinweis: eigener Inhalt mit ProgressBarInfinite in
	// NewCustomWithoutButtons.
	bar := widget.NewProgressBarInfinite()
	progress := dialog.NewCustomWithoutButtons(i18n.SettingsTestAccount,
		container.NewVBox(widget.NewLabel(i18n.OnboardingTestRunning), bar), v.window)
	progress.Show()

	v.runBackground(func() {
		err := v.accounts.TestConnectionByID(context.Background(), acc.ID)
		fyne.Do(func() {
			progress.Hide()
			if err != nil {
				dialog.ShowInformation(i18n.SettingsTestAccount, i18n.ErrorConnectFailed, v.window)
				return
			}
			dialog.ShowInformation(i18n.SettingsTestAccount, i18n.OnboardingTestSuccess, v.window)
		})
	})
}

func (v *View) confirmDelete(acc account.MailAccount) {
	dialog.ShowConfirm(i18n.SettingsDeleteConfirmTitle, i18n.SettingsDeleteConfirmBody, func(ok bool) {
		if !ok {
			return
		}
		v.runBackground(func() {
			err := v.accounts.Delete(context.Background(), acc.ID)
			fyne.Do(func() {
				if err != nil {
					dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.ErrorLoadFailed, v.window)
					return
				}
				v.Reload()
				if v.OnAccountsChanged != nil {
					v.OnAccountsChanged()
				}
			})
		})
	}, v.window)
}

// SetRunBackgroundForTest ersetzt die interne Hintergrund-Ausführung —
// für Tests aus anderen Paketen, die View einbetten (z. B. internal/ui).
// Nicht für Produktivcode gedacht, siehe AGENTS.md, Abschnitt zu
// Fyne-Tests.
func (v *View) SetRunBackgroundForTest(run func(func())) {
	v.runBackground = run
}
