package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// runAccount verwaltet Konten (add/list/test/delete).
func runAccount(ctx context.Context, a *app, args []string) error {
	if len(args) == 0 {
		return errors.New("unterbefehl erwartet: dmarc-analyzer account add|list|test|delete")
	}

	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return runAccountAdd(ctx, a, rest)
	case "list":
		return runAccountList(ctx, a)
	case "test":
		return runAccountTest(ctx, a, rest)
	case "delete":
		return runAccountDelete(ctx, a, rest)
	default:
		return fmt.Errorf("unbekannter account-Unterbefehl %q", sub)
	}
}

// runAccountAdd fragt die Kontodaten über Flags ab, das Passwort
// interaktiv über stdin (nie als Flag — ein Passwort in der
// Shell-History oder Prozessliste wäre ein Leck, IMPLEMENTIERUNG.md
// Abschnitt 9).
func runAccountAdd(ctx context.Context, a *app, args []string) error {
	fs := flag.NewFlagSet("account add", flag.ContinueOnError)
	displayName := fs.String("display-name", "", "Anzeigename (Standard: Host)")
	host := fs.String("host", "", "IMAP-Host, z. B. imap.example.com")
	port := fs.Int("port", 993, "IMAP-Port")
	username := fs.String("username", "", "Benutzername / E-Mail-Adresse")
	mailbox := fs.String("mailbox", "INBOX", "Postfach")
	useTLS := fs.Bool("tls", true, "IMAPS verwenden (Klartext nur mit --tls=false, siehe IMPLEMENTIERUNG.md Abschnitt 9)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *host == "" || *username == "" {
		return errors.New("--host und --username sind erforderlich")
	}

	id := account.AccountID(uuid.NewString())
	acc, err := account.NewMailAccount(id, *displayName, *host, *port, *username, *mailbox, *useTLS, time.Now())
	if err != nil {
		return fmt.Errorf("kontodaten sind ungültig: %w", err)
	}

	fmt.Print("App-Passwort: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("passwort konnte nicht gelesen werden: %w", err)
	}
	secret := account.NewSecretFromString(strings.TrimRight(line, "\r\n"))
	defer secret.Zero()

	if err := a.accounts.Create(ctx, acc, secret); err != nil {
		return fmt.Errorf("konto konnte nicht angelegt werden: %w", err)
	}

	fmt.Printf("Konto %q angelegt (ID: %s)\n", acc.DisplayName, acc.ID)
	return nil
}

func runAccountList(ctx context.Context, a *app) error {
	accounts, err := a.accounts.List(ctx)
	if err != nil {
		return fmt.Errorf("konten konnten nicht geladen werden: %w", err)
	}
	if len(accounts) == 0 {
		fmt.Println("Keine Konten konfiguriert.")
		return nil
	}

	for _, acc := range accounts {
		fmt.Printf("%-36s  %-20s  %s@%s:%d (%s)\n", acc.ID, acc.DisplayName, acc.Username, acc.Host, acc.Port, acc.Mailbox)
	}
	return nil
}

func runAccountTest(ctx context.Context, a *app, args []string) error {
	id, err := requireAccountID(args)
	if err != nil {
		return err
	}

	if err := a.accounts.TestConnectionByID(ctx, id); err != nil {
		return fmt.Errorf("verbindungstest fehlgeschlagen: %w", err)
	}

	fmt.Printf("Verbindung zu %q erfolgreich.\n", id)
	return nil
}

func runAccountDelete(ctx context.Context, a *app, args []string) error {
	id, err := requireAccountID(args)
	if err != nil {
		return err
	}

	if err := a.accounts.Delete(ctx, id); err != nil {
		return fmt.Errorf("konto konnte nicht gelöscht werden: %w", err)
	}

	fmt.Printf("Konto %q gelöscht.\n", id)
	return nil
}

// requireAccountID liest die Konto-ID aus dem ersten Argument. Echte
// Validierung (existiert das Konto?) übernimmt account.Repository beim
// nächsten Zugriff — hier wird nur geprüft, dass überhaupt ein Argument da ist.
func requireAccountID(args []string) (account.AccountID, error) {
	if len(args) == 0 {
		return "", errors.New("konto-id erwartet")
	}
	return account.AccountID(args[0]), nil
}
