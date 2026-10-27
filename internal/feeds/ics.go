package feeds

import (
	"bufio"
	"bytes"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Event is one occurrence of a calendar event. All-day events carry the
// day as Date ("2026-10-27") and start at midnight in the server's zone.
type Event struct {
	Title    string    `json:"title"`
	Location string    `json:"location,omitempty"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	AllDay   bool      `json:"allDay,omitempty"`
	Date     string    `json:"date,omitempty"`
}

// maxSteps bounds recurrence expansion, so a rule like "every minute since
// 1970" can't stall the server.
const maxSteps = 20000

type prop struct {
	name   string
	params map[string]string
	value  string
}

type vevent struct {
	uid, summary, location, status string
	start, end                     time.Time
	allDay                         bool
	duration                       time.Duration
	rrule                          map[string]string
	exdates                        map[time.Time]bool
	recurrenceID                   *time.Time
}

// ParseICS reads an iCalendar file and returns the occurrences that overlap
// [from, to), soonest first, at most limit of them (0 for all).
func ParseICS(data []byte, from, to time.Time, limit int) ([]Event, error) {
	if !bytes.Contains(data, []byte("BEGIN:VCALENDAR")) {
		return nil, ErrFormat
	}
	var events []*vevent
	var cur *vevent
	for _, p := range props(data) {
		switch {
		case p.name == "BEGIN" && p.value == "VEVENT":
			cur = &vevent{exdates: map[time.Time]bool{}}
		case p.name == "END" && p.value == "VEVENT":
			if cur != nil && !cur.start.IsZero() {
				events = append(events, cur)
			}
			cur = nil
		case cur == nil:
		case p.name == "UID":
			cur.uid = p.value
		case p.name == "SUMMARY":
			cur.summary = unescape(p.value)
		case p.name == "LOCATION":
			cur.location = unescape(p.value)
		case p.name == "STATUS":
			cur.status = strings.ToUpper(p.value)
		case p.name == "DTSTART":
			cur.start, cur.allDay = icsTime(p)
		case p.name == "DTEND":
			cur.end, _ = icsTime(p)
		case p.name == "DURATION":
			cur.duration = icsDuration(p.value)
		case p.name == "RRULE":
			cur.rrule = map[string]string{}
			for _, part := range strings.Split(p.value, ";") {
				if k, v, ok := strings.Cut(part, "="); ok {
					cur.rrule[strings.ToUpper(k)] = strings.ToUpper(v)
				}
			}
		case p.name == "EXDATE":
			for _, v := range strings.Split(p.value, ",") {
				t, _ := icsTime(prop{params: p.params, value: v})
				cur.exdates[t.UTC()] = true
			}
		case p.name == "RECURRENCE-ID":
			t, _ := icsTime(p)
			cur.recurrenceID = &t
		}
	}

	// Single occurrences moved or cancelled ("RECURRENCE-ID") replace the
	// matching occurrence of their series.
	overridden := map[string]map[time.Time]bool{}
	for _, e := range events {
		if e.recurrenceID != nil {
			if overridden[e.uid] == nil {
				overridden[e.uid] = map[time.Time]bool{}
			}
			overridden[e.uid][e.recurrenceID.UTC()] = true
		}
	}

	var out []Event
	for _, e := range events {
		if e.status == "CANCELLED" {
			continue
		}
		length := e.length()
		for _, start := range e.occurrences(to) {
			end := start.Add(length)
			if e.allDay {
				end = start.AddDate(0, 0, max(1, int(length.Hours()/24+0.5)))
			}
			if e.exdates[start.UTC()] || (e.recurrenceID == nil && overridden[e.uid][start.UTC()]) {
				continue
			}
			if !end.After(from) || !start.Before(to) {
				continue
			}
			ev := Event{Title: e.summary, Location: e.location, Start: start, End: end, AllDay: e.allDay}
			if e.allDay {
				ev.Date = start.Format("2006-01-02")
			}
			out = append(out, ev)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []Event{}
	}
	return out, nil
}

func (e *vevent) length() time.Duration {
	switch {
	case !e.end.IsZero() && e.end.After(e.start):
		return e.end.Sub(e.start)
	case e.duration > 0:
		return e.duration
	case e.allDay:
		return 24 * time.Hour
	}
	return 0
}

// occurrences lists the start times of the event up to `to`, following its
// RRULE when it has one (RFC 5545 §3.3.10, the common parts).
func (e *vevent) occurrences(to time.Time) []time.Time {
	if e.rrule == nil {
		return []time.Time{e.start}
	}
	r := e.rrule
	interval, _ := strconv.Atoi(r["INTERVAL"])
	interval = max(interval, 1)
	count, _ := strconv.Atoi(r["COUNT"])
	until := to
	if u, ok := r["UNTIL"]; ok {
		t, _ := icsTime(prop{value: u, params: map[string]string{}})
		if e.allDay || len(u) == 8 {
			t = t.AddDate(0, 0, 1).Add(-time.Nanosecond) // the whole last day
		}
		if !t.IsZero() && t.Before(until) {
			until = t
		}
	}
	byday := parseByDay(r["BYDAY"])
	var monthDays []int
	for _, d := range strings.Split(r["BYMONTHDAY"], ",") {
		if n, err := strconv.Atoi(d); err == nil && n != 0 {
			monthDays = append(monthDays, n)
		}
	}

	var out []time.Time
	emitted := 0
	emit := func(t time.Time) bool { // false: stop
		if t.Before(e.start) {
			return true
		}
		if t.After(until) || (count > 0 && emitted >= count) {
			return false
		}
		emitted++
		out = append(out, t)
		return true
	}
	at := func(day time.Time) time.Time { // day's date at the event's wall-clock time
		h, m, s := e.start.Clock()
		return time.Date(day.Year(), day.Month(), day.Day(), h, m, s, 0, e.start.Location())
	}

	for step := 0; step < maxSteps; step++ {
		var batch []time.Time
		var period time.Time // where this step's period starts
		switch r["FREQ"] {
		case "DAILY":
			period = e.start.AddDate(0, 0, step*interval)
			batch = []time.Time{period}
		case "WEEKLY":
			// Weeks start on Monday (WKST=MO, the default).
			offset := (int(e.start.Weekday()) + 6) % 7
			period = at(e.start.AddDate(0, 0, -offset+7*step*interval))
			if len(byday) == 0 {
				batch = []time.Time{e.start.AddDate(0, 0, 7*step*interval)}
				break
			}
			for _, d := range byday {
				batch = append(batch, at(period.AddDate(0, 0, (int(d.day)+6)%7)))
			}
		case "MONTHLY":
			first := time.Date(e.start.Year(), e.start.Month()+time.Month(step*interval), 1, 0, 0, 0, 0, e.start.Location())
			period = first
			switch {
			case len(byday) > 0:
				for _, d := range byday {
					if day, ok := nthWeekday(first, d); ok {
						batch = append(batch, at(day))
					}
				}
			case len(monthDays) > 0:
				for _, n := range monthDays {
					if day, ok := monthDay(first, n); ok {
						batch = append(batch, at(day))
					}
				}
			default:
				if day, ok := monthDay(first, e.start.Day()); ok {
					batch = append(batch, at(day))
				}
			}
		case "YEARLY":
			t := e.start.AddDate(step*interval, 0, 0)
			period = time.Date(e.start.Year()+step*interval, 1, 1, 0, 0, 0, 0, e.start.Location())
			if t.Day() == e.start.Day() { // skip Feb 29 in other years
				batch = []time.Time{t}
			}
		default:
			return []time.Time{e.start}
		}
		sort.Slice(batch, func(i, j int) bool { return batch[i].Before(batch[j]) })
		for _, t := range batch {
			if !emit(t) {
				return out
			}
		}
		if period.After(until) {
			return out
		}
	}
	return out
}

type weekday struct {
	n   int // 0: every; 1: first; -1: last
	day time.Weekday
}

var dayCodes = map[string]time.Weekday{"SU": time.Sunday, "MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday, "TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday}

func parseByDay(s string) []weekday {
	var out []weekday
	for _, part := range strings.Split(s, ",") {
		if len(part) < 2 {
			continue
		}
		d, ok := dayCodes[part[len(part)-2:]]
		if !ok {
			continue
		}
		n, _ := strconv.Atoi(part[:len(part)-2])
		out = append(out, weekday{n: n, day: d})
	}
	return out
}

// nthWeekday finds e.g. the second Tuesday (n=2) or last Friday (n=-1) of
// first's month. n=0 is taken as the first.
func nthWeekday(first time.Time, d weekday) (time.Time, bool) {
	if d.n >= 0 {
		day := first.AddDate(0, 0, (int(d.day)-int(first.Weekday())+7)%7+7*max(d.n-1, 0))
		return day, day.Month() == first.Month()
	}
	last := first.AddDate(0, 1, -1)
	day := last.AddDate(0, 0, -((int(last.Weekday())-int(d.day)+7)%7)+7*(d.n+1))
	return day, day.Month() == first.Month()
}

// monthDay is day n of first's month (negative counts from the end).
func monthDay(first time.Time, n int) (time.Time, bool) {
	var day time.Time
	if n > 0 {
		day = first.AddDate(0, 0, n-1)
	} else {
		day = first.AddDate(0, 1, n)
	}
	return day, day.Month() == first.Month()
}

// props unfolds lines and splits each into name, parameters and value.
func props(data []byte) []prop {
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(lines) > 0 {
			lines[len(lines)-1] += line[1:]
			continue
		}
		lines = append(lines, line)
	}
	out := make([]prop, 0, len(lines))
	for _, line := range lines {
		head, value, ok := cutUnquoted(line, ':')
		if !ok {
			continue
		}
		parts := strings.Split(head, ";")
		p := prop{name: strings.ToUpper(parts[0]), params: map[string]string{}, value: value}
		for _, param := range parts[1:] {
			if k, v, ok := strings.Cut(param, "="); ok {
				p.params[strings.ToUpper(k)] = strings.Trim(v, `"`)
			}
		}
		out = append(out, p)
	}
	return out
}

