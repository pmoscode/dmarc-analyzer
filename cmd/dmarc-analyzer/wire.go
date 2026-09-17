package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/retention"
	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/config"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/dmarcxml"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/imap"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/keyring"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/mailmime"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sourceinfo"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
	"github.com/pmoscode/dmarc-analyzer/internal/platform/paths"
)

// app bündelt die für die CLI verdrahteten Use Cases. Das grafische
// Programm (AP 5) bekommt eine eigene, analoge Verdrahtung — beide teilen
// sich dieselben Use Cases, nur die Präsentationsschicht unterscheidet
// sich (Clean Architecture, IMPLEMENTIERUNG.md Abschnitt 4.1).
type app struct {
	db *sql.DB

	accounts    *manageaccount.UseCase
	queries     *queryreports.UseCase
	sync        *syncreports.UseCase
	importer    *importfiles.UseCase
	stats       *statistics.UseCase
	sourceStats *sourcestats.UseCase
	// retention ist unabhängig von Zugangsdaten für alle Unterbefehle
	// verdrahtet (AP 7) — sowohl die Web-Oberfläche (Einstellungen-Seite,
	// automatische Anwendung im Hintergrund) als auch ein künftiger
	// eigener CLI-Unterbefehl brauchen keinen Schlüsselbund dafür.
	retention *retention.UseCase
	// credentials ist nur für "web" gesetzt — die Web-Oberfläche braucht
	// den Store selbst (nicht nur die darauf aufbauenden Use Cases), um
	// auf /entsperren einen sperrbaren Store (LockableFileStore)
	// entsperren zu können (MIGRATIONSPLAN.md Erweiterung 9.4).
	credentials account.CredentialStore
}

// credentialAwareCommands sind die einzigen Unterbefehle, die Zugangsdaten
// brauchen — nur für sie wird der Credential-Store aufgebaut. Sonst würde
// z. B. ein harmloses "dmarc-analyzer stats" unnötig den echten
// OS-Schlüsselbund ansprechen (bei fehlendem Schlüsselbund sogar
// interaktiv nach einer Master-Passphrase fragen), obwohl stats gar keine
// Zugangsdaten braucht. "gui" ist Zugangsdaten-bewusst, weil sowohl die
// Konten- als auch die Sync-Ansicht des grafischen Programms Konten
// verwalten bzw. abgleichen können (siehe cmd_gui.go). "web" ebenso, seit
// M3 (Einstellungen/Ersteinrichtung/Sync in der Web-Oberfläche).
var credentialAwareCommands = map[string]bool{"sync": true, "account": true, "gui": true, "web": true}

// newApp öffnet die Datenbank und verdrahtet die für cmd tatsächlich
// benötigten Adapter mit den Use Cases. Einziger Ort im Programm, der
// konkrete Infra-Typen kennt (AGENTS.md: "cmd/dmarc-analyzer: einziger
// Ort, an dem Adapter mit Use Cases verdrahtet werden").
func newApp(ctx context.Context, cmd string) (*app, error) {
	dbPath, err := paths.DatabasePath()
	if err != nil {
		return nil, fmt.Errorf("datenbankpfad konnte nicht ermittelt werden: %w", err)
	}

	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("datenbank konnte nicht geöffnet werden: %w", err)
	}

	accountRepo := sqlite.NewAccountRepository(db)
	reportRepo := sqlite.NewReportRepository(db)
	stateRepo := sqlite.NewSyncStateRepository(db)
	failedRepo := sqlite.NewFailedImportRepository(db)
	statsRepo := sqlite.NewStatisticsRepository(db)
	sourceStatsRepo := sqlite.NewSourceStatsRepository(db)

	settingsPath, err := paths.SettingsPath()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("einstellungspfad konnte nicht ermittelt werden: %w", err)
	}
	settingsStore := config.NewStore(settingsPath)

	decoder := mailmime.NewDecoder()
	parsers := []domainsync.ReportParser{dmarcxml.NewParser()}
	newSource := func() domainsync.MessageSource { return imap.NewAdapter() }
	// enricher: eine gemeinsame Instanz für Dashboard und Sendequellen-
	// Ansicht — beide reichern dieselben Quell-IPs an, ein gemeinsamer
	// Cache spart doppelte PTR-Lookups.
	enricher := sourceinfo.NewEnricher()

	a := &app{
		db:      db,
		queries: &queryreports.UseCase{Reports: reportRepo},
		importer: &importfiles.UseCase{
			Reports:       reportRepo,
			FailedImports: failedRepo,
			Decoder:       decoder,
			Parsers:       parsers,
		},
		stats: &statistics.UseCase{Repository: statsRepo, Enricher: enricher},
		sourceStats: &sourcestats.UseCase{
			Sources:  sourceStatsRepo,
			Enricher: enricher,
		},
		retention: &retention.UseCase{
			Settings: settingsStore,
			Reports:  reportRepo,
		},
	}

	if credentialAwareCommands[cmd] {
		credentialStore, err := newCredentialStore(cmd)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		a.credentials = credentialStore

		a.accounts = &manageaccount.UseCase{
			Accounts:    accountRepo,
			Credentials: credentialStore,
			NewSource:   newSource,
		}
		a.sync = &syncreports.UseCase{
			Accounts:      accountRepo,
			Credentials:   credentialStore,
			States:        stateRepo,
			Reports:       reportRepo,
			FailedImports: failedRepo,
			Decoder:       decoder,
			Parsers:       parsers,
			NewSource:     newSource,
		}
	}

	return a, nil
}

