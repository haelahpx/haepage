package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Arr covers Radarr (movies) and Sonarr (series): the download queue, the
// next week's releases, and health warnings.
type Arr struct {
	http httpClient
	App  string // radarr or sonarr
	now  func() time.Time
}

type ArrData struct {
	Queue    []ArrQueueItem `json:"queue"`
	Upcoming []ArrUpcoming  `json:"upcoming"`
	Health   []ArrHealth    `json:"health"`
}

type ArrQueueItem struct {
	ID       int     `json:"id"`
	Title    string  `json:"title"`
	Subtitle string  `json:"subtitle,omitempty"`
	Progress float64 `json:"progress"` // 0 to 100
	TimeLeft string  `json:"timeLeft,omitempty"`
	Status   string  `json:"status"`
}

type ArrUpcoming struct {
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle,omitempty"`
	Date     time.Time `json:"date"`
	HasFile  bool      `json:"hasFile"`
	Poster   string    `json:"poster,omitempty"`
}

type ArrHealth struct {
	Type    string `json:"type"` // warning or error
	Message string `json:"message"`
}

func (a *Arr) Interval() time.Duration { return 30 * time.Second }

func (a *Arr) header(c Conn) http.Header { return http.Header{"X-Api-Key": {c.Key}} }

func (a *Arr) clock() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}

func (a *Arr) Test(ctx context.Context, c Conn) error {
	var st struct {
		AppName string `json:"appName"`
	}
	if err := a.http.getJSON(ctx, c.BaseURL+"/api/v3/system/status", a.header(c), &st); err != nil {
		return err
	}
	if !strings.EqualFold(st.AppName, a.App) {
		if st.AppName == "" {
			return fmt.Errorf("this doesn't look like %s", title(a.App))
		}
		return fmt.Errorf("this is %s, not %s", st.AppName, title(a.App))
	}
	return nil
}

type arrImage struct {
	CoverType string `json:"coverType"`
	RemoteURL string `json:"remoteUrl"`
}

func poster(images []arrImage) string {
	for _, img := range images {
		if img.CoverType == "poster" && strings.HasPrefix(img.RemoteURL, "https://") {
			return img.RemoteURL
		}
	}
	return ""
}

func episodeLabel(season, episode int, name string) string {
	s := fmt.Sprintf("S%02dE%02d", season, episode)
	if name != "" {
		s += " · " + name
	}
	return s
}

