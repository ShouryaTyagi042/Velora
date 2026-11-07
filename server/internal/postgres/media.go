package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

// selectMedia reads media with their comic pages, video metadata, progress and actors, one row
// per item. $1 is the user whose progress to join (0 matches no rows: no progress).
//
// The actors come from a subquery per row, aggregated into a JSON array, so a page of 50 items is
// still one query, not 51 (N+1). It used to be a join plus GROUP BY over the whole query, and that
// is what made paging impossible: Postgres had to build and group every matching row before it
// could sort them and apply LIMIT (at 100k rows: 800 ms and a 25 MB sort on disk for 50 items).
// Without the GROUP BY, it walks an index in the requested order and stops after LIMIT rows,
// running the subquery only for those. The folder actor comes first (false sorts before true),
// then manual ones by name; json_agg over no rows is null, hence the coalesce.
const selectMedia = `
select m.id, m.fingerprint, m.kind, m.title, m.rel_path, coalesce(m.series, ''), coalesce(m.mime_type, ''), m.size_bytes,
       m.mod_time, m.added_at, cm.page_count, cm.pages,
       vm.extracted_at is not null, coalesce(vm.duration_ms, 0), coalesce(vm.width, 0),
       coalesce(vm.height, 0), coalesce(vm.codec, ''), coalesce(vm.fps, 0)::float8,
       coalesce(vm.preview_for = m.mod_time, false), coalesce(vm.short_for = m.mod_time, false),
       coalesce(wp.updated_at, rp.updated_at), coalesce(wp.position_ms, 0), coalesce(wp.duration_ms, 0),
       coalesce(rp.page, 0), coalesce(rp.page_count, 0), coalesce(wp.completed, rp.completed, false),
       coalesce((select json_agg(json_build_object('name', a.name, 'source', ma.source)
                                 order by ma.source = 'manual', a.name)
                 from media_actors ma join actors a on a.id = ma.actor_id
                 where ma.media_id = m.id), '[]'),
       coalesce((select json_agg(t.name order by t.name)
                 from media_tags mt join tags t on t.id = mt.tag_id
                 where mt.media_id = m.id), '[]'),
       exists (select 1 from favourites f where f.user_id = $1 and f.media_id = m.id),
       m.in_bin, m.binned_at,
       (select count(*) from media_views v where v.media_id = m.id), coalesce(m.source_url, '')
from media m
left join comic_metadata cm on cm.media_id = m.id
left join video_metadata vm on vm.media_id = m.id
left join watch_progress wp   on wp.media_id = m.id and wp.user_id = $1
left join reading_progress rp on rp.media_id = m.id and rp.user_id = $1
where m.missing_since is null`

