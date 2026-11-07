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

// PendingPreviews implements extractor.Store: videos with metadata whose preview is missing, or
// was made from an older version of the file (preview_for isn't the file's mod_time), and hasn't
// failed MaxAttempts times. Oldest first, like Pending.
func (r *MediaRepo) PendingPreviews(ctx context.Context, limit int) ([]media.Media, error) {
	return r.pendingClips(ctx, clipPreview, limit)
}

// SetPreview implements extractor.Store. The attempts start again from 0: if the file changes
// later, its new preview gets the full number of tries.
func (r *MediaRepo) SetPreview(ctx context.Context, id string, modTime time.Time) error {
	return r.setClip(ctx, clipPreview, id, modTime)
}

// RecordPreviewFailure implements extractor.Store.
func (r *MediaRepo) RecordPreviewFailure(ctx context.Context, id string, reason string) error {
	return r.recordClipFailure(ctx, clipPreview, id, reason)
}

// PendingShorts, SetShort and RecordShortFailure are the same for the Shorts clips (0013).
func (r *MediaRepo) PendingShorts(ctx context.Context, limit int) ([]media.Media, error) {
	return r.pendingClips(ctx, clipShort, limit)
}

func (r *MediaRepo) SetShort(ctx context.Context, id string, modTime time.Time) error {
	return r.setClip(ctx, clipShort, id, modTime)
}

func (r *MediaRepo) RecordShortFailure(ctx context.Context, id string, reason string) error {
	return r.recordClipFailure(ctx, clipShort, id, reason)
}

// clipColumns is the prefix of a clip's three video_metadata columns: <prefix>_for,
// <prefix>_attempts, <prefix>_error. A fixed set: never built from input.
type clipColumns string

const (
	clipPreview clipColumns = "preview"
	clipShort   clipColumns = "short"
)

func (r *MediaRepo) pendingClips(ctx context.Context, c clipColumns, limit int) ([]media.Media, error) {
	rows, err := r.db.QueryContext(ctx, `
		select m.id, m.title, m.rel_path, m.mod_time, vm.duration_ms
		from media m
		join video_metadata vm on vm.media_id = m.id
		where m.kind = 'video' and m.missing_since is null
		  and vm.extracted_at is not null and vm.duration_ms > 0
		  and vm.`+string(c)+`_for is distinct from m.mod_time and vm.`+string(c)+`_attempts < $1
		order by m.added_at, m.id
		limit $2`, extractor.MaxAttempts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []media.Media
	for rows.Next() {
		var id int64
		m := media.Media{Kind: media.KindVideo, Video: &media.VideoMeta{}}
		if err := rows.Scan(&id, &m.Title, &m.RelPath, &m.ModTime, &m.Video.DurationMs); err != nil {
			return nil, err
		}
		m.ID = formatID(id)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MediaRepo) setClip(ctx context.Context, c clipColumns, id string, modTime time.Time) error {
	n, ok := parseID(id)
	if !ok {
		return media.ErrNotFound
	}
	_, err := r.db.ExecContext(ctx, `
		update video_metadata set `+string(c)+`_for = $2, `+string(c)+`_attempts = 0, `+string(c)+`_error = null
		where media_id = $1`, n, modTime)
	return err
}

func (r *MediaRepo) recordClipFailure(ctx context.Context, c clipColumns, id string, reason string) error {
	n, ok := parseID(id)
	if !ok {
		return media.ErrNotFound
	}
	_, err := r.db.ExecContext(ctx, `
		update video_metadata set `+string(c)+`_attempts = `+string(c)+`_attempts + 1, `+string(c)+`_error = $2
		where media_id = $1`, n, reason)
	return err
}
