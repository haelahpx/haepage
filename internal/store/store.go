// Package store holds the merged service list in memory and fans changes
// out to subscribers (the SSE connections).
package store

import (
	"reflect"
	"sort"
	"sync"
	"time"
)

// URLs mirror config.URLs; "{host}" stands for the host the visitor used.
type URLs struct {
	LAN       string `json:"lan,omitempty"`
	Tailscale string `json:"tailscale,omitempty"`
	Public    string `json:"public,omitempty"`
}

// StoppedReason is the health reason for a container that isn't running.
const StoppedReason = "container is not running"

type Health struct {
	Status      string     `json:"status"` // up, down, unknown
	MS          int64      `json:"ms,omitempty"`
	Reason      string     `json:"reason,omitempty"`
	CertExpires *time.Time `json:"certExpires,omitempty"` // https services: the certificate's end date
}

// Service is the merged view the browser receives.
type Service struct {
	ID          string  `json:"id"`     // identity key, e.g. "docker:media/app" or "manual:router"
	Source      string  `json:"source"` // manual, docker, port
	State       string  `json:"state"`  // detected, pinned, hidden
	Name        string  `json:"name"`
	Icon        string  `json:"icon,omitempty"`
	Group       string  `json:"group,omitempty"`
	Suggested   string  `json:"suggestedGroup,omitempty"` // where "accept" puts a new service
	Description string  `json:"description,omitempty"`
	URLs        URLs    `json:"urls"`
	IsNew       bool    `json:"isNew,omitempty"`
	Running     *bool   `json:"running,omitempty"`     // containers only
	Integration string  `json:"integration,omitempty"` // built-in integration kind, e.g. "jellyfin"
	NoCheck     bool    `json:"noCheck,omitempty"`     // health checks are turned off
	Health      *Health `json:"health,omitempty"`

	// Check settings stay on the server.
	CheckURL      string `json:"-"`
	CheckType     string `json:"-"`
	CheckExpect   int    `json:"-"`
	CheckInterval int    `json:"-"`
	CheckInsecure bool   `json:"-"`
}

// Event is one message for subscribers.
type Event struct {
	Type string
	Data any
}

type subscriber struct {
	ch chan Event
}

// Registry is the current service list. Every change is published as an
// event; subscribers that fall behind are dropped and must reconnect.
type Registry struct {
	mu           sync.RWMutex
	services     map[string]Service
	order        []string
	subs         map[*subscriber]struct{}
	integrations map[string]any // latest integration state per service
	activity     []any          // newest last, at most activityLimit
}

const activityLimit = 100

const subscriberBuffer = 64

func NewRegistry() *Registry {
	return &Registry{services: map[string]Service{}, subs: map[*subscriber]struct{}{}, integrations: map[string]any{}}
}

// Subscribe returns a channel of events and a function to stop. The
// channel is closed if the subscriber falls too far behind.
func (r *Registry) Subscribe() (<-chan Event, func()) {
	s := &subscriber{ch: make(chan Event, subscriberBuffer)}
	r.mu.Lock()
	r.subs[s] = struct{}{}
	r.mu.Unlock()
	return s.ch, func() {
		r.mu.Lock()
		if _, ok := r.subs[s]; ok {
			delete(r.subs, s)
			close(s.ch)
		}
		r.mu.Unlock()
	}
}

// Subscribers reports how many subscribers are connected.
func (r *Registry) Subscribers() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.subs)
}

// publishLocked sends without blocking; a full subscriber is dropped.
func (r *Registry) publishLocked(e Event) {
	for s := range r.subs {
		select {
		case s.ch <- e:
		default:
			delete(r.subs, s)
			close(s.ch)
		}
	}
}

// Publish sends an event that isn't a service change (e.g. config).
func (r *Registry) Publish(e Event) {
	r.mu.Lock()
	r.publishLocked(e)
	r.mu.Unlock()
}

// Set replaces the service list, keeping known health, and publishes the
// differences.
func (r *Registry) Set(list []Service) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := make(map[string]Service, len(list))
	order := make([]string, 0, len(list))
	for _, s := range list {
		// Keep the last check result, except a "stopped" result once the
		// container runs again: that one is out of date.
		if old, ok := r.services[s.ID]; ok && s.Health == nil && old.Health != nil &&
			!(old.Health.Reason == StoppedReason && s.Running != nil && *s.Running) {
			s.Health = old.Health
		}
		next[s.ID] = s
		order = append(order, s.ID)
	}
	for id := range r.services {
		if _, ok := next[id]; !ok {
			r.publishLocked(Event{Type: "service.remove", Data: map[string]string{"id": id}})
		}
	}
	for _, id := range order {
		if old, ok := r.services[id]; !ok || !reflect.DeepEqual(old, next[id]) {
			r.publishLocked(Event{Type: "service.upsert", Data: next[id]})
		}
	}
	r.services, r.order = next, order
}

// SetHealth records a check result and publishes it if it changed.
func (r *Registry) SetHealth(id string, h Health) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.services[id]
	if !ok {
		return
	}
	if s.Health != nil && s.Health.Status == h.Status && bucket(s.Health.MS) == bucket(h.MS) && sameTime(s.Health.CertExpires, h.CertExpires) {
		return
	}
	s.Health = &h
	r.services[id] = s
	r.publishLocked(Event{Type: "health", Data: map[string]any{"id": id, "health": h}})
}

func sameTime(a, b *time.Time) bool {
	return (a == nil) == (b == nil) && (a == nil || a.Equal(*b))
}

// bucket groups latencies so small changes don't flood subscribers.
func bucket(ms int64) int {
	switch {
	case ms < 100:
		return 0
	case ms < 300:
		return 1
	case ms < 1000:
		return 2
	}
	return 3
}

// SetIntegration records the latest state of a service's integration and
// publishes it. A nil state removes it (disconnected).
func (r *Registry) SetIntegration(id string, state any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if state == nil {
		delete(r.integrations, id)
		r.publishLocked(Event{Type: "integration.remove", Data: map[string]string{"service": id}})
		return
	}
	r.integrations[id] = state
	r.publishLocked(Event{Type: "integration", Data: state})
}

// AddActivity appends an activity event (a new request, a finished
// download, ...) and publishes it.
func (r *Registry) AddActivity(a any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activity = append(r.activity, a)
	if len(r.activity) > activityLimit {
		r.activity = r.activity[len(r.activity)-activityLimit:]
	}
	r.publishLocked(Event{Type: "activity", Data: a})
}

// State is everything a new subscriber needs: services, integration
// states and recent activity.
func (r *Registry) State() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	services := make([]Service, 0, len(r.order))
	for _, id := range r.order {
		services = append(services, r.services[id])
	}
	integrations := make([]any, 0, len(r.integrations))
	for _, v := range r.integrations {
		integrations = append(integrations, v)
	}
	return map[string]any{"services": services, "integrations": integrations, "activity": append([]any{}, r.activity...)}
}

// Snapshot returns the services in list order.
func (r *Registry) Snapshot() []Service {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Service, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.services[id])
	}
	return out
}

// Get returns one service.
func (r *Registry) Get(id string) (Service, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.services[id]
	return s, ok
}

// SortByName orders services by name, for stable output.
func SortByName(list []Service) {
	sort.SliceStable(list, func(i, j int) bool { return list[i].Name < list[j].Name })
}
