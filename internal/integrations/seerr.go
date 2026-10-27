package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Seerr covers Jellyseerr and Overseerr (same API): pending media
// requests, and instant notifications through their webhook agent.
type Seerr struct {
	http   httpClient
	mu     sync.Mutex
	titles map[string]seerrTitle // base URL + media type + TMDB id
}

type seerrTitle struct{ title, poster string }

type SeerrData struct {
	Pending  int            `json:"pending"`
	Requests []SeerrRequest `json:"requests"`
}

type SeerrRequest struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Poster      string    `json:"poster,omitempty"`
	Type        string    `json:"type"` // movie or tv
	RequestedBy string    `json:"requestedBy"`
	CreatedAt   time.Time `json:"createdAt"`
}

const tmdbPoster = "https://image.tmdb.org/t/p/w154"

func (s *Seerr) Interval() time.Duration { return 30 * time.Second }

func (s *Seerr) header(c Conn) http.Header { return http.Header{"X-Api-Key": {c.Key}} }

func (s *Seerr) Test(ctx context.Context, c Conn) error {
	var count struct {
		Total *int `json:"total"`
	}
	if err := s.http.getJSON(ctx, c.BaseURL+"/api/v1/request/count", s.header(c), &count); err != nil {
		return err
	}
	if count.Total == nil {
		return fmt.Errorf("this doesn't look like Jellyseerr or Overseerr")
	}
	return nil
}

func (s *Seerr) Poll(ctx context.Context, c Conn) (any, error) {
	var count struct {
		Pending int `json:"pending"`
	}
	if err := s.http.getJSON(ctx, c.BaseURL+"/api/v1/request/count", s.header(c), &count); err != nil {
		return nil, err
	}
	var list struct {
		Results []struct {
			ID          int       `json:"id"`
			Type        string    `json:"type"`
			CreatedAt   time.Time `json:"createdAt"`
			RequestedBy struct {
				DisplayName string `json:"displayName"`
			} `json:"requestedBy"`
			Media struct {
				TmdbID int `json:"tmdbId"`
			} `json:"media"`
		} `json:"results"`
	}
	if err := s.http.getJSON(ctx, c.BaseURL+"/api/v1/request?take=10&skip=0&filter=pending&sort=added", s.header(c), &list); err != nil {
		return nil, err
	}
	data := SeerrData{Pending: count.Pending, Requests: []SeerrRequest{}}
	for _, r := range list.Results {
		t := s.title(ctx, c, r.Type, r.Media.TmdbID)
		data.Requests = append(data.Requests, SeerrRequest{
			ID: r.ID, Title: t.title, Poster: t.poster, Type: r.Type,
			RequestedBy: r.RequestedBy.DisplayName, CreatedAt: r.CreatedAt,
		})
	}
	return data, nil
}

// title looks up a request's title and poster once per TMDB id.
func (s *Seerr) title(ctx context.Context, c Conn, mediaType string, tmdbID int) seerrTitle {
	key := fmt.Sprintf("%s|%s|%d", c.BaseURL, mediaType, tmdbID)
	s.mu.Lock()
	if t, ok := s.titles[key]; ok {
		s.mu.Unlock()
		return t
	}
	s.mu.Unlock()
	path := "/api/v1/movie/"
	if mediaType == "tv" {
		path = "/api/v1/tv/"
	}
	var info struct {
		Title      string `json:"title"`
		Name       string `json:"name"`
		PosterPath string `json:"posterPath"`
	}
	t := seerrTitle{title: "Unknown title"}
	if err := s.http.getJSON(ctx, fmt.Sprintf("%s%s%d", c.BaseURL, path, tmdbID), s.header(c), &info); err == nil {
		t.title = firstOf(info.Title, info.Name, t.title)
		if info.PosterPath != "" {
			t.poster = tmdbPoster + info.PosterPath
		}
		s.mu.Lock()
		if s.titles == nil {
			s.titles = map[string]seerrTitle{}
		}
		s.titles[key] = t
		s.mu.Unlock()
	}
	return t
}

func (s *Seerr) Diff(prev, cur any) []Activity {
	before := map[int]bool{}
	for _, r := range prev.(SeerrData).Requests {
		before[r.ID] = true
	}
	var out []Activity
	for _, r := range cur.(SeerrData).Requests {
		if !before[r.ID] {
			out = append(out, Activity{Type: "request.new", Title: firstOf(r.RequestedBy, "Someone") + " requested " + r.Title, Image: r.Poster})
		}
	}
	return out
}

// Webhook reads the default JSON payload of the webhook notification agent.
func (s *Seerr) Webhook(body []byte) []Activity {
	var p struct {
		Type    string `json:"notification_type"`
		Subject string `json:"subject"`
		Image   string `json:"image"`
		Request *struct {
			By string `json:"requestedBy_username"`
		} `json:"request"`
	}
	if json.Unmarshal(body, &p) != nil {
		return nil
	}
	by := "Someone"
	if p.Request != nil && p.Request.By != "" {
		by = p.Request.By
	}
	a := Activity{Image: p.Image}
	switch p.Type {
	case "MEDIA_PENDING":
		a.Type, a.Title = "request.new", by+" requested "+p.Subject
	case "MEDIA_APPROVED", "MEDIA_AUTO_APPROVED":
		a.Type, a.Title = "request.approved", p.Subject+" was approved"
	case "MEDIA_AVAILABLE":
		a.Type, a.Title = "request.available", p.Subject+" is ready to watch"
	case "TEST_NOTIFICATION":
		a.Type, a.Title = "test", "Webhook connected"
	default:
		return nil
	}
	return []Activity{a}
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
