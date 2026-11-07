package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

// fakeRemover records what it was asked to remove, and fails for one id.
type fakeRemover struct {
	removed []string
	failFor string
}

func (f *fakeRemover) Remove(m media.Media) error {
	if m.ID == f.failFor {
		return errors.New("permission denied")
	}
	f.removed = append(f.removed, m.ID)
	return nil
}

func TestBinFlow(t *testing.T) {
	a, token := newTestAuth()
	rm := &fakeRemover{failFor: "v3"}
	h := NewHandler(memstore.New([]media.Media{
		{ID: "v1", Title: "One", Kind: media.KindVideo, RelPath: "movies/One.mp4"},
		{ID: "v2", Title: "Two", Kind: media.KindVideo, RelPath: "movies/Two.mp4"},
		{ID: "v3", Title: "Three", Kind: media.KindVideo, RelPath: "movies/Three.mp4"},
	}), newFakeJobs(nil, nil), fstest.MapFS{}, fstest.MapFS{}, fstest.MapFS{}, fstest.MapFS{}, rm, a)
	bearer := "Bearer " + token

	if rec := do(h, "DELETE", "/api/media/v1", "", bearer); rec.Code != http.StatusConflict {
		t.Fatalf("delete outside the bin = %d, want 409", rec.Code)
	}
	for _, id := range []string{"v1", "v2", "v3"} {
		if rec := do(h, "POST", "/api/media/"+id+"/bin", "", bearer); rec.Code != 200 {
			t.Fatalf("bin %s = %d %s", id, rec.Code, rec.Body)
		}
	}
	if rec := do(h, "GET", "/api/media", "", bearer); !jsonHasItems(t, rec.Body.Bytes(), 0) {
		t.Errorf("library after binning everything = %s, want empty", rec.Body)
	}
	if rec := do(h, "GET", "/api/media?bin=true&sort=binned", "", bearer); !jsonHasItems(t, rec.Body.Bytes(), 3) {
		t.Errorf("bin = %s, want 3 items", rec.Body)
	}
	if rec := do(h, "GET", "/api/media?sort=binned", "", bearer); rec.Code != 400 {
		t.Errorf("sort=binned without bin=true = %d, want 400", rec.Code)
	}

	if rec := do(h, "DELETE", "/api/media/v1", "", bearer); rec.Code != http.StatusNoContent {
		t.Fatalf("delete from the bin = %d %s", rec.Code, rec.Body)
	}
	// Emptying only comics leaves the videos alone.
	if rec := do(h, "DELETE", "/api/bin?kind=comic", "", bearer); rec.Code != 200 || rec.Body.String() != `{"deleted":0,"failed":0}`+"\n" {
		t.Fatalf("empty the bin's comics = %d %s, want nothing deleted", rec.Code, rec.Body)
	}
	rec := do(h, "DELETE", "/api/bin", "", bearer)
	var res emptyBinResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || rec.Code != 200 || res.Deleted != 1 || res.Failed != 1 {
		t.Fatalf("empty bin = %d %s, want 1 deleted and 1 failed", rec.Code, rec.Body)
	}
	if len(rm.removed) != 2 || rm.removed[0] != "v1" || rm.removed[1] != "v2" {
		t.Errorf("files removed for %v, want [v1 v2]", rm.removed)
	}
	// The one whose files couldn't be removed is still in the bin, so it can be tried again.
	if rec := do(h, "GET", "/api/media?bin=true", "", bearer); !jsonHasItems(t, rec.Body.Bytes(), 1) {
		t.Errorf("bin after emptying = %s, want the one that failed", rec.Body)
	}
}

func TestFavouritesAndTags(t *testing.T) {
	h, token := rawHandler()
	bearer := "Bearer " + token
	if rec := do(h, "PUT", "/api/media/v1/favourite", "", bearer); rec.Code != 200 {
		t.Fatalf("favourite = %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/api/media?favourites=true", "", bearer); !jsonHasItems(t, rec.Body.Bytes(), 1) {
		t.Errorf("favourites = %s", rec.Body)
	}
	if rec := do(h, "POST", "/api/media/v1/tags", `{"name":"  Sci  Fi "}`, bearer); rec.Code != 200 {
		t.Fatalf("tag = %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/api/media?tag=SCI+FI", "", bearer); !jsonHasItems(t, rec.Body.Bytes(), 1) {
		t.Errorf("by tag = %s: tags match normalised", rec.Body)
	}
	if rec := do(h, "GET", "/api/tags", "", bearer); rec.Code != 200 || rec.Body.String() != `{"items":[{"name":"sci fi","videoCount":1}]}`+"\n" {
		t.Errorf("tags = %d %q", rec.Code, rec.Body)
	}
	for body, want := range map[string]int{`{"name":"   "}`: 400, `{"name":"a/b"}`: 400} {
		if rec := do(h, "POST", "/api/media/v1/tags", body, bearer); rec.Code != want {
			t.Errorf("tag %s = %d, want %d", body, rec.Code, want)
		}
	}
	if rec := do(h, "DELETE", "/api/media/v1/tags/sci%20fi", "", bearer); rec.Code != 200 {
		t.Errorf("untag = %d %s", rec.Code, rec.Body)
	}
}

func jsonHasItems(t *testing.T, body []byte, n int) bool {
	t.Helper()
	var page struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("not a page: %s", body)
	}
	return len(page.Items) == n
}
