// Package ui enthält das Fyne-Hauptfenster, Navigation und Theme
// (IMPLEMENTIERUNG.md Abschnitt 4.1). Ruft ausschließlich Use Cases aus
// internal/app auf, nie direkt einen Infra-Adapter (AGENTS.md).
package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/onboarding"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/reports"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/settings"
)

// Dependencies bündelt die Use Cases, die das Hauptfenster braucht — die
// Composition Root (cmd/dmarc-analyzer, künftig auch der grafische
// Einstiegspunkt) verdrahtet sie gegen die echten Adapter.
type Dependencies struct {
	Accounts *manageaccount.UseCase
	Queries  *queryreports.UseCase
	Sync     *syncreports.UseCase
}

// BuildMainWindow erzeugt das Hauptfenster: eigenes Theme, dann je nach
// vorhandenen Konten entweder der Ersteinrichtungs-Assistent oder die
// normale Ansicht mit Navigation (IMPLEMENTIERUNG.md Abschnitt 10.1).
func BuildMainWindow(a fyne.App, deps Dependencies) fyne.Window {
	a.Settings().SetTheme(appTheme{})

	w := a.NewWindow(i18n.AppTitle)
	w.Resize(fyne.NewSize(1000, 700))

	sh := newShell(deps, w)
	w.SetContent(sh)
	sh.start()

	return w
}

// navItem sind die Einträge der seitlichen Navigation
// (IMPLEMENTIERUNG.md Abschnitt 10.1: "container.NewBorder + widget.List").
type navItem int

const (
	navReports navItem = iota
	navSettings
)

var navLabels = []string{i18n.NavReports, i18n.NavSettings}

// shell ist das Hauptfenster nach der Ersteinrichtung: Navigation links,
// Inhalt rechts, Sync-Knopf oben.
type shell struct {
	widget.BaseWidget

	deps   Dependencies
	window fyne.Window
	// runBackground: siehe internal/ui/settings.View (AGENTS.md).
	runBackground func(f func())

	reportsView  *reports.View
	settingsView *settings.View

	nav     *widget.List
	content *fyne.Container

	syncButton *widget.Button
	cancelSync context.CancelFunc

	container *fyne.Container
}

func newShell(deps Dependencies, window fyne.Window) *shell {
	s := &shell{
		deps:          deps,
		window:        window,
		runBackground: func(f func()) { go f() },
	}

	s.reportsView = reports.NewView(deps.Queries, window)
	s.settingsView = settings.NewView(deps.Accounts, window)
	s.settingsView.OnAccountsChanged = s.reportsView.Reload

	s.nav = widget.NewList(
		func() int { return len(navLabels) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(navLabels[id])
		},
	)
	s.nav.OnSelected = func(id widget.ListItemID) { s.selectNav(navItem(id)) }

	s.syncButton = widget.NewButton(i18n.SyncButton, s.startSync)
	s.syncButton.Importance = widget.HighImportance

	s.content = container.NewStack()

	s.container = container.NewBorder(
		container.NewHBox(widget.NewLabelWithStyle(i18n.AppTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), s.syncButton),
		nil, container.NewVScroll(s.nav), nil,
		s.content,
	)

	s.ExtendBaseWidget(s)
	return s
}

// CreateRenderer erfüllt fyne.Widget.
func (s *shell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.container)
}

func (s *shell) selectNav(item navItem) {
	switch item {
	case navSettings:
		s.content.Objects = []fyne.CanvasObject{s.settingsView}
	default:
		s.content.Objects = []fyne.CanvasObject{s.reportsView}
	}
	s.content.Refresh()
}

// start entscheidet zwischen Ersteinrichtung und normaler Ansicht — je
// nachdem, ob bereits ein Konto existiert (UMSETZUNGSPLAN.md Abschnitt
// 3.1: geführte Ersteinrichtung beim ersten Start).
func (s *shell) start() {
	s.runBackground(func() {
		accounts, err := s.deps.Accounts.List(context.Background())
		fyne.Do(func() {
			if err != nil || len(accounts) == 0 {
				s.showOnboarding()
				return
			}
			s.showMain()
		})
	})
}

func (s *shell) showOnboarding() {
	wiz := onboarding.NewWizard(s.deps.Accounts, s.deps.Sync, s.window)
	wiz.OnComplete = s.showMain
	s.content.Objects = []fyne.CanvasObject{wiz}
	s.content.Refresh()
	s.nav.Hide()
	s.syncButton.Hide()
}

func (s *shell) showMain() {
	s.nav.Show()
	s.syncButton.Show()
	s.settingsView.Reload()
	s.reportsView.Reload()
	s.nav.Select(int(navReports))
}

// startSync gleicht alle konfigurierten Konten ab — I/O läuft über
// runBackground, nie im UI-Thread (UMSETZUNGSPLAN.md AP 5). Der Knopf
// wechselt während des Laufs zu "Abbrechen".
func (s *shell) startSync() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelSync = cancel

	s.syncButton.SetText(i18n.SyncCancel)
	s.syncButton.OnTapped = s.requestCancelSync

	s.runBackground(func() {
		accounts, err := s.deps.Accounts.List(ctx)
		if err != nil {
			s.finishSync(nil, err)
			return
		}

		total := syncreports.Result{}
		var firstErr error
		for _, acc := range accounts {
			if ctx.Err() != nil {
				break
			}
			result, syncErr := s.deps.Sync.SyncAccount(ctx, acc.ID)
			total.New += result.New
			total.Skipped += result.Skipped
			total.Failed += result.Failed
			if syncErr != nil && firstErr == nil {
				firstErr = syncErr
			}
		}
		s.finishSync(&total, firstErr)
	})
}

func (s *shell) requestCancelSync() {
	if s.cancelSync != nil {
		s.cancelSync()
	}
	s.syncButton.SetText(i18n.SyncCancelling)
	s.syncButton.Disable()
}

func (s *shell) finishSync(result *syncreports.Result, err error) {
	fyne.Do(func() {
		s.syncButton.Enable()
		s.syncButton.SetText(i18n.SyncButton)
		s.syncButton.OnTapped = s.startSync
		s.cancelSync = nil

		if err != nil {
			dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.SyncErrorPrefix+err.Error(), s.window)
		} else if result != nil {
			dialog.ShowInformation(i18n.SyncButton, fmt.Sprintf(i18n.SyncResultFmt, result.New, result.Skipped, result.Failed), s.window)
		}
		s.reportsView.Reload()
	})
}
