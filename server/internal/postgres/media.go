package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// MediaRepo implements media.Repository and media.Syncer.
type MediaRepo struct {
	db    *sql.DB
	types *pgtype.Map // scans Postgres arrays (comic pages) into Go slices
}

func NewMediaRepo(db *sql.DB) *MediaRepo {
	return &MediaRepo{db: db, types: pgtype.NewMap()}
}

// querier is what *sql.DB and *sql.Tx have in common, so a read can run inside a write's transaction.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// selectMedia reads media with their comic pages and actors, one row per item. The actors are
// aggregated into a JSON array in the same query, so a page of 50 items is one query, not 51 (N+1).
// The folder actor comes first (false sorts before true), then manual ones by name.
// Left joins keep comics and actor-less videos; the filter stops json_agg from producing
// [{"name":null,"source":null}] for them.
const selectMedia = `
select m.id, m.fingerprint, m.kind, m.title, m.rel_path, coalesce(m.mime_type, ''), m.size_bytes,
       m.mod_time, m.added_at, cm.page_count, cm.pages,
       coalesce(json_agg(json_build_object('name', a.name, 'source', ma.source)
                         order by ma.source = 'manual', a.name)
                  filter (where a.id is not null), '[]')
from media m
left join comic_metadata cm on cm.media_id = m.id
left join media_actors ma   on ma.media_id = m.id
left join actors a          on a.id = ma.actor_id
where m.missing_since is null`

const groupMedia = `
group by m.id, cm.media_id`

