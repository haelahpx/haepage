// Package fakeapps imitates the parts of the Jellyfin, Jellyseerr/Overseerr,
// Radarr and Sonarr APIs that Haepage uses, for tests and for trying the
// integrations without installing the apps (see tools/fakemedia).
//
// Each app lives under its own path prefix (/jellyfin, /seerr, /radarr,
// /sonarr) and accepts the API key Key.
package fakeapps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Key is the API key every fake app accepts.
const Key = "test-key"

type Session struct {
	ID, User, Client, Device string
	ItemID, Name, Series     string
	Season, Episode, Year    int
	Type                     string
	PositionMin, RuntimeMin  int
	Paused, Transcoding      bool
}

type Request struct {
	ID          int
	Type        string // movie or tv
	TmdbID      int
	Title       string
	RequestedBy string
	CreatedAt   time.Time
}

type QueueItem struct {
	ID              int
	Title           string // movie or series title
	Season, Episode int
	EpisodeTitle    string
	SizeMB, LeftMB  float64
	TimeLeft        string
}

type Upcoming struct {
	Title           string
	Season, Episode int
	EpisodeTitle    string
	Date            time.Time
}

// Apps holds the state of the fake apps. Change it under Lock to simulate
// activity.
type Apps struct {
	sync.Mutex
	Sessions []Session
	Requests []Request
	Radarr   AppState
	Sonarr   AppState
	Calls    map[string]int // path → number of calls, for tests
}

type AppState struct {
	Queue    []QueueItem
	Upcoming []Upcoming
	Health   []string // warning messages
}

// Demo returns apps with a little of everything going on.
func Demo(now time.Time) *Apps {
	return &Apps{
		Sessions: []Session{
			{ID: "s1", User: "Alex", Client: "Jellyfin Web", Device: "Living room TV", ItemID: "a1b2c3", Name: "The Long Night", Series: "Northern Lights", Season: 2, Episode: 4, Type: "Episode", PositionMin: 23, RuntimeMin: 52},
			{ID: "s2", User: "Sam", Client: "Jellyfin Android", Device: "Phone", ItemID: "d4e5f6", Name: "Ocean Drift", Year: 2024, Type: "Movie", PositionMin: 71, RuntimeMin: 118, Transcoding: true},
		},
		Requests: []Request{
			{ID: 41, Type: "movie", TmdbID: 1001, Title: "Starfall", RequestedBy: "Sam", CreatedAt: now.Add(-3 * time.Hour)},
			{ID: 42, Type: "tv", TmdbID: 2002, Title: "Harbor City", RequestedBy: "Jo", CreatedAt: now.Add(-40 * time.Minute)},
		},
		Radarr: AppState{
			Queue:    []QueueItem{{ID: 7, Title: "Starfall", SizeMB: 8200, LeftMB: 2900, TimeLeft: "00:14:10"}},
			Upcoming: []Upcoming{{Title: "Quiet Valley", Date: now.Add(50 * time.Hour)}},
		},
		Sonarr: AppState{
			Queue:    []QueueItem{{ID: 9, Title: "Harbor City", Season: 1, Episode: 1, EpisodeTitle: "Arrival", SizeMB: 1400, LeftMB: 1300, TimeLeft: "00:05:02"}},
			Upcoming: []Upcoming{{Title: "Northern Lights", Season: 2, Episode: 5, EpisodeTitle: "Thaw", Date: now.Add(26 * time.Hour)}},
			Health:   []string{"Indexer is unavailable due to failures for more than 6 hours"},
		},
	}
}

