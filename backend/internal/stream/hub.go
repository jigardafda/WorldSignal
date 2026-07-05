// Package stream provides an in-process fan-out notifier that wakes streaming
// (SSE/WebSocket) connections when a new delivery lands for their subscription.
//
// The hub carries no event data — it is a pure wakeup. Delivery rows are the
// durable source of truth; on wake, a connection re-queries the rows it hasn't
// seen (by cursor), so a coalesced or missed notification never loses an event.
// Notifications are non-blocking and coalescing: a busy connection sees at most
// one pending wakeup.
//
// Scope: in-process, so it works when the API and workers run in one process
// (ROLE=all). A split api/worker deployment would need a Postgres LISTEN/NOTIFY
// bridge; streaming still functions there via each connection's periodic
// re-query fallback, just at poll latency.
package stream

import "sync"

// Hub fans out per-subscription wakeups to subscribed connections.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan struct{}]struct{}
}

// NewHub returns an empty hub ready to use.
func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[chan struct{}]struct{})}
}

// Subscribe registers interest in a subscription and returns a wakeup channel
// plus an unsubscribe func the caller MUST invoke when done. The channel has a
// one-slot buffer so notifications coalesce.
func (h *Hub) Subscribe(subID string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	m := h.subs[subID]
	if m == nil {
		m = make(map[chan struct{}]struct{})
		h.subs[subID] = m
	}
	m[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			if m := h.subs[subID]; m != nil {
				delete(m, ch)
				if len(m) == 0 {
					delete(h.subs, subID)
				}
			}
			h.mu.Unlock()
		})
	}
}

// Notify wakes every current subscriber of subID. Non-blocking: a subscriber
// that already has a pending wakeup is skipped (the wakeup coalesces).
func (h *Hub) Notify(subID string) {
	h.mu.Lock()
	m := h.subs[subID]
	chans := make([]chan struct{}, 0, len(m))
	for ch := range m {
		chans = append(chans, ch)
	}
	h.mu.Unlock()

	for _, ch := range chans {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
