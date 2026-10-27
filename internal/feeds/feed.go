// Package feeds parses RSS, Atom and iCalendar (ICS) for the feed,
// release and calendar widgets (IMPLEMENTATION.md M4.5). The server fetches
// and parses them, so browsers need no CORS access to the source.
package feeds

import (
	"bytes"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ErrFormat means the data isn't a feed or calendar Haepage can read.
var ErrFormat = errors.New("not a feed")

// Item is one feed entry.
type Item struct {
	Title   string     `json:"title"`
	Link    string     `json:"link,omitempty"`
	Date    *time.Time `json:"date,omitempty"`
	Summary string     `json:"summary,omitempty"`
}

// Feed is a parsed RSS or Atom feed, newest first.
type Feed struct {
	Title string `json:"title"`
	Items []Item `json:"items"`
}

type rssDoc struct {
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			GUID        string `xml:"guid"`
			PubDate     string `xml:"pubDate"`
			Date        string `xml:"http://purl.org/dc/elements/1.1/ date"`
			Description string `xml:"description"`
		} `xml:"item"`
	} `xml:"channel"`
	// RSS 1.0 (RDF) keeps items next to the channel.
	Items []struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Date        string `xml:"http://purl.org/dc/elements/1.1/ date"`
		Description string `xml:"description"`
	} `xml:"item"`
}

type atomDoc struct {
	Title   string `xml:"title"`
	Entries []struct {
		Title string `xml:"title"`
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
		Updated   string `xml:"updated"`
		Published string `xml:"published"`
		Summary   string `xml:"summary"`
		Content   string `xml:"content"`
	} `xml:"entry"`
}

// ParseFeed reads RSS 2.0, RSS 1.0 or Atom and returns at most limit items,
// newest first when dates are known.
func ParseFeed(data []byte, limit int) (Feed, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	var root xml.StartElement
	for {
		tok, err := dec.Token()
		if err != nil {
			return Feed{}, ErrFormat
		}
		if se, ok := tok.(xml.StartElement); ok {
			root = se
			break
		}
	}

	var feed Feed
	switch strings.ToLower(root.Name.Local) {
	case "rss", "rdf":
		var doc rssDoc
		if err := dec.DecodeElement(&doc, &root); err != nil {
			return Feed{}, ErrFormat
		}
		feed.Title = clean(doc.Channel.Title)
		for _, it := range doc.Channel.Items {
			link := strings.TrimSpace(it.Link)
			if link == "" && strings.HasPrefix(it.GUID, "http") {
				link = strings.TrimSpace(it.GUID)
			}
			feed.Items = append(feed.Items, Item{Title: clean(it.Title), Link: link, Date: parseDate(it.PubDate, it.Date), Summary: summary(it.Description)})
		}
		for _, it := range doc.Items {
			feed.Items = append(feed.Items, Item{Title: clean(it.Title), Link: strings.TrimSpace(it.Link), Date: parseDate(it.Date), Summary: summary(it.Description)})
		}
	case "feed":
		var doc atomDoc
		if err := dec.DecodeElement(&doc, &root); err != nil {
			return Feed{}, ErrFormat
		}
		feed.Title = clean(doc.Title)
		for _, e := range doc.Entries {
			var link string
			for _, l := range e.Links {
				if l.Rel == "" || l.Rel == "alternate" {
					link = l.Href
					break
				}
			}
			s := e.Summary
			if s == "" {
				s = e.Content
			}
			feed.Items = append(feed.Items, Item{Title: clean(e.Title), Link: strings.TrimSpace(link), Date: parseDate(e.Published, e.Updated), Summary: summary(s)})
		}
	default:
		return Feed{}, ErrFormat
	}

	sort.SliceStable(feed.Items, func(i, j int) bool {
		a, b := feed.Items[i].Date, feed.Items[j].Date
		return a != nil && (b == nil || a.After(*b))
	})
	if limit > 0 && len(feed.Items) > limit {
		feed.Items = feed.Items[:limit]
	}
	if feed.Items == nil {
		feed.Items = []Item{}
	}
	return feed, nil
}

var dateLayouts = []string{
	time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC3339Nano,
	"Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST", "2 Jan 2006 15:04:05 -0700",
	"Mon, 02 Jan 2006 15:04 -0700", "2006-01-02T15:04:05", "2006-01-02",
}

// parseDate returns the first of values that parses as a date.
func parseDate(values ...string) *time.Time {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		for _, layout := range dateLayouts {
			if t, err := time.Parse(layout, v); err == nil {
				t = t.UTC()
				return &t
			}
		}
	}
	return nil
}

var tags = regexp.MustCompile(`(?s)<[^>]*>`)
var spaces = regexp.MustCompile(`\s+`)

// clean turns feed text (which may hold HTML and entities) into plain text.
func clean(s string) string {
	s = tags.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}

// summary is the first 200 characters of an entry's text.
func summary(s string) string {
	s = clean(s)
	if r := []rune(s); len(r) > 200 {
		s = strings.TrimSpace(string(r[:200])) + "…"
	}
	return s
}
