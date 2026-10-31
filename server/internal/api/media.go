package api

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// listResponse is one page of the list. NextCursor is null on the last page; otherwise passing it
// back as ?after= returns the next one.
type listResponse struct {
	Items      []media.Media `json:"items"`
	NextCursor *string       `json:"nextCursor"`
}

const (
	// maxFilterLen bounds ?actor= and ?q=, so a huge query string can't be used as a filter value.
	maxFilterLen = 200
	// defaultLimit fills a phone screen a few times over; maxLimit bounds what one request costs.
	defaultLimit = 50
	maxLimit     = 200
)

// listMedia handles GET /api/media?kind=video|comic&actor=<name>&q=<text>&sort=title|recent
// &limit=<n>&after=<cursor>: one page of the library.
//
// Every request is a page. Before phase 12 this returned the whole library: at 100k items that was
// 33 MB and 1.5 s, for a screen that shows a dozen cards.
func listMedia(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.URL.Query()
		f := media.ListFilter{Kind: media.Kind(q.Get("kind")), Actor: q.Get("actor"), Limit: defaultLimit}
		if f.Kind != "" && !f.Kind.Valid() {
			return validationError("kind must be %q or %q", media.KindVideo, media.KindComic)
		}
		if q.Has("actor") && f.Actor == "" {
			return validationError("actor must be non-empty")
		}
		f.Query = strings.TrimSpace(q.Get("q")) // an empty search is no search
		if len(f.Actor) > maxFilterLen || len(f.Query) > maxFilterLen {
			return validationError("actor and q must be at most %d bytes", maxFilterLen)
		}
		switch s := q.Get("sort"); s {
		case "", "title":
			f.Sort = media.SortTitle
		case "recent":
			f.Sort = media.SortRecent
		default:
			return validationError(`sort must be "title" or "recent"`)
		}
		if s := q.Get("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 1 || n > maxLimit {
				return validationError("limit must be a whole number from 1 to %d", maxLimit)
			}
			f.Limit = n
		}
		if c := q.Get("after"); c != "" {
			id, ok := decodeCursor(c, f.Sort)
			if !ok {
				return validationError("after is not a cursor from this list; start again without it")
			}
			f.After = id
		}

		// One extra row says whether there is a next page, without a separate count query.
		want := f.Limit
		f.Limit++
		items, err := repo.List(r.Context(), currentUser(r).ID, f)
		if err != nil {
			return err
		}
		resp := listResponse{Items: items}
		if len(items) > want {
			resp.Items = items[:want]
			next := encodeCursor(f.Sort, resp.Items[want-1].ID)
			resp.NextCursor = &next
		}
		respondCachedJSON(w, r, resp)
		return nil
	}
}

// A cursor is the order it belongs to and the id of the last item sent, base64url-encoded so
// clients treat it as opaque and don't build their own. It says where the page ended, not what
// was in it; the repository finds that item's sort key itself (media.ListFilter.After).
func encodeCursor(s media.Sort, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(sortName(s) + ":" + id))
}

// decodeCursor returns the id in c, if c is a cursor for order s. A cursor from the other order
// would continue from the right item in the wrong sequence, so it's refused.
func decodeCursor(c string, s media.Sort) (string, bool) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return "", false
	}
	name, id, ok := strings.Cut(string(b), ":")
	return id, ok && name == sortName(s) && id != ""
}

func sortName(s media.Sort) string {
	if s == media.SortRecent {
		return "recent"
	}
	return "title"
}

// getMedia handles GET /api/media/{id}.
func getMedia(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		m, err := repo.Get(r.Context(), currentUser(r).ID, id)
		if errors.Is(err, media.ErrNotFound) {
			return notFoundError("media %s not found", id)
		}
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, m)
		return nil
	}
}
