package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

// newSectionHandler is a library with one movie, one anime episode and one comic.
func newSectionHandler() http.Handler {
	return signedInHandler(memstore.New([]media.Media{
		{ID: "v1", Title: "The Matrix", Kind: media.KindVideo, RelPath: "movies/Keanu Reeves/The Matrix.mp4",
			Actors: []media.ActorTag{{Name: "Keanu Reeves", Source: media.ActorFromFolder}}},
		{ID: "a1", Title: "Episode 1", Kind: media.KindVideo, RelPath: "anime/Cowboy Bebop/Episode 1.mkv"},
		{ID: "c1", Title: "Watchmen", Kind: media.KindComic, RelPath: "comics/Watchmen",
			Comic: &media.ComicMeta{PageCount: 1, Pages: []string{"1.png"}}},
	}), newFakeJobs(nil, nil), fstest.MapFS{}, fstest.MapFS{})
}

func TestListBySection(t *testing.T) {
	tests := []struct {
		query   string
		wantIDs []string
	}{
		{"", []string{"a1", "v1", "c1"}},
		{"?section=anime", []string{"a1"}},
		{"?section=movies", []string{"v1"}},
		{"?kind=video&section=movies", []string{"v1"}},
		{"?kind=comic&section=anime", []string{}}, // comics have no section
		{"?series=Cowboy%20Bebop", []string{"a1"}},
		{"?series=Trigun", []string{}},
		{"?q=bebop", []string{"a1"}}, // search finds the series
	}
	h := newSectionHandler()
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			rec := get(t, h, "/api/media"+tt.query)
			var body listResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for _, m := range body.Items {
				got = append(got, m.ID)
			}
			if !slices.Equal(got, tt.wantIDs) {
				t.Errorf("ids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func TestSectionInJSON(t *testing.T) {
	h := newSectionHandler()
	for id, want := range map[string]string{"a1": "anime", "v1": "movies", "c1": ""} {
		var m map[string]any
		if err := json.Unmarshal(get(t, h, "/api/media/"+id).Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		if got, _ := m["section"].(string); got != want {
			t.Errorf("%s: section = %q, want %q", id, got, want)
		}
	}
}

func TestActorsBySection(t *testing.T) {
	h := newSectionHandler()
	for query, want := range map[string][]string{
		"":                {"Keanu Reeves"},
		"?section=anime":  nil, // a series is not an actor
		"?section=movies": {"Keanu Reeves"},
	} {
		var body actorsResponse
		if err := json.Unmarshal(get(t, h, "/api/actors"+query).Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, a := range body.Items {
			got = append(got, a.Name)
		}
		if !slices.Equal(got, want) {
			t.Errorf("/api/actors%s = %v, want %v", query, got, want)
		}
	}
}

func TestBadSection(t *testing.T) {
	h := newSectionHandler()
	for _, path := range []string{"/api/media?section=tv", "/api/actors?section=tv", "/api/media?series="} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", path, rec.Code)
		}
	}
}

func TestSeriesEndpoint(t *testing.T) {
	var body seriesResponse
	if err := json.Unmarshal(get(t, newSectionHandler(), "/api/series").Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if want := []media.SeriesCount{{Name: "Cowboy Bebop", EpisodeCount: 1}}; !slices.Equal(body.Items, want) {
		t.Errorf("/api/series = %v, want %v", body.Items, want)
	}
	var m map[string]any
	json.Unmarshal(get(t, newSectionHandler(), "/api/media/a1").Body.Bytes(), &m)
	if m["series"] != "Cowboy Bebop" || m["actors"] != nil {
		t.Errorf("anime JSON series = %v, actors = %v; want Cowboy Bebop and none", m["series"], m["actors"])
	}
}