func (a *Apps) count(path string) {
	if a.Calls == nil {
		a.Calls = map[string]int{}
	}
	a.Calls[path]++
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// Handler serves all four fake apps.
func (a *Apps) Handler() http.Handler {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ok := r.Header.Get("X-Api-Key") == Key ||
				r.Header.Get("Authorization") == fmt.Sprintf(`MediaBrowser Token="%s"`, Key)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			a.Lock()
			a.count(r.URL.Path)
			a.Unlock()
			h(w, r)
		}
	}

	mux.HandleFunc("GET /jellyfin/System/Info", auth(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"ServerName": "fake", "Version": "10.10.0"})
	}))
	mux.HandleFunc("GET /jellyfin/Sessions", auth(func(w http.ResponseWriter, r *http.Request) {
		a.Lock()
		defer a.Unlock()
		out := []map[string]any{{"Id": "idle", "UserName": "Nobody", "Client": "Web"}} // a session with nothing playing
		for _, s := range a.Sessions {
			item := map[string]any{"Id": s.ItemID, "Name": s.Name, "Type": s.Type, "RunTimeTicks": int64(s.RuntimeMin) * 60 * 10_000_000}
			if s.Series != "" {
				item["SeriesName"], item["ParentIndexNumber"], item["IndexNumber"] = s.Series, s.Season, s.Episode
			}
			if s.Year > 0 {
				item["ProductionYear"] = s.Year
			}
			session := map[string]any{
				"Id": s.ID, "UserName": s.User, "Client": s.Client, "DeviceName": s.Device,
				"NowPlayingItem": item,
				"PlayState":      map[string]any{"PositionTicks": int64(s.PositionMin) * 60 * 10_000_000, "IsPaused": s.Paused},
			}
			if s.Transcoding {
				session["TranscodingInfo"] = map[string]any{"IsVideoDirect": false}
			}
			out = append(out, session)
		}
		writeJSON(w, out)
	}))
	mux.HandleFunc("GET /jellyfin/Items/{id}/Images/Primary", auth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2 3"><rect width="2" height="3" fill="#%s"/></svg>`, colorFor(r.PathValue("id")))
	}))

	mux.HandleFunc("GET /seerr/api/v1/request/count", auth(func(w http.ResponseWriter, r *http.Request) {
		a.Lock()
		defer a.Unlock()
		writeJSON(w, map[string]int{"total": len(a.Requests) + 10, "pending": len(a.Requests), "approved": 10})
	}))
	mux.HandleFunc("GET /seerr/api/v1/request", auth(func(w http.ResponseWriter, r *http.Request) {
		a.Lock()
		defer a.Unlock()
		var results []map[string]any
		for i := len(a.Requests) - 1; i >= 0; i-- { // newest first, like sort=added
			q := a.Requests[i]
			results = append(results, map[string]any{
				"id": q.ID, "type": q.Type, "createdAt": q.CreatedAt.Format(time.RFC3339),
				"requestedBy": map[string]string{"displayName": q.RequestedBy},
				"media":       map[string]any{"tmdbId": q.TmdbID, "mediaType": q.Type},
			})
		}
		writeJSON(w, map[string]any{"pageInfo": map[string]int{"results": len(results)}, "results": results})
	}))
	title := func(kind string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			a.Lock()
			defer a.Unlock()
			for _, q := range a.Requests {
				if fmt.Sprint(q.TmdbID) == r.PathValue("id") {
					field := map[string]string{"movie": "title", "tv": "name"}[kind]
					writeJSON(w, map[string]string{field: q.Title, "posterPath": "/fake" + r.PathValue("id") + ".jpg"})
					return
				}
			}
			http.NotFound(w, r)
		}
	}
	mux.HandleFunc("GET /seerr/api/v1/movie/{id}", auth(title("movie")))
	mux.HandleFunc("GET /seerr/api/v1/tv/{id}", auth(title("tv")))

	for _, app := range []string{"radarr", "sonarr"} {
		app := app
		state := func() *AppState {
			if app == "radarr" {
				return &a.Radarr
			}
			return &a.Sonarr
		}
		mux.HandleFunc("GET /"+app+"/api/v3/system/status", auth(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]string{"appName": strings.ToUpper(app[:1]) + app[1:], "version": "5.0.0"})
		}))
		mux.HandleFunc("GET /"+app+"/api/v3/queue", auth(func(w http.ResponseWriter, r *http.Request) {
			a.Lock()
			defer a.Unlock()
			var records []map[string]any
			for _, q := range state().Queue {
				rec := map[string]any{"id": q.ID, "title": q.Title + ".mkv", "size": q.SizeMB * 1e6, "sizeleft": q.LeftMB * 1e6, "timeleft": q.TimeLeft, "status": "downloading"}
				if app == "radarr" {
					rec["movie"] = map[string]string{"title": q.Title}
				} else {
					rec["series"] = map[string]string{"title": q.Title}
					rec["episode"] = map[string]any{"seasonNumber": q.Season, "episodeNumber": q.Episode, "title": q.EpisodeTitle}
				}
				records = append(records, rec)
			}
			writeJSON(w, map[string]any{"page": 1, "totalRecords": len(records), "records": records})
		}))
		mux.HandleFunc("GET /"+app+"/api/v3/calendar", auth(func(w http.ResponseWriter, r *http.Request) {
			a.Lock()
			defer a.Unlock()
			var out []map[string]any
			for _, u := range state().Upcoming {
				if app == "radarr" {
					out = append(out, map[string]any{"title": u.Title, "digitalRelease": u.Date.Format(time.RFC3339), "hasFile": false})
				} else {
					out = append(out, map[string]any{"title": u.EpisodeTitle, "seasonNumber": u.Season, "episodeNumber": u.Episode,
						"airDateUtc": u.Date.Format(time.RFC3339), "hasFile": false, "series": map[string]string{"title": u.Title}})
				}
			}
			writeJSON(w, out)
		}))
		mux.HandleFunc("GET /"+app+"/api/v3/health", auth(func(w http.ResponseWriter, r *http.Request) {
			a.Lock()
			defer a.Unlock()
			out := []map[string]string{{"source": "Update", "type": "notice", "message": "An update is available"}}
			for _, m := range state().Health {
				out = append(out, map[string]string{"source": "Indexer", "type": "warning", "message": m})
			}
			writeJSON(w, out)
		}))
	}
	return mux
}

func colorFor(s string) string {
	h := 0
	for _, c := range s {
		h = (h*31 + int(c)) % 0xffffff
	}
	return fmt.Sprintf("%06x", h)
}
