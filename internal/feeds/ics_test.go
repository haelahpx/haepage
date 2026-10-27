package feeds

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func ics(events ...string) []byte {
	return []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" + strings.Join(events, "") + "END:VCALENDAR\r\n")
}

func ev(lines ...string) string {
	return "BEGIN:VEVENT\r\n" + strings.Join(lines, "\r\n") + "\r\nEND:VEVENT\r\n"
}

func titles(evs []Event) string {
	var s []string
	for _, e := range evs {
		s = append(s, e.Title+"@"+e.Start.UTC().Format("01-02T15:04"))
	}
	return strings.Join(s, " ")
}

func TestICSBasics(t *testing.T) {
	from := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 7)
	data := ics(
		ev("UID:1", "SUMMARY:Standup", "DTSTART:20261027T090000Z", "DTEND:20261027T091500Z"),
		ev("UID:2", "SUMMARY:Dentist\\, then lunch", "LOCATION:Main St", "DTSTART;TZID=Asia/Jakarta:20261028T140000", "DURATION:PT1H"),
		ev("UID:3", "SUMMARY:Holiday", "DTSTART;VALUE=DATE:20261030", "DTEND;VALUE=DATE:20261031"),
		ev("UID:4", "SUMMARY:Last month", "DTSTART:20260901T090000Z"),
		ev("UID:5", "SUMMARY:Cancelled", "STATUS:CANCELLED", "DTSTART:20261029T090000Z"),
		ev("UID:6", "SUMMARY:Folded line th", " at continues", "DTSTART:20261029T100000Z"),
		ev("UID:7", "SUMMARY:Outlook zone", `DTSTART;TZID="Tokyo Standard Time":20261029T180000`),
	)
	got, err := ParseICS(data, from, to, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "Standup@10-27T09:00 Dentist, then lunch@10-28T07:00 Outlook zone@10-29T09:00 Folded line that continues@10-29T10:00 Holiday@"
	if s := titles(got); !strings.HasPrefix(s, want) {
		t.Fatalf("got  %s\nwant %s…", s, want)
	}
	hol := got[len(got)-1]
	if !hol.AllDay || hol.Date != "2026-10-30" || hol.End.Sub(hol.Start) != 24*time.Hour {
		t.Errorf("all-day = %+v", hol)
	}
	if got[1].Location != "Main St" || got[1].End.Sub(got[1].Start) != time.Hour {
		t.Errorf("duration/location = %+v", got[1])
	}
	if _, err := ParseICS([]byte("not a calendar"), from, to, 0); !errors.Is(err, ErrFormat) {
		t.Errorf("bad input = %v", err)
	}
	if got, _ := ParseICS(data, from, to, 2); len(got) != 2 {
		t.Errorf("limit: %d", len(got))
	}
}

func TestICSRecurrence(t *testing.T) {
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		rule  []string
		count int
		first string
	}{
		"daily":            {[]string{"DTSTART:20261020T070000Z", "RRULE:FREQ=DAILY"}, 30, "11-01T07:00"},
		"every 2 days":     {[]string{"DTSTART:20261031T070000Z", "RRULE:FREQ=DAILY;INTERVAL=2"}, 15, "11-02T07:00"},
		"daily count":      {[]string{"DTSTART:20261029T070000Z", "RRULE:FREQ=DAILY;COUNT=5"}, 2, "11-01T07:00"},
		"daily until":      {[]string{"DTSTART:20261101T070000Z", "RRULE:FREQ=DAILY;UNTIL=20261103T070000Z"}, 3, "11-01T07:00"},
		"weekly":           {[]string{"DTSTART:20261006T180000Z", "RRULE:FREQ=WEEKLY"}, 4, "11-03T18:00"},
		"weekly mo we fr":  {[]string{"DTSTART:20261005T080000Z", "RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR"}, 13, "11-02T08:00"},
		"monthly by date":  {[]string{"DTSTART:20260115T120000Z", "RRULE:FREQ=MONTHLY"}, 1, "11-15T12:00"},
		"second tuesday":   {[]string{"DTSTART:20260113T190000Z", "RRULE:FREQ=MONTHLY;BYDAY=2TU"}, 1, "11-10T19:00"},
		"last friday":      {[]string{"DTSTART:20260130T170000Z", "RRULE:FREQ=MONTHLY;BYDAY=-1FR"}, 1, "11-27T17:00"},
		"monthly 31st":     {[]string{"DTSTART:20260131T090000Z", "RRULE:FREQ=MONTHLY"}, 0, ""},
		"bymonthday -1":    {[]string{"DTSTART:20260131T090000Z", "RRULE:FREQ=MONTHLY;BYMONTHDAY=-1"}, 1, "11-30T09:00"},
		"yearly birthday":  {[]string{"DTSTART;VALUE=DATE:19900314", "RRULE:FREQ=YEARLY"}, 0, ""},
		"yearly in window": {[]string{"DTSTART;VALUE=DATE:19901118", "RRULE:FREQ=YEARLY"}, 1, ""},
		"ended before":     {[]string{"DTSTART:20260101T090000Z", "RRULE:FREQ=WEEKLY;UNTIL=20260601T000000Z"}, 0, ""},
		"unsupported freq": {[]string{"DTSTART:20261105T090000Z", "RRULE:FREQ=MINUTELY"}, 1, "11-05T09:00"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseICS(ics(ev(append([]string{"UID:x", "SUMMARY:E"}, c.rule...)...)), from, to, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != c.count {
				t.Fatalf("%d occurrences, want %d: %s", len(got), c.count, titles(got))
			}
			if c.first != "" && got[0].Start.UTC().Format("01-02T15:04") != c.first {
				t.Errorf("first = %s, want %s", got[0].Start.UTC().Format("01-02T15:04"), c.first)
			}
		})
	}
}

func TestICSExceptions(t *testing.T) {
	from := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 3)
	data := ics(
		ev("UID:s", "SUMMARY:Sync", "DTSTART:20261026T100000Z", "RRULE:FREQ=DAILY", "EXDATE:20261103T100000Z"),
		// The 4 Nov occurrence was moved to 15:00.
		ev("UID:s", "SUMMARY:Sync (moved)", "RECURRENCE-ID:20261104T100000Z", "DTSTART:20261104T150000Z"),
	)
	got, _ := ParseICS(data, from, to, 0)
	if s := titles(got); s != "Sync@11-02T10:00 Sync (moved)@11-04T15:00" {
		t.Errorf("got %s", s)
	}
}

func TestICSWeeklyKeepsWallClockAcrossDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tzdata")
	}
	data := ics(ev("UID:d", "SUMMARY:Gym", "DTSTART;TZID=America/New_York:20261026T070000", "RRULE:FREQ=WEEKLY"))
	got, _ := ParseICS(data, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC), 0)
	if len(got) != 1 || got[0].Start.In(ny).Hour() != 7 {
		t.Errorf("after DST ends the event should still be 7:00 local: %v", got)
	}
}

func TestICSDuration(t *testing.T) {
	for v, want := range map[string]time.Duration{"PT1H30M": 90 * time.Minute, "P1D": 24 * time.Hour, "P1W": 7 * 24 * time.Hour, "P1DT2H": 26 * time.Hour, "PT45S": 45 * time.Second, "x": 0} {
		if got := icsDuration(v); got != want {
			t.Errorf("%s = %v, want %v", v, got, want)
		}
	}
}
