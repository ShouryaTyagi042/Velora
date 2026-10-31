package postgres

import (
	"context"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/extractor"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// The extractor's queue is a query, not a separate table: a video is pending while it has no
// video_metadata row, or one that was never extracted and has failed fewer than MaxAttempts
// times. These methods implement extractor.Store on MediaRepo.

// Pending implements extractor.Store. Oldest first, so a fresh library fills in in the order it
// was added. Only what extraction needs is read: id, title, path.
func (r *MediaRepo) Pending(ctx context.Context, limit int) ([]media.Media, error) {
	rows, err := r.db.QueryContext(ctx, `
		select m.id, m.title, m.rel_path
		from media m
		left join video_metadata vm on vm.media_id = m.id
		where m.kind = 'video' and m.missing_since is null
		  and (vm.media_id is null or (vm.extracted_at is null and vm.attempts < $1))
		order by m.added_at, m.id
		limit $2`, extractor.MaxAttempts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []media.Media
	for rows.Next() {
		var id int64
		m := media.Media{Kind: media.KindVideo}
		if err := rows.Scan(&id, &m.Title, &m.RelPath); err != nil {
			return nil, err
		}
		m.ID = formatID(id)
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetVideoMeta implements extractor.Store.
func (r *MediaRepo) SetVideoMeta(ctx context.Context, id string, v media.VideoMeta) error {
	n, ok := parseID(id)
	if !ok {
		return media.ErrNotFound
	}
	_, err := r.db.ExecContext(ctx, `
		insert into video_metadata (media_id, duration_ms, width, height, codec, fps, extracted_at, attempts)
		values ($1, $2, $3, $4, $5, $6, $7, 1)
		on conflict (media_id) do update
		  set duration_ms = excluded.duration_ms, width = excluded.width, height = excluded.height,
		      codec = excluded.codec, fps = excluded.fps, extracted_at = excluded.extracted_at,
		      attempts = video_metadata.attempts + 1, last_error = null`,
		n, v.DurationMs, v.Width, v.Height, v.Codec, v.FPS, time.Now())
	return err
}

// RecordFailure implements extractor.Store: one more attempt, and why it failed.
func (r *MediaRepo) RecordFailure(ctx context.Context, id string, reason string) error {
	n, ok := parseID(id)
	if !ok {
		return media.ErrNotFound
	}
	_, err := r.db.ExecContext(ctx, `
		insert into video_metadata (media_id, attempts, last_error) values ($1, 1, $2)
		on conflict (media_id) do update
		  set attempts = video_metadata.attempts + 1, last_error = excluded.last_error`,
		n, reason)
	return err
}
