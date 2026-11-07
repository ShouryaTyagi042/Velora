// Command velora-admin manages users and library records on the server's own machine. There is
// no signup endpoint: on a personal server, whoever can run this already owns the database.
//
//	velora-admin create-user <name>
//	velora-admin import-sources <history.json|pending.json>...
//
// import-sources stores the web page each item was downloaded from (media.source_url), from the
// nomedia downloader's records. VELORA_MEDIA_DIR is the library folder the records' paths are
// matched under. Run a scan first: only items the database knows can be matched.
//
// The password is read from the terminal without echo, typed twice; or, when stdin isn't a
// terminal, from its first line (echo "$PW" | velora-admin create-user ana), so it never appears
// in the process list or the shell history as an argument would.
package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/ShouryaTyagi042/Velora/server/internal/auth"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/postgres"
	"github.com/ShouryaTyagi042/Velora/server/internal/sources"
	"github.com/ShouryaTyagi042/Velora/server/migrations"
)

const usage = "usage: velora-admin create-user <name> | import-sources <history.json|pending.json>..."

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "velora-admin:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	switch {
	case len(args) == 2 && args[0] == "create-user":
		return withDB(func(ctx context.Context, db *sql.DB) error { return createUser(ctx, db, args[1]) })
	case len(args) >= 2 && args[0] == "import-sources":
		return withDB(func(ctx context.Context, db *sql.DB) error { return importSources(ctx, db, args[1:]) })
	}
	return errors.New(usage)
}

// withDB opens the database and brings its schema up to date (an admin command may come before
// the server's first start), then runs fn.
func withDB(fn func(context.Context, *sql.DB) error) error {
	dsn := os.Getenv("VELORA_DATABASE_URL")
	if dsn == "" {
		return errors.New("VELORA_DATABASE_URL is not set (the make targets load it from .env)")
	}
	ctx := context.Background()
	db, err := postgres.Open(ctx, dsn)
	if err != nil {
		return fmt.Errorf("database %s: %w", postgres.Redact(dsn), err)
	}
	defer db.Close()
	if _, err := postgres.Migrate(ctx, db, migrations.FS); err != nil {
		return err
	}
	return fn(ctx, db)
}

// importSources reads the downloader's lists in the order given (put history.json first: what
// finished outranks what is only queued) and stores each matched item's page.
func importSources(ctx context.Context, db *sql.DB, files []string) error {
	root := os.Getenv("VELORA_MEDIA_DIR")
	if root == "" {
		return errors.New("VELORA_MEDIA_DIR is not set: it's the library folder the records are matched under")
	}
	var records []sources.Record
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			return err
		}
		rs, err := sources.Read(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		fmt.Printf("%s: %d records with a page\n", f, len(rs))
		records = append(records, rs...)
	}

	repo := postgres.NewMediaRepo(db)
	items, err := repo.Locations(ctx)
	if err != nil {
		return err
	}
	res := sources.MatchAll(records, items, root)
	urls := make(map[string]string, len(res.Matches))
	how := map[string]int{}
	for _, m := range res.Matches {
		urls[m.ID] = m.PageURL
		how[m.How]++
	}
	changed, err := repo.SetSourceURLs(ctx, urls)
	if err != nil {
		return err
	}

	var videos, comics, videosWith, comicsWith int
	for _, it := range items {
		_, has := urls[it.ID]
		if it.Kind == media.KindComic {
			comics++
			if has {
				comicsWith++
			}
		} else {
			videos++
			if has {
				videosWith++
			}
		}
	}
	fmt.Printf("matched %d items (%d by path, %d by name); %d updated, %d already had that page\n",
		len(res.Matches), how[sources.ByPath], how[sources.ByName], changed, len(res.Matches)-changed)
	fmt.Printf("videos: %d of %d have a page from these records; comics: %d of %d\n", videosWith, videos, comicsWith, comics)
	fmt.Printf("records not matched: %d (not in the library); left alone as ambiguous: %d\n", res.Unmatched, res.Ambiguous)
	return nil
}

func createUser(ctx context.Context, db *sql.DB, name string) error {
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