// List implements media.Repository. Filters, order and paging all run in SQL.
//
// The statement is assembled from fixed pieces, one per filter actually set, rather than one
// statement with a catch-all condition per filter (kind matches $2, or $2 is empty). That catch-all form gets a single
// cached plan that has to work whether the kind is set or not, so it can't seek the
// (kind, title) index. Values are always parameters; only the fixed SQL text varies.
func (r *MediaRepo) List(ctx context.Context, user int64, f media.ListFilter) ([]media.Media, error) {
	q := selectMedia
	args := []any{user}
	param := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	if f.InBin {
		q += ` and m.in_bin` // media_bin_idx
	} else {
		q += ` and not m.in_bin`
	}
	if f.Favourites {
		q += ` and exists (select 1 from favourites f2 where f2.user_id = $1 and f2.media_id = m.id)`
	}
	if f.Tag != "" {
		q += ` and exists (select 1 from media_tags mt2 join tags t2 on t2.id = mt2.tag_id
		                   where mt2.media_id = m.id and t2.name = ` + param(f.Tag) + `)`
	}
	if f.Kind != "" {
		q += ` and m.kind = ` + param(string(f.Kind))
	}
	if f.Section != "" {
		// The section is the top-level folder (media.SectionOf), so it is read off rel_path rather
		// than stored twice. Only videos have one.
		q += ` and m.kind = 'video' and ` + sectionCond("m.rel_path", param)(f.Section)
	}
	if f.Series != "" {
		q += ` and m.series = ` + param(f.Series) // media_series_title_idx
	}
	switch f.Clip {
	case media.ClipPreview:
		q += ` and vm.preview_for = m.mod_time`
	case media.ClipShort:
		q += ` and vm.short_for = m.mod_time`
	}
	if f.Actor != "" {
		// An exists, not a join: the subquery in selectMedia still sees every actor of the video.
		q += ` and exists (select 1 from media_actors ma2 join actors a2 on a2.id = ma2.actor_id
		                   where ma2.media_id = m.id and a2.name = ` + param(f.Actor) + `)`
	}
	if f.Query != "" {
		// like, with the user's % and _ escaped, so they match themselves.
		p := param("%" + escapeLike(f.Query) + "%")
		q += ` and (velora_fold(m.title) like velora_fold(` + p + `)
		       or velora_fold(m.series) like velora_fold(` + p + `)
		       or exists (select 1 from media_actors ma3 join actors a3 on a3.id = ma3.actor_id
		                  where ma3.media_id = m.id and velora_fold(a3.name) like velora_fold(` + p + `))
		       or exists (select 1 from media_tags mt3 join tags t3 on t3.id = mt3.tag_id
		                  where mt3.media_id = m.id and velora_fold(t3.name) like velora_fold(` + p + `)))`
	}
	if f.After != "" {
		after, ok := parseID(f.After)
		if !ok {
			return []media.Media{}, nil // not an id this store hands out: nothing comes after it
		}
		// The cursor row's own sort key, read from the database. Row comparison (a, b) > (c, d)
		// means a > c, or a = c and b > d: exactly "after it" in a two-column order. An id that
		// doesn't exist gives no row, the comparison is null, and the page is empty.
		// Descending orders continue below the cursor (<), ascending ones above it (>).
		switch f.Sort {
		case media.SortBinned:
			q += ` and (m.binned_at, m.id) < (select binned_at, id from media where id = ` + param(after) + `)`
		case media.SortRecent:
			q += ` and (m.added_at, m.id) < (select added_at, id from media where id = ` + param(after) + `)`
		case media.SortTitleDesc:
			q += ` and (` + titleKey("m.title") + `, m.id) < (select ` + titleKey("title") + `, id from media where id = ` + param(after) + `)`
		default:
			q += ` and (` + titleKey("m.title") + `, m.id) > (select ` + titleKey("title") + `, id from media where id = ` + param(after) + `)`
		}
	}
	switch f.Sort {
	case media.SortBinned:
		q += ` order by m.binned_at desc nulls last, m.id desc`
	case media.SortRandom:
		q += ` order by random()` // a full scan of what matches; fine for one shuffle of a library
	case media.SortRecent:
		q += ` order by m.added_at desc, m.id desc`
	case media.SortTitleDesc:
		// Both columns descending: Postgres reads media_kind_title_idx backwards, no sort step.
		q += ` order by ` + titleKey("m.title") + ` desc, m.id desc`
	default:
		q += ` order by ` + titleKey("m.title") + `, m.id`
	}
	if f.Limit > 0 {
		// A literal, not a parameter, for the reason in Continue: with "limit $n" Postgres re-plans
		// the statement on every call (measured: 0.9 ms a page against 0.16 ms on the real
		// library). Clients use a few limits, so this adds a few cached statements, not hundreds.
		q += ` limit ` + strconv.Itoa(f.Limit)
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
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
func (r *MediaRepo) Get(ctx context.Context, user int64, id string) (media.Media, error) {
	return r.get(ctx, r.db, user, id)
}

func (r *MediaRepo) get(ctx context.Context, q querier, user int64, id string) (media.Media, error) {
	n, ok := parseID(id)
	if !ok {
		return media.Media{}, media.ErrNotFound
	}
	m, err := r.scanMedia(q.QueryRowContext(ctx, selectMedia+` and m.id = $2`, user, n))
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
		extracted bool
		v         media.VideoMeta
		saved     sql.NullTime
		p         media.Progress
		actors    []byte
		tags      []byte
		binnedAt  sql.NullTime
	)
	err := row.Scan(&id, &m.Fingerprint, &m.Kind, &m.Title, &m.RelPath, &m.Series, &m.MimeType, &m.SizeBytes,
		&m.ModTime, &m.AddedAt, &pageCount, r.types.SQLScanner(&pages),
		&extracted, &v.DurationMs, &v.Width, &v.Height, &v.Codec, &v.FPS, &v.Preview, &v.Short,
		&saved, &p.PositionMs, &p.DurationMs, &p.Page, &p.PageCount, &p.Completed, &actors,
		&tags, &m.Favourite, &m.InBin, &binnedAt, &m.Views, &m.SourceURL)
	if err != nil {
		return media.Media{}, err
	}
	m.ID = formatID(id)
	m.Section = media.SectionOf(m.Kind, m.RelPath)
	if pageCount.Valid {
		m.Comic = &media.ComicMeta{PageCount: int(pageCount.Int32), Pages: pages}
	}
	if extracted {
		m.Video = &v
	}
	if saved.Valid {
		p.UpdatedAt = saved.Time
		m.Progress = &p
	}
	if err := json.Unmarshal(actors, &m.Actors); err != nil {
		return media.Media{}, fmt.Errorf("media %d actors: %w", id, err)
	}
	if len(m.Actors) == 0 {
		m.Actors = nil // omitted from JSON, like memstore
	}
	if err := json.Unmarshal(tags, &m.Tags); err != nil {
		return media.Media{}, fmt.Errorf("media %d tags: %w", id, err)
	}
	if len(m.Tags) == 0 {
		m.Tags = nil
	}
	if binnedAt.Valid {
		t := binnedAt.Time
		m.BinnedAt = &t
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

		// Anything this scan didn't touch is gone from disk. Mark it, don't delete it: progress
		// rows point at it, and a disconnected drive shouldn't wipe your watch history.
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
		insert into media (fingerprint, kind, title, rel_path, series, mime_type, size_bytes, mod_time, last_seen_at)
		values ($1, $2, $3, $4, nullif($9, ''), nullif($5, ''), $6, $7, $8)
		on conflict (fingerprint) do update
		  set title = excluded.title, rel_path = excluded.rel_path, series = excluded.series,
		      mime_type = excluded.mime_type,
		      size_bytes = excluded.size_bytes, mod_time = excluded.mod_time,
		      last_seen_at = excluded.last_seen_at, missing_since = null
		returning id`,
		m.Fingerprint, m.Kind, m.Title, m.RelPath, m.MimeType, m.SizeBytes, m.ModTime, scannedAt,
		// From the path, by the one rule, not from m.Series: whatever built m, the column can't
		// disagree with rel_path.
		media.SeriesOf(m.Kind, m.RelPath),
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
func (r *MediaRepo) ListActors(ctx context.Context, section media.Section) ([]media.ActorCount, error) {
	// The cover is the newest of their videos that has a thumbnail (extracted metadata): the
	// first element of an ordered, filtered array_agg, so it costs no second query.
	q := `
		select a.name, count(*),
		       (array_agg(m.id order by m.added_at desc, m.id desc) filter (where vm.extracted_at is not null))[1],
		       (array_agg(m.mod_time order by m.added_at desc, m.id desc) filter (where vm.extracted_at is not null))[1]
		from actors a
		join media_actors ma on ma.actor_id = a.id
		join media m         on m.id = ma.media_id
		left join video_metadata vm on vm.media_id = m.id
		where m.kind = 'video' and m.missing_since is null and not m.in_bin`
	var args []any
	if section != "" {
		q += ` and ` + sectionCond("m.rel_path", func(v any) string { args = append(args, v); return "$1" })(section)
	}
	q += `
		group by a.id, a.name
		order by a.name`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []media.ActorCount{}
	for rows.Next() {
		var (
			c       media.ActorCount
			coverID sql.NullInt64
			coverMT sql.NullTime
		)
		if err := rows.Scan(&c.Name, &c.VideoCount, &coverID, &coverMT); err != nil {
			return nil, err
		}
		c.Cover = cover(coverID, coverMT)
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddActor implements media.Repository.
func (r *MediaRepo) AddActor(ctx context.Context, user int64, id, name string) (media.Media, error) {
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
		out, err = r.get(ctx, tx, user, id)
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

// ListSeries implements media.Repository: available episodes per series, in natural order.
func (r *MediaRepo) ListSeries(ctx context.Context) ([]media.SeriesCount, error) {
	// The cover is the first episode in natural order that has a thumbnail.
	rows, err := r.db.QueryContext(ctx, `
		select m.series, count(*),
		       (array_agg(m.id order by velora_title_key(m.title) collate "C", m.id) filter (where vm.extracted_at is not null))[1],
		       (array_agg(m.mod_time order by velora_title_key(m.title) collate "C", m.id) filter (where vm.extracted_at is not null))[1]
		from media m
		left join video_metadata vm on vm.media_id = m.id
		where m.series is not null and m.missing_since is null and not m.in_bin
		group by m.series
		order by velora_title_key(m.series) collate "C", m.series`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []media.SeriesCount{}
	for rows.Next() {
		var (
			c       media.SeriesCount
			coverID sql.NullInt64
			coverMT sql.NullTime
		)
		if err := rows.Scan(&c.Name, &c.EpisodeCount, &coverID, &coverMT); err != nil {
			return nil, err
		}
		c.Cover = cover(coverID, coverMT)
		out = append(out, c)
	}
	return out, rows.Err()
}

// titleKey is the natural sort key of a title column (velora_title_key, migration 0010), in byte
// order. It must be written exactly like the media_kind_title_idx expression for the index to apply.
func titleKey(col string) string { return `(velora_title_key(` + col + `) collate "C")` }

// sectionCond returns a condition on col (a rel_path column) matching videos in a section, the
// way media.SectionOf decides it: anime/… is anime, every other video is movies.
func sectionCond(col string, param func(any) string) func(media.Section) string {
	return func(s media.Section) string {
		p := param(string(media.SectionAnime) + "/%")
		if s == media.SectionAnime {
			return col + ` like ` + p
		}
		return col + ` not like ` + p
	}
}

// cover is an actor's or a series' cover from the two array_agg columns; nil when none has a thumbnail.
func cover(id sql.NullInt64, modTime sql.NullTime) *media.Cover {
	if !id.Valid || !modTime.Valid {
		return nil
	}
	return &media.Cover{ID: formatID(id.Int64), ModTime: modTime.Time} // as scanned, so the URL matches the video's own
}

func formatID(n int64) string { return strconv.FormatInt(n, 10) }

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// parseID turns an API id into the bigint key. Anything else can't exist, so it's not found.
func parseID(id string) (int64, bool) {
	n, err := strconv.ParseInt(id, 10, 64)
	return n, err == nil && n > 0
}

// escapeLike makes s match itself literally inside a like pattern: \ is like's default escape
// character, and % and _ are its wildcards.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// Locations returns every available item's id, kind and rel_path: what velora-admin
// import-sources matches the downloader's records against.
func (r *MediaRepo) Locations(ctx context.Context) ([]media.Media, error) {
	rows, err := r.db.QueryContext(ctx, `select id, kind, rel_path from media where missing_since is null`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []media.Media
	for rows.Next() {
		var (
			m  media.Media
			id int64
		)
		if err := rows.Scan(&id, &m.Kind, &m.RelPath); err != nil {
			return nil, err
		}
		m.ID = formatID(id)
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetSourceURLs stores each item's source page (media.source_url), all in one transaction, and
// returns how many rows changed: setting the URL an item already has is not a change, so
// importing the same records twice reports 0 the second time.
func (r *MediaRepo) SetSourceURLs(ctx context.Context, urls map[string]string) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	changed := 0
	for id, u := range urls {
		n, ok := parseID(id)
		if !ok {
			continue
		}
		res, err := tx.ExecContext(ctx,
			`update media set source_url = $2 where id = $1 and source_url is distinct from $2`, n, u)
		if err != nil {
			return 0, fmt.Errorf("media %s: %w", id, err)
		}
		k, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		changed += int(k)
	}
	return changed, tx.Commit()
}

// SetInBin implements media.Repository. binned_at keeps the first time it was binned while it
// stays there; restoring clears both.
func (r *MediaRepo) SetInBin(ctx context.Context, user int64, id string, inBin bool) (media.Media, error) {
	n, ok := parseID(id)
	if !ok {
		return media.Media{}, media.ErrNotFound
	}
	res, err := r.db.ExecContext(ctx, `
		update media set in_bin = $2, binned_at = case when $2 then coalesce(binned_at, now()) end
		where id = $1 and missing_since is null`, n, inBin)
	if err != nil {
		return media.Media{}, err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return media.Media{}, media.ErrNotFound
	}
	return r.Get(ctx, user, id)
}

// Delete implements media.Repository. Everything that points at the row goes with it (on delete
// cascade); an orphaned tag stays, and simply counts nothing.
func (r *MediaRepo) Delete(ctx context.Context, id string) error {
	n, ok := parseID(id)
	if !ok {
		return media.ErrNotFound
	}
	res, err := r.db.ExecContext(ctx, `delete from media where id = $1 and in_bin`, n)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k > 0 {
		return nil
	}
	var exists bool
	if err := r.db.QueryRowContext(ctx, `select exists (select 1 from media where id = $1)`, n).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return media.ErrNotInBin
	}
	return media.ErrNotFound
}

// SetFavourite implements media.Repository.
func (r *MediaRepo) SetFavourite(ctx context.Context, user int64, id string, favourite bool) (media.Media, error) {
	n, ok := parseID(id)
	if !ok {
		return media.Media{}, media.ErrNotFound
	}
	if _, err := r.Get(ctx, user, id); err != nil {
		return media.Media{}, err
	}
	q := `insert into favourites (user_id, media_id) values ($1, $2) on conflict do nothing`
	if !favourite {
		q = `delete from favourites where user_id = $1 and media_id = $2`
	}
	if _, err := r.db.ExecContext(ctx, q, user, n); err != nil {
		return media.Media{}, err
	}
	return r.Get(ctx, user, id)
}

// AddTag implements media.Repository.
func (r *MediaRepo) AddTag(ctx context.Context, user int64, id, name string) (media.Media, error) {
	return r.changeTag(ctx, user, id, func(tx *sql.Tx, n int64) error {
		var tagID int64
		if err := tx.QueryRowContext(ctx, `
			insert into tags (name) values ($1)
			on conflict (name) do update set name = excluded.name
			returning id`, name).Scan(&tagID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `insert into media_tags (media_id, tag_id) values ($1, $2) on conflict do nothing`, n, tagID)
		return err
	})
}

// RemoveTag implements media.Repository.
func (r *MediaRepo) RemoveTag(ctx context.Context, user int64, id, name string) (media.Media, error) {
	return r.changeTag(ctx, user, id, func(tx *sql.Tx, n int64) error {
		_, err := tx.ExecContext(ctx, `
			delete from media_tags mt using tags t
			where mt.tag_id = t.id and mt.media_id = $1 and t.name = $2`, n, name)
		return err
	})
}

// changeTag runs fn on a video's tags in a transaction, after checking it's an available video,
// and returns the video as it is afterwards.
func (r *MediaRepo) changeTag(ctx context.Context, user int64, id string, fn func(tx *sql.Tx, n int64) error) (media.Media, error) {
	n, ok := parseID(id)
	if !ok {
		return media.Media{}, media.ErrNotFound
	}
	var out media.Media
	err := inTx(ctx, r.db, func(tx *sql.Tx) error {
		var kind media.Kind
		err := tx.QueryRowContext(ctx, `select kind from media where id = $1 and missing_since is null for update`, n).Scan(&kind)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return media.ErrNotFound
		case err != nil:
			return err
		case kind != media.KindVideo:
			return media.ErrNotVideo
		}
		if err := fn(tx, n); err != nil {
			return err
		}
		out, err = r.get(ctx, tx, user, id)
		return err
	})
	return out, err
}

// ListTags implements media.Repository: tags on available, unbinned videos only.
func (r *MediaRepo) ListTags(ctx context.Context, section media.Section) ([]media.TagCount, error) {
	q := `
		select t.name, count(*)
		from tags t
		join media_tags mt on mt.tag_id = t.id
		join media m       on m.id = mt.media_id
		where m.missing_since is null and not m.in_bin`
	var args []any
	if section != "" {
		q += ` and ` + sectionCond("m.rel_path", func(v any) string { args = append(args, v); return "$1" })(section)
	}
	q += `
		group by t.name
		order by t.name`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []media.TagCount{}
	for rows.Next() {
		var c media.TagCount
		if err := rows.Scan(&c.Name, &c.VideoCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RecordView implements media.Repository. The half-hour check and the insert are one statement.
// Two requests for the same person and item arriving at the same instant could both count (under
// read committed neither sees the other's row): at worst one extra view, in a race the app never
// starts (it sends one view per player or reader session). A lock isn't worth it.
func (r *MediaRepo) RecordView(ctx context.Context, user int64, id string, at time.Time) (int, bool, error) {
	n, ok := parseID(id)
	if !ok {
		return 0, false, media.ErrNotFound
	}
	var exists bool
	if err := r.db.QueryRowContext(ctx, `select exists (select 1 from media where id = $1 and missing_since is null)`, n).Scan(&exists); err != nil {
		return 0, false, err
	}
	if !exists {
		return 0, false, media.ErrNotFound
	}
	res, err := r.db.ExecContext(ctx, `
		insert into media_views (media_id, user_id, viewed_at)
		select $1, $2, $3
		where not exists (select 1 from media_views
		                  where user_id = $2 and media_id = $1 and viewed_at > $3::timestamptz - $4::interval)`,
		n, user, at, fmt.Sprintf("%d seconds", int(media.ViewWindow.Seconds())))
	if err != nil {
		return 0, false, err
	}
	k, _ := res.RowsAffected()
	var views int
	if err := r.db.QueryRowContext(ctx, `select count(*) from media_views where media_id = $1`, n).Scan(&views); err != nil {
		return 0, false, err
	}
	return views, k > 0, nil
}
