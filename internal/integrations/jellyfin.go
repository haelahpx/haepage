package integrations

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"
)

// Jellyfin shows who is watching what.
type Jellyfin struct{ http httpClient }

type JellyfinData struct {
	Sessions []JellyfinSession `json:"sessions"`
}

type JellyfinSession struct {
	ID          string `json:"id"`
	User        string `json:"user"`
	Client      string `json:"client"`
	Device      string `json:"device"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle,omitempty"`
	Type        string `json:"type"` // Movie, Episode, Audio, ...
	ItemID      string `json:"itemId"`
	PositionMS  int64  `json:"positionMs"`
	RuntimeMS   int64  `json:"runtimeMs"`
	Paused      bool   `json:"paused"`
	Transcoding bool   `json:"transcoding"`
}

func (j *Jellyfin) Interval() time.Duration { return 10 * time.Second }

func (j *Jellyfin) header(c Conn) http.Header {
	return http.Header{"Authorization": {fmt.Sprintf(`MediaBrowser Token="%s"`, c.Key)}}
}

func (j *Jellyfin) Test(ctx context.Context, c Conn) error {
	var info struct {
		ServerName string `json:"ServerName"`
		Version    string `json:"Version"`
	}
	if err := j.http.getJSON(ctx, c.BaseURL+"/System/Info", j.header(c), &info); err != nil {
		return err
	}
	if info.Version == "" {
		return fmt.Errorf("this doesn't look like Jellyfin")
	}
	return nil
}

func (j *Jellyfin) Poll(ctx context.Context, c Conn) (any, error) {
	var raw []struct {
		ID             string `json:"Id"`
		UserName       string `json:"UserName"`
		Client         string `json:"Client"`
		DeviceName     string `json:"DeviceName"`
		NowPlayingItem *struct {
			ID                string `json:"Id"`
			Name              string `json:"Name"`
			SeriesName        string `json:"SeriesName"`
			ParentIndexNumber int    `json:"ParentIndexNumber"`
			IndexNumber       int    `json:"IndexNumber"`
			Type              string `json:"Type"`
			RunTimeTicks      int64  `json:"RunTimeTicks"`
			ProductionYear    int    `json:"ProductionYear"`
		} `json:"NowPlayingItem"`
		PlayState struct {
			PositionTicks int64 `json:"PositionTicks"`
			IsPaused      bool  `json:"IsPaused"`
		} `json:"PlayState"`
		TranscodingInfo *struct {
			IsVideoDirect bool `json:"IsVideoDirect"`
		} `json:"TranscodingInfo"`
	}
	if err := j.http.getJSON(ctx, c.BaseURL+"/Sessions?activeWithinSeconds=960", j.header(c), &raw); err != nil {
		return nil, err
	}
	data := JellyfinData{Sessions: []JellyfinSession{}}
	for _, r := range raw {
		item := r.NowPlayingItem
		if item == nil {
			continue
		}
		s := JellyfinSession{
			ID: r.ID, User: r.UserName, Client: r.Client, Device: r.DeviceName,
			Title: item.Name, Type: item.Type, ItemID: item.ID,
			PositionMS: r.PlayState.PositionTicks / 10_000, RuntimeMS: item.RunTimeTicks / 10_000,
			Paused:      r.PlayState.IsPaused,
			Transcoding: r.TranscodingInfo != nil && !r.TranscodingInfo.IsVideoDirect,
		}
		switch {
		case item.Type == "Episode" && item.SeriesName != "":
			s.Title = item.SeriesName
			s.Subtitle = fmt.Sprintf("S%02dE%02d · %s", item.ParentIndexNumber, item.IndexNumber, item.Name)
		case item.ProductionYear > 0:
			s.Subtitle = fmt.Sprint(item.ProductionYear)
		}
		data.Sessions = append(data.Sessions, s)
	}
	return data, nil
}

func (j *Jellyfin) Diff(prev, cur any) []Activity {
	before := map[string]bool{}
	for _, s := range prev.(JellyfinData).Sessions {
		before[s.ID+"/"+s.ItemID] = true
	}
	var out []Activity
	for _, s := range cur.(JellyfinData).Sessions {
		if !before[s.ID+"/"+s.ItemID] {
			user := s.User
			if user == "" {
				user = "Someone"
			}
			out = append(out, Activity{Type: "playback.start", Title: user + " started watching " + s.Title, Detail: s.Subtitle})
		}
	}
	return out
}

func (j *Jellyfin) Webhook([]byte) []Activity { return nil }

var itemIDRE = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// Image returns an item's poster. Jellyfin needs the API key for it, so
// the browser gets it through Haepage.
func (j *Jellyfin) Image(ctx context.Context, c Conn, item string) ([]byte, string, error) {
	if !itemIDRE.MatchString(item) {
		return nil, "", fmt.Errorf("bad item id")
	}
	return j.http.get(ctx, c.BaseURL+"/Items/"+item+"/Images/Primary?fillHeight=180&quality=80", j.header(c))
}