func (a *app) Close() error {
	return a.db.Close()
}

// newCredentialStore wählt zwischen OS-Schlüsselbund und
// verschlüsseltem Datei-Fallback — IsAvailable() prüft per
// Kanarienwert, ob der Schlüsselbund tatsächlich nutzbar ist, statt aus
// einer bestimmten Fehlerklasse zu raten (IMPLEMENTIERUNG.md Abschnitt 9,
// siehe auch internal/infra/keyring/probe.go).
//
// Für cmd == "web" wird beim Datei-Fallback NIE auf der Konsole nach der
// Passphrase gefragt (anders als bei den übrigen Unterbefehlen): ein per
// Doppelklick gestarteter Server hat keine Konsole, die blockierend auf
// Eingabe wartet, würde also einfach hängen. Stattdessen liefert
// LockableFileStore einen gesperrten Store — die Web-Oberfläche fragt
// die Passphrase stattdessen auf /entsperren ab (MIGRATIONSPLAN.md
// Erweiterung 9.4).
func newCredentialStore(cmd string) (account.CredentialStore, error) {
	if keyring.IsAvailable() {
		return keyring.NewOSStore(), nil
	}

	dir, err := paths.ConfigDir()
	if err != nil {
		return nil, fmt.Errorf("konfigverzeichnis konnte nicht ermittelt werden: %w", err)
	}

	if cmd == "web" {
		return keyring.NewLockableFileStore(dir), nil
	}

	fmt.Fprintln(os.Stderr, "Kein Betriebssystem-Schlüsselbund verfügbar — Zugangsdaten werden "+
		"stattdessen verschlüsselt lokal abgelegt (IMPLEMENTIERUNG.md Abschnitt 9).")

	passphrase, err := readMasterPassphrase()
	if err != nil {
		return nil, err
	}
	defer passphrase.Zero()

	store, err := keyring.NewFileStore(dir, passphrase)
	if err != nil {
		return nil, fmt.Errorf("verschlüsselter Dateispeicher konnte nicht geöffnet werden: %w", err)
	}
	return store, nil
}

// readMasterPassphrase fragt die Passphrase für den Datei-Fallback
// interaktiv ab. Bewusst ohne verstecktes Eingabefeld (kein zusätzliches
// Terminal-Dependency wie golang.org/x/term) — für den seltenen Fall
// (kein OS-Schlüsselbund, vor allem Linux ohne Secret Service) reicht das
// für AP 4; eine versteckte Eingabe wäre eine sinnvolle Politur für AP 7.
func readMasterPassphrase() (account.Secret, error) {
	fmt.Print("Master-Passphrase: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return account.Secret{}, fmt.Errorf("passphrase konnte nicht gelesen werden: %w", err)
	}
	return account.NewSecretFromString(strings.TrimRight(line, "\r\n")), nil
}
