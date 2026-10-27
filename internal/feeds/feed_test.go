package feeds

import (
	"errors"
	"strings"
	"testing"
)

const rss2 = `<?xml version="1.0" encoding="ISO-8859-1"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/"><channel>
  <title>Home &amp; Lab</title>
  <item><title>Older post</title><link>https://blog.example/old</link><pubDate>Mon, 20 Oct 2026 08:00:00 +0000</pubDate></item>
  <item><title><![CDATA[New <b>post</b>]]></title><guid>https://blog.example/new</guid>
    <pubDate>Tue, 27 Oct 2026 09:30:00 GMT</pubDate>
    <description>&lt;p&gt;Hello &amp;amp; welcome&lt;/p&gt;</description></item>
  <item><title>No date</title><link>https://blog.example/x</link></item>
</channel></rss>`

const atom = `<feed xmlns="http://www.w3.org/2005/Atom"><title>Releases from app</title>
  <entry><title>v1.2.0</title><link rel="alternate" type="text/html" href="https://code.example/app/releases/tag/v1.2.0"/>
    <updated>2026-10-25T10:00:00Z</updated><content type="html">&lt;p&gt;Fixes&lt;/p&gt;</content></entry>
  <entry><title>v1.3.0</title><link href="https://code.example/app/releases/tag/v1.3.0"/><updated>2026-10-27T10:00:00+07:00</updated></entry>
</feed>`

const rdf = `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns="http://purl.org/rss/1.0/" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel><title>Old style</title></channel>
  <item><title>Entry</title><link>https://x.example/1</link><dc:date>2026-10-01T00:00:00Z</dc:date></item>
</rdf:RDF>`

func TestParseFeed(t *testing.T) {
	f, err := ParseFeed([]byte(rss2), 10)
	if err != nil {
		t.Fatal(err)
	}
	if f.Title != "Home & Lab" || len(f.Items) != 3 {
		t.Fatalf("feed = %+v", f)
	}
	first := f.Items[0]
	if first.Title != "New post" || first.Link != "https://blog.example/new" || first.Summary != "Hello & welcome" || first.Date.Hour() != 9 {
		t.Errorf("newest = %+v", first)
	}
	if f.Items[2].Title != "No date" || f.Items[2].Date != nil {
		t.Errorf("undated items go last: %+v", f.Items[2])
	}

	a, err := ParseFeed([]byte(atom), 1)
	if err != nil || len(a.Items) != 1 || a.Items[0].Title != "v1.3.0" || !strings.HasSuffix(a.Items[0].Link, "v1.3.0") {
		t.Errorf("atom = %+v, %v", a, err)
	}
	if a.Items[0].Date.Hour() != 3 {
		t.Errorf("atom date not in UTC: %v", a.Items[0].Date)
	}

	r, err := ParseFeed([]byte(rdf), 0)
	if err != nil || r.Title != "Old style" || len(r.Items) != 1 || r.Items[0].Date == nil {
		t.Errorf("rss 1.0 = %+v, %v", r, err)
	}

	for _, bad := range []string{"", "{}", "<html><body>hi</body></html>", "<?xml version='1.0'?><other/>"} {
		if _, err := ParseFeed([]byte(bad), 5); !errors.Is(err, ErrFormat) {
			t.Errorf("ParseFeed(%q) = %v", bad, err)
		}
	}
}

func TestSummaryIsShortPlainText(t *testing.T) {
	long := strings.Repeat("word ", 100)
	if s := summary("<script>x</script>" + long); len([]rune(s)) > 201 || strings.Contains(s, "<") {
		t.Errorf("summary = %q", s)
	}
}
