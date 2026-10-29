// Command velora-admin manages users on the server's own machine. There is no signup endpoint:
// on a personal server, whoever can run this already owns the database.
//
//	velora-admin create-user <name>
//
// The password is read from the terminal without echo, typed twice; or, when stdin isn't a
// terminal, from its first line (echo "$PW" | velora-admin create-user ana), so it never appears
// in the process list or the shell history as an argument would.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/ShouryaTyagi042/Velora/server/internal/auth"
	"github.com/ShouryaTyagi042/Velora/server/internal/postgres"
	"github.com/ShouryaTyagi042/Velora/server/migrations"
)

const usage = "usage: velora-admin create-user <name>"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "velora-admin:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 || args[0] != "create-user" {
		return errors.New(usage)
	}
	name := args[1]

	dsn := os.Getenv("VELORA_DATABASE_URL")
	if dsn == "" {
		return errors.New("VELORA_DATABASE_URL is not set (make user loads it from .env)")
	}
	ctx := context.Background()
	db, err := postgres.Open(ctx, dsn)
	if err != nil {
		return fmt.Errorf("database %s: %w", postgres.Redact(dsn), err)
	}
	defer db.Close()
	// Creating the first user may come before the server's first start: bring the schema up first.
	if _, err := postgres.Migrate(ctx, db, migrations.FS); err != nil {
		return err
	}

	password, err := readPassword(os.Stdin)
	if err != nil {
		return err
	}
	u, err := auth.New(postgres.NewAuthRepo(db), auth.Options{}).CreateUser(ctx, name, password)
	switch {
	case errors.Is(err, auth.ErrUserExists):
		return fmt.Errorf("user %q already exists", name)
	case errors.Is(err, auth.ErrInvalidInput):
		return errors.New(strings.TrimPrefix(err.Error(), auth.ErrInvalidInput.Error()+": "))
	case err != nil:
		return err
	}
	fmt.Printf("created user %q; sign in to the app with it\n", u.Username)
	return nil
}

func readPassword(stdin *os.File) (string, error) {
	fd := int(stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Again: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("the passwords don't match")
	}
	return string(first), nil
}