// List implements media.Repository. Both filters run in SQL. The actor filter is an exists, not a
// join, so the aggregate still sees every actor of a matching video.
func (r *MediaRepo) List(ctx context.Context, f media.ListFilter) ([]media.Media, error) {
	rows, err := r.db.QueryContext(ctx, selectMedia+`
  and ($1::text = '' or m.kind = $1)
  and ($2::text = '' or exists (
        select 1 from media_actors ma2 join actors a2 on a2.id = ma2.actor_id
        where ma2.media_id = m.id and a2.name = $2))`+groupMedia+`
order by lower(m.title), m.id`, string(f.Kind), f.Actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []media.Media{} // [] rather than null in JSON
	for rows.Next() {
		m, err := r.scanMedia(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	// The loop also ends on an error mid-stream; only Err tells the two apart.
	return items, rows.Err()
}

// Get implements media.Repository.
func (r *MediaRepo) Get(ctx context.Context, id string) (media.Media, error) {
	return r.get(ctx, r.db, id)
}

func (r *MediaRepo) get(ctx context.Context, q querier, id string) (media.Media, error) {
	n, ok := parseID(id)
	if !ok {
		return media.Media{}, media.ErrNotFound
	}
	m, err := r.scanMedia(q.QueryRowContext(ctx, selectMedia+` and m.id = $1`+groupMedia, n))
	if errors.Is(err, sql.ErrNoRows) {
		return media.Media{}, media.ErrNotFound
	}
	return m, err
}

// scanMedia reads one row of selectMedia. *sql.Row and *sql.Rows both have Scan.
func (r *MediaRepo) scanMedia(row interface{ Scan(...any) error }) (media.Media, error) {
	var (
		m         media.Media
		id        int64
		pageCount sql.NullInt32
		pages     []string
		actors    []byte
	)
	err := row.Scan(&id, &m.Fingerprint, &m.Kind, &m.Title, &m.RelPath, &m.MimeType, &m.SizeBytes,
		&m.ModTime, &m.AddedAt, &pageCount, r.types.SQLScanner(&pages), &actors)
	if err != nil {
		return media.Media{}, err
	}
	m.ID = formatID(id)
	if pageCount.Valid {
		m.Comic = &media.ComicMeta{PageCount: int(pageCount.Int32), Pages: pages}
	}
	if err := json.Unmarshal(actors, &m.Actors); err != nil {
		return media.Media{}, fmt.Errorf("media %d actors: %w", id, err)
	}
	if len(m.Actors) == 0 {
		m.Actors = nil // omitted from JSON, like memstore
	}
	return m, nil
}

// Sync implements media.Syncer: one transaction for the whole scan.
func (r *MediaRepo) Sync(ctx context.Context, items []media.Media, scannedAt time.Time) (media.SyncResult, error) {
	var res media.SyncResult
	err := inTx(ctx, r.db, func(tx *sql.Tx) error {
		for _, m := range items {
			if err := upsert(ctx, tx, m, scannedAt); err != nil {
				return fmt.Errorf("upsert %s: %w", m.RelPath, err)
			}
		}
		res.Upserted = len(items)

		// Anything this scan didn't touch is gone from disk. Mark it, don't delete it: manual actors
		// (and, later, progress) point at it, and a disconnected drive shouldn't wipe them.
		out, err := tx.ExecContext(ctx, `
			update media set missing_since = $1
			where last_seen_at < $1 and missing_since is null`, scannedAt)
		if err != nil {
			return err
		}
		n, err := out.RowsAffected()
		res.Missing = int(n)
		return err
	})
	return res, err
}

func upsert(ctx context.Context, tx *sql.Tx, m media.Media, scannedAt time.Time) error {
	// on conflict (fingerprint): "what makes this the same item" is one column with one name.
	// added_at is set on insert only; missing_since clears when a missing file comes back.
	var id int64
	err := tx.QueryRowContext(ctx, `
		insert into media (fingerprint, kind, title, rel_path, mime_type, size_bytes, mod_time, last_seen_at)
		values ($1, $2, $3, $4, nullif($5, ''), $6, $7, $8)
		on conflict (fingerprint) do update
		  set title = excluded.title, rel_path = excluded.rel_path, mime_type = excluded.mime_type,
		      size_bytes = excluded.size_bytes, mod_time = excluded.mod_time,
		      last_seen_at = excluded.last_seen_at, missing_since = null
		returning id`,
		m.Fingerprint, m.Kind, m.Title, m.RelPath, m.MimeType, m.SizeBytes, m.ModTime, scannedAt,
	).Scan(&id)
	if err != nil {
		return err
	}

	switch m.Kind {
	case media.KindComic:
		if m.Comic == nil {
			return errors.New("comic without pages")
		}
		// The pages and their count change together, and only when the list did.
		_, err = tx.ExecContext(ctx, `
			insert into comic_metadata (media_id, page_count, pages) values ($1, $2, $3)
			on conflict (media_id) do update
			  set page_count = excluded.page_count, pages = excluded.pages
			  where comic_metadata.pages is distinct from excluded.pages`,
			id, m.Comic.PageCount, m.Comic.Pages)
		return err
	case media.KindVideo:
		return syncFolderActor(ctx, tx, id, m.Actors)
	}
	return nil
}

// syncFolderActor makes the video's folder link match the disk and leaves manual links alone:
// the scanner owns only source = 'folder' rows.
func syncFolderActor(ctx context.Context, tx *sql.Tx, mediaID int64, tags []media.ActorTag) error {
	if _, err := tx.ExecContext(ctx,
		`delete from media_actors where media_id = $1 and source = 'folder'`, mediaID); err != nil {
		return err
	}
	for _, a := range tags {
		if a.Source != media.ActorFromFolder {
			continue // manual links are written only by AddActor
		}
		actorID, err := upsertActor(ctx, tx, a.Name)
		if err != nil {
			return err
		}
		// The folder wins: an earlier manual link to the same actor becomes the folder's.
		if _, err := tx.ExecContext(ctx, `
			insert into media_actors (media_id, actor_id, source) values ($1, $2, 'folder')
			on conflict (media_id, actor_id) do update set source = 'folder'`,
			mediaID, actorID); err != nil {
			return err
		}
	}
	return nil
}

// upsertActor returns the actor's id, creating the actor if new. "do nothing" would return no
// row for an existing actor; the no-op update makes returning yield its id.
func upsertActor(ctx context.Context, q querier, name string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		insert into actors (name) values ($1)
		on conflict (name) do update set name = excluded.name
		returning id`, name).Scan(&id)
	return id, err
}

// ListActors implements media.Repository: both sources count, only available videos.
// The primary key allows one link per (video, actor), so count(*) counts distinct videos.
func (r *MediaRepo) ListActors(ctx context.Context) ([]media.ActorCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		select a.name, count(*)
		from actors a
		join media_actors ma on ma.actor_id = a.id
		join media m         on m.id = ma.media_id
		where m.kind = 'video' and m.missing_since is null
		group by a.id, a.name
		order by a.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []media.ActorCount{}
	for rows.Next() {
		var c media.ActorCount
		if err := rows.Scan(&c.Name, &c.VideoCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddActor implements media.Repository.
func (r *MediaRepo) AddActor(ctx context.Context, id, name string) (media.Media, error) {
	n, ok := parseID(id)
	if !ok {
		return media.Media{}, media.ErrNotFound
	}
	var out media.Media
	err := inTx(ctx, r.db, func(tx *sql.Tx) error {
		var kind media.Kind
		err := tx.QueryRowContext(ctx,
			`select kind from media where id = $1 and missing_since is null`, n).Scan(&kind)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return media.ErrNotFound
		case err != nil:
			return err
		case kind != media.KindVideo:
			return media.ErrNotVideo
		}

		actorID, err := upsertActor(ctx, tx, name)
		if err != nil {
			return err
		}
		// An existing link, folder or manual, is left as it is: a manual add never downgrades
		// the scanner's row.
		if _, err := tx.ExecContext(ctx, `
			insert into media_actors (media_id, actor_id, source) values ($1, $2, 'manual')
			on conflict (media_id, actor_id) do nothing`, n, actorID); err != nil {
			return err
		}
		out, err = r.get(ctx, tx, id)
		return err
	})
	return out, err
}

// RemoveActor implements media.Repository.
func (r *MediaRepo) RemoveActor(ctx context.Context, id, name string) error {
	n, ok := parseID(id)
	if !ok {
		return media.ErrNotFound
	}
	return inTx(ctx, r.db, func(tx *sql.Tx) error {
		// for update locks the link, so a scan can't turn it into a folder link between this
		// select and the delete.
		var source media.ActorSource
		err := tx.QueryRowContext(ctx, `
			select ma.source
			from media_actors ma
			join actors a on a.id = ma.actor_id
			where ma.media_id = $1 and a.name = $2
			for update of ma`, n, name).Scan(&source)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return media.ErrNotFound // unknown media and unknown link are both 404
		case err != nil:
			return err
		case source == media.ActorFromFolder:
			return media.ErrFolderActor
		}
		// The ownership rule is in the SQL too, not only in the Go above.
		_, err = tx.ExecContext(ctx, `
			delete from media_actors ma using actors a
			where ma.actor_id = a.id and ma.media_id = $1 and a.name = $2 and ma.source = 'manual'`,
			n, name)
		return err
	})
}

func formatID(n int64) string { return strconv.FormatInt(n, 10) }

// parseID turns an API id into the bigint key. Anything else can't exist, so it's not found.
func parseID(id string) (int64, bool) {
	n, err := strconv.ParseInt(id, 10, 64)
	return n, err == nil && n > 0
}
