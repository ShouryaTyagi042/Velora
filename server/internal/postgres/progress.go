package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// SaveProgress implements media.Repository. The upsert's where clause is the timestamp guard:
// an existing row is only overwritten by a newer save, so an older request that arrives last
// changes nothing. Either way the answer is the row as it stands afterwards.
func (r *MediaRepo) SaveProgress(ctx context.Context, user int64, id string, p media.Progress) (media.Progress, error) {
	n, ok := parseID(id)
	if !ok {
		return media.Progress{}, media.ErrNotFound
	}
	var out media.Progress
	err := inTx(ctx, r.db, func(tx *sql.Tx) error {
		var kind media.Kind
		err := tx.QueryRowContext(ctx,
			`select kind from media where id = $1 and missing_since is null`, n).Scan(&kind)
		if errors.Is(err, sql.ErrNoRows) {
			return media.ErrNotFound
		}
		if err != nil {
			return err
		}

		switch kind {
		case media.KindVideo:
			_, err = tx.ExecContext(ctx, `
				insert into watch_progress (user_id, media_id, position_ms, duration_ms, completed, updated_at)
				values ($1, $2, $3, $4, $5, $6)
				on conflict (user_id, media_id) do update
				  set position_ms = excluded.position_ms, duration_ms = excluded.duration_ms,
				      completed = excluded.completed, updated_at = excluded.updated_at
				  where watch_progress.updated_at < excluded.updated_at`,
				user, n, p.PositionMs, p.DurationMs, p.Completed, p.UpdatedAt)
		case media.KindComic:
			_, err = tx.ExecContext(ctx, `
				insert into reading_progress (user_id, media_id, page, page_count, completed, updated_at)
				values ($1, $2, $3, $4, $5, $6)
				on conflict (user_id, media_id) do update
				  set page = excluded.page, page_count = excluded.page_count,
				      completed = excluded.completed, updated_at = excluded.updated_at
				  where reading_progress.updated_at < excluded.updated_at`,
				user, n, p.Page, p.PageCount, p.Completed, p.UpdatedAt)
		}
		if err != nil {
			return err
		}

		m, err := r.get(ctx, tx, user, id)
		if err != nil {
			return err
		}
		out = *m.Progress
		return nil
	})
	return out, err
}

// Continue implements media.Repository. The (user_id, updated_at desc) index serves the order.
func (r *MediaRepo) Continue(ctx context.Context, user int64, kind media.Kind, limit int) ([]media.Media, error) {
	filter := ` and wp.media_id is not null and not wp.completed` + groupMedia + ` order by wp.updated_at desc, m.id limit $2`
	if kind == media.KindComic {
		filter = ` and rp.media_id is not null and not rp.completed` + groupMedia + ` order by rp.updated_at desc, m.id limit $2`
	}
	rows, err := r.db.QueryContext(ctx, selectMedia+filter, user, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []media.Media{}
	for rows.Next() {
		m, err := r.scanMedia(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}