// cutUnquoted splits at the first sep outside double quotes, since
// parameter values like TZID="a:b" may contain it.
func cutUnquoted(s string, sep byte) (string, string, bool) {
	quoted := false
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '"':
			quoted = !quoted
		case s[i] == sep && !quoted:
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// icsTime reads DATE ("20261027"), UTC ("…T090000Z"), zoned (TZID=…) or
// floating date-times. Floating times and unknown zones use the server's
// local zone.
func icsTime(p prop) (time.Time, bool) {
	v := strings.TrimSpace(p.value)
	loc := time.Local
	if tz := p.params["TZID"]; tz != "" {
		if l, err := time.LoadLocation(windowsZones[tz]); err == nil && windowsZones[tz] != "" {
			loc = l
		} else if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	if len(v) == 8 || p.params["VALUE"] == "DATE" {
		t, err := time.ParseInLocation("20060102", v[:min(8, len(v))], time.Local)
		if err != nil {
			return time.Time{}, true
		}
		return t, true
	}
	if strings.HasSuffix(v, "Z") {
		t, _ := time.Parse("20060102T150405Z", v)
		return t, false
	}
	t, _ := time.ParseInLocation("20060102T150405", v, loc)
	return t, false
}

// Outlook writes Windows zone names; map the common ones.
var windowsZones = map[string]string{
	"Pacific Standard Time": "America/Los_Angeles", "Mountain Standard Time": "America/Denver",
	"Central Standard Time": "America/Chicago", "Eastern Standard Time": "America/New_York",
	"GMT Standard Time": "Europe/London", "W. Europe Standard Time": "Europe/Berlin",
	"Romance Standard Time": "Europe/Paris", "Central Europe Standard Time": "Europe/Budapest",
	"India Standard Time": "Asia/Kolkata", "China Standard Time": "Asia/Shanghai",
	"Tokyo Standard Time": "Asia/Tokyo", "SE Asia Standard Time": "Asia/Bangkok",
	"Singapore Standard Time": "Asia/Singapore", "AUS Eastern Standard Time": "Australia/Sydney",
	"UTC": "UTC",
}

// icsDuration reads durations like PT1H30M or P1D.
func icsDuration(v string) time.Duration {
	v = strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(v)), "+")
	if !strings.HasPrefix(v, "P") {
		return 0
	}
	var d time.Duration
	num := 0
	inTime := false
	for _, c := range v[1:] {
		switch {
		case c >= '0' && c <= '9':
			num = num*10 + int(c-'0')
		case c == 'T':
			inTime = true
		case c == 'W':
			d += time.Duration(num) * 7 * 24 * time.Hour
			num = 0
		case c == 'D':
			d += time.Duration(num) * 24 * time.Hour
			num = 0
		case c == 'H' && inTime:
			d += time.Duration(num) * time.Hour
			num = 0
		case c == 'M' && inTime:
			d += time.Duration(num) * time.Minute
			num = 0
		case c == 'S' && inTime:
			d += time.Duration(num) * time.Second
			num = 0
		}
	}
	return d
}

func unescape(s string) string {
	return strings.NewReplacer(`\n`, " ", `\N`, " ", `\,`, ",", `\;`, ";", `\\`, `\`).Replace(s)
}
