// Package onboarding enthält den Ersteinrichtungs-Assistenten: Konto
// eingeben, Verbindung testen, ersten Abgleich anstoßen
// (UMSETZUNGSPLAN.md Abschnitt 3.1: "Ersteinrichtung geführt — beim
// ersten Start ein Assistent in drei Schritten").
package onboarding

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/components"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/settings"
)

type step int

const (
	stepAccount step = iota
	stepTest
	stepFirstSync
)

// Wizard führt in drei Schritten durch die Ersteinrichtung: Konto
// eingeben, Verbindung testen (bevor gespeichert wird), ersten Abgleich
// anbieten.
type Wizard struct {
	widget.BaseWidget

	accounts *manageaccount.UseCase
	sync     *syncreports.UseCase
	window   fyne.Window
	// runBackground: siehe internal/ui/settings.View (AGENTS.md).
	runBackground func(f func())

	// OnComplete wird aufgerufen, sobald der Assistent fertig ist — Konto
	// gespeichert, unabhängig davon ob der erste Abgleich angestoßen oder
	// übersprungen wurde. Der Aufrufer (Hauptfenster) wechselt dann in die
	// normale Ansicht.
	OnComplete func()

	current step
	form    *settings.AccountForm
	account *account.MailAccount
	secret  account.Secret

	banner    *components.ErrorBanner
	body      *fyne.Container
	container *fyne.Container
}

// NewWizard erzeugt den Assistenten, startend bei Schritt 1 (Konto).
func NewWizard(accounts *manageaccount.UseCase, sync *syncreports.UseCase, window fyne.Window) *Wizard {
	w := &Wizard{
		accounts:      accounts,
		sync:          sync,
		window:        window,
		runBackground: func(f func()) { go f() },
		banner:        components.NewErrorBanner(),
	}

	title := widget.NewLabelWithStyle(i18n.OnboardingTitle, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	intro := widget.NewLabel(i18n.OnboardingIntro)
	intro.Wrapping = fyne.TextWrapWord

	w.body = container.NewVBox()
	w.container = container.NewVBox(title, intro, w.banner, w.body)

	w.ExtendBaseWidget(w)
	w.showAccountStep()
	return w
}

// CreateRenderer erfüllt fyne.Widget.
func (w *Wizard) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.container)
}

func (w *Wizard) setBody(objects ...fyne.CanvasObject) {
	w.body.Objects = objects
	w.body.Refresh()
}

// --- Schritt 1: Konto -------------------------------------------------

func (w *Wizard) showAccountStep() {
	w.current = stepAccount
	w.banner.Hide()
	w.form = settings.NewAccountForm(w.window)
	w.form.ListMailboxes = w.accounts.ListMailboxes

	next := widget.NewButton(i18n.ButtonNext, w.submitAccountStep)
	next.Importance = widget.HighImportance

	w.setBody(
		widget.NewLabelWithStyle(i18n.OnboardingStepAccount, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		w.form,
		container.NewHBox(next),
	)
}

func (w *Wizard) submitAccountStep() {
	if err := w.form.Validate(); err != nil {
		w.banner.SetError(err.Error(), "")
		return
	}

	acc, err := w.form.BuildAccount(account.AccountID(newAccountID()))
	if err != nil {
		w.banner.SetError(err.Error(), "")
		return
	}
	w.account = acc
	w.secret = w.form.Secret()
	w.banner.Hide()
	w.showTestStep()
}

// --- Schritt 2: Verbindung testen --------------------------------------

func (w *Wizard) showTestStep() {
	w.current = stepTest
	w.banner.Hide()

	prompt := widget.NewLabel(i18n.OnboardingTestPrompt)
	prompt.Wrapping = fyne.TextWrapWord

	back := widget.NewButton(i18n.ButtonBack, w.showAccountStep)
	testButton := widget.NewButton(i18n.OnboardingTestButton, w.runConnectionTest)
	testButton.Importance = widget.HighImportance

	w.setBody(
		widget.NewLabelWithStyle(i18n.OnboardingStepTest, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		prompt,
		container.NewHBox(back, testButton),
	)
}

func (w *Wizard) runConnectionTest() {
	w.banner.Hide()
	acc, secret := *w.account, w.secret

	w.runBackground(func() {
		err := w.accounts.TestConnection(context.Background(), acc, secret)
		fyne.Do(func() {
			if err != nil {
				w.banner.SetError(i18n.ErrorConnectFailed, err.Error())
				return
			}
			w.createAccount()
		})
	})
}

func (w *Wizard) createAccount() {
	acc, secret := w.account, w.secret

	w.runBackground(func() {
		err := w.accounts.Create(context.Background(), acc, secret)
		secret.Zero()

		fyne.Do(func() {
			if err != nil {
				w.banner.SetError(i18n.ErrorGenericTitle, err.Error())
				return
			}
			w.showFirstSyncStep()
		})
	})
}

// --- Schritt 3: Erster Abgleich -----------------------------------------

func (w *Wizard) showFirstSyncStep() {
	w.current = stepFirstSync
	w.banner.Hide()

	intro := widget.NewLabel(i18n.OnboardingFirstSyncIntro)
	intro.Wrapping = fyne.TextWrapWord

	now := widget.NewButton(i18n.OnboardingFirstSyncNow, w.runFirstSync)
	now.Importance = widget.HighImportance
	skip := widget.NewButton(i18n.OnboardingFirstSyncSkip, w.complete)

	w.setBody(
		widget.NewLabelWithStyle(i18n.OnboardingStepFirstSync, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		intro,
		container.NewHBox(skip, now),
	)
}

func (w *Wizard) runFirstSync() {
	w.banner.Hide()
	id := w.account.ID

	progress := widget.NewProgressBarInfinite()
	w.setBody(widget.NewLabel(i18n.SyncRunning), progress)

	w.runBackground(func() {
		result, err := w.sync.SyncAccount(context.Background(), id)
		fyne.Do(func() {
			if err != nil {
				w.banner.SetError(i18n.SyncErrorPrefix+err.Error(), "")
				w.showFirstSyncStep()
				return
			}
			w.showDoneStep(result)
		})
	})
}

// showDoneStep zeigt kurz das Ergebnis des ersten Abgleichs, bevor der
// Assistent schließt — ohne das würde runFirstSync bei Erfolg sofort in
// die normale Ansicht wechseln, ohne dass der Nutzer überhaupt sieht,
// was passiert ist.
func (w *Wizard) showDoneStep(result syncreports.Result) {
	done := widget.NewLabelWithStyle(i18n.OnboardingDone, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	summary := widget.NewLabel(fmt.Sprintf(i18n.SyncResultFmt, result.New, result.Skipped, result.Failed))

	finish := widget.NewButton(i18n.ButtonOK, w.complete)
	finish.Importance = widget.HighImportance

	w.setBody(done, summary, container.NewHBox(finish))
}

func (w *Wizard) complete() {
	if w.OnComplete != nil {
		w.OnComplete()
	}
}