func (a *Arr) Poll(ctx context.Context, c Conn) (any, error) {
	data := ArrData{Queue: []ArrQueueItem{}, Upcoming: []ArrUpcoming{}, Health: []ArrHealth{}}

	include := "includeMovie=true"
	if a.App == "sonarr" {
		include = "includeSeries=true&includeEpisode=true"
	}
	var queue struct {
		Records []struct {
			ID       int     `json:"id"`
			Title    string  `json:"title"`
			Size     float64 `json:"size"`
			SizeLeft float64 `json:"sizeleft"`
			TimeLeft string  `json:"timeleft"`
			Status   string  `json:"status"`
			Movie    *struct {
				Title string `json:"title"`
			} `json:"movie"`
			Series *struct {
				Title string `json:"title"`
			} `json:"series"`
			Episode *struct {
				SeasonNumber  int    `json:"seasonNumber"`
				EpisodeNumber int    `json:"episodeNumber"`
				Title         string `json:"title"`
			} `json:"episode"`
		} `json:"records"`
	}
	if err := a.http.getJSON(ctx, c.BaseURL+"/api/v3/queue?page=1&pageSize=20&"+include, a.header(c), &queue); err != nil {
		return nil, err
	}
	for _, r := range queue.Records {
		item := ArrQueueItem{ID: r.ID, Title: r.Title, TimeLeft: r.TimeLeft, Status: r.Status}
		if r.Movie != nil && r.Movie.Title != "" {
			item.Title = r.Movie.Title
		}
		if r.Series != nil && r.Series.Title != "" {
			item.Title = r.Series.Title
			if r.Episode != nil {
				item.Subtitle = episodeLabel(r.Episode.SeasonNumber, r.Episode.EpisodeNumber, r.Episode.Title)
			}
		}
		if r.Size > 0 {
			item.Progress = float64(int((1-r.SizeLeft/r.Size)*1000)) / 10
		}
		data.Queue = append(data.Queue, item)
	}

	now := a.clock()
	start, end := now.Format("2006-01-02"), now.AddDate(0, 0, 7).Format("2006-01-02")
	calURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s", c.BaseURL, start, end)
	if a.App == "sonarr" {
		calURL += "&includeSeries=true"
	}
	var cal []struct {
		Title           string     `json:"title"`
		HasFile         bool       `json:"hasFile"`
		InCinemas       *time.Time `json:"inCinemas"`
		DigitalRelease  *time.Time `json:"digitalRelease"`
		PhysicalRelease *time.Time `json:"physicalRelease"`
		Images          []arrImage `json:"images"`
		AirDateUtc      *time.Time `json:"airDateUtc"`
		SeasonNumber    int        `json:"seasonNumber"`
		EpisodeNumber   int        `json:"episodeNumber"`
		Series          *struct {
			Title  string     `json:"title"`
			Images []arrImage `json:"images"`
		} `json:"series"`
	}
	if err := a.http.getJSON(ctx, calURL, a.header(c), &cal); err != nil {
		return nil, err
	}
	for _, e := range cal {
		up := ArrUpcoming{Title: e.Title, HasFile: e.HasFile, Poster: poster(e.Images)}
		if e.Series != nil {
			up.Title = e.Series.Title
			up.Subtitle = episodeLabel(e.SeasonNumber, e.EpisodeNumber, e.Title)
			up.Poster = poster(e.Series.Images)
		}
		// The release that falls in the window: aired, digital, physical or cinema.
		for _, d := range []*time.Time{e.AirDateUtc, e.DigitalRelease, e.PhysicalRelease, e.InCinemas} {
			if d != nil && !d.Before(now.Truncate(24*time.Hour)) && (up.Date.IsZero() || d.Before(up.Date)) {
				up.Date = *d
			}
		}
		if !up.Date.IsZero() {
			data.Upcoming = append(data.Upcoming, up)
		}
	}
	sort.SliceStable(data.Upcoming, func(i, j int) bool { return data.Upcoming[i].Date.Before(data.Upcoming[j].Date) })
	if len(data.Upcoming) > 10 {
		data.Upcoming = data.Upcoming[:10]
	}

	var health []struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := a.http.getJSON(ctx, c.BaseURL+"/api/v3/health", a.header(c), &health); err != nil {
		return nil, err
	}
	for _, h := range health {
		if h.Type == "warning" || h.Type == "error" {
			data.Health = append(data.Health, ArrHealth{Type: h.Type, Message: h.Message})
		}
	}
	return data, nil
}

func (a *Arr) Diff(prev, cur any) []Activity {
	p, c := prev.(ArrData), cur.(ArrData)
	queued := map[int]bool{}
	for _, q := range p.Queue {
		queued[q.ID] = true
	}
	warned := map[string]bool{}
	for _, h := range p.Health {
		warned[h.Message] = true
	}
	var out []Activity
	for _, q := range c.Queue {
		if !queued[q.ID] {
			out = append(out, Activity{Type: "download.grabbed", Title: q.Title + " is downloading", Detail: q.Subtitle})
		}
	}
	for _, h := range c.Health {
		if !warned[h.Message] {
			out = append(out, Activity{Type: "health.warning", Title: title(a.App) + " needs attention", Detail: h.Message})
		}
	}
	return out
}

// Webhook reads the payload of the "Webhook" connection (Settings →
// Connect).
func (a *Arr) Webhook(body []byte) []Activity {
	var p struct {
		EventType string `json:"eventType"`
		Message   string `json:"message"`
		Movie     *struct {
			Title string `json:"title"`
		} `json:"movie"`
		Series *struct {
			Title string `json:"title"`
		} `json:"series"`
		Episodes []struct {
			SeasonNumber  int    `json:"seasonNumber"`
			EpisodeNumber int    `json:"episodeNumber"`
			Title         string `json:"title"`
		} `json:"episodes"`
	}
	if json.Unmarshal(body, &p) != nil {
		return nil
	}
	name, detail := "", ""
	if p.Movie != nil {
		name = p.Movie.Title
	}
	if p.Series != nil {
		name = p.Series.Title
		if len(p.Episodes) > 0 {
			e := p.Episodes[0]
			detail = episodeLabel(e.SeasonNumber, e.EpisodeNumber, e.Title)
		}
	}
	switch p.EventType {
	case "Grab":
		return []Activity{{Type: "download.grabbed", Title: name + " is downloading", Detail: detail}}
	case "Download":
		return []Activity{{Type: "download.imported", Title: name + " is ready to watch", Detail: detail}}
	case "Health":
		return []Activity{{Type: "health.warning", Title: title(a.App) + " needs attention", Detail: p.Message}}
	case "Test":
		return []Activity{{Type: "test", Title: "Webhook connected"}}
	}
	return nil
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
