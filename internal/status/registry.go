package status

import (
	"sync"
	"time"
)

type Stream struct {
	Channel       string
	Stage         string
	Online        bool
	StreamURL     string
	ListenerCount int
	UpdatedAt     time.Time
}

// Registry tracks the last-known state of every monitored channel so the UI can
// render online and offline streams alike.
type Registry struct {
	mu    sync.RWMutex
	order []string
	items map[string]*Stream
}

func New(channels []string) *Registry {
	r := &Registry{
		order: append([]string(nil), channels...),
		items: make(map[string]*Stream, len(channels)),
	}
	now := time.Now()
	for _, c := range channels {
		r.items[c] = &Stream{Channel: c, Stage: c, UpdatedAt: now}
	}
	return r
}

func (r *Registry) Set(channel, stage string, online bool, listeners int, streamURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.items[channel]
	if !ok {
		s = &Stream{Channel: channel}
		r.items[channel] = s
		r.order = append(r.order, channel)
	}
	if stage != "" {
		s.Stage = stage
	}
	s.Online = online
	if online {
		s.StreamURL = streamURL
	} else {
		s.StreamURL = ""
	}
	s.ListenerCount = listeners
	s.UpdatedAt = time.Now()
}

// All returns a snapshot of all streams in channel order.
func (r *Registry) All() []Stream {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Stream, 0, len(r.order))
	for _, c := range r.order {
		if s, ok := r.items[c]; ok {
			out = append(out, *s)
		}
	}
	return out
}
