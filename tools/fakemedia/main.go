// Command fakemedia runs fake Jellyfin, Jellyseerr, Radarr and Sonarr APIs,
// so Haepage's media integrations can be tried without installing them.
// Every 20 seconds something happens: downloads progress, new requests
// come in, someone starts watching.
//
//	go run ./tools/fakemedia            # then add the services it prints
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/haelahpx/haepage/internal/integrations/fakeapps"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8095", "listen address")
	flag.Parse()

	apps := fakeapps.Demo(time.Now())
	go simulate(apps)

	base := "http://" + *addr
	fmt.Printf(`Fake media apps on %s (API key for all: %s)

Add these to the "services" list in haepage.json, then Connect each one:

  {"id": "fake-jellyfin", "name": "Jellyfin", "group": "media", "integration": "jellyfin", "urls": {"lan": "%[1]s/jellyfin"}},
  {"id": "fake-seerr",    "name": "Requests", "group": "media", "integration": "seerr",    "urls": {"lan": "%[1]s/seerr"}},
  {"id": "fake-radarr",   "name": "Radarr",   "group": "downloads", "integration": "radarr", "urls": {"lan": "%[1]s/radarr"}},
  {"id": "fake-sonarr",   "name": "Sonarr",   "group": "downloads", "integration": "sonarr", "urls": {"lan": "%[1]s/sonarr"}}

`, base, fakeapps.Key)
	log.Fatal(http.ListenAndServe(*addr, apps.Handler()))
}

var titles = []string{"Paper Moon", "Night Train", "Glass Garden", "Cold Harbor", "Silver Line", "Wild Coast"}

func simulate(apps *fakeapps.Apps) {
	nextID := 100
	for i := 0; ; i++ {
		time.Sleep(20 * time.Second)
		apps.Lock()
		for _, st := range []*fakeapps.AppState{&apps.Radarr, &apps.Sonarr} {
			kept := st.Queue[:0]
			for _, q := range st.Queue {
				q.LeftMB -= q.SizeMB / 5
				if q.LeftMB > 0 {
					kept = append(kept, q)
				}
			}
			st.Queue = kept
		}
		title := titles[i%len(titles)]
		switch i % 3 {
		case 0:
			nextID++
			apps.Requests = append(apps.Requests, fakeapps.Request{ID: nextID, Type: "movie", TmdbID: 3000 + nextID, Title: title, RequestedBy: "Jo", CreatedAt: time.Now()})
		case 1:
			nextID++
			apps.Radarr.Queue = append(apps.Radarr.Queue, fakeapps.QueueItem{ID: nextID, Title: title, SizeMB: 6000, LeftMB: 6000, TimeLeft: "00:20:00"})
		case 2:
			nextID++
			apps.Sessions = append(apps.Sessions, fakeapps.Session{ID: fmt.Sprint("s", nextID), User: "Kim", Client: "Jellyfin Web", Device: "Laptop", ItemID: fmt.Sprint("i", nextID), Name: title, Year: 2023, Type: "Movie", PositionMin: 5, RuntimeMin: 104})
			if len(apps.Sessions) > 3 {
				apps.Sessions = apps.Sessions[1:]
			}
		}
		apps.Unlock()
	}
}
