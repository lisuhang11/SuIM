package ws

import "sync"

// Hub keeps the websocket connections on this process.
type Hub struct {
	mu    sync.RWMutex
	users map[string]map[int][]*Client
	watch map[string]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		users: map[string]map[int][]*Client{},
		watch: map[string]map[*Client]struct{}{},
	}
}

// Add stores the client and returns older connections on the same platform.
// Same-token connections are replaced quietly. A different token is a kick.
func (h *Hub) Add(c *Client) (quiet, kicked []*Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	platforms := h.users[c.userID]
	if platforms == nil {
		platforms = map[int][]*Client{}
		h.users[c.userID] = platforms
	}
	for _, old := range platforms[c.platformID] {
		if old == c {
			continue
		}
		if old.token == c.token {
			quiet = append(quiet, old)
		} else {
			kicked = append(kicked, old)
		}
	}
	platforms[c.platformID] = []*Client{c}
	return quiet, kicked
}

func (h *Hub) Remove(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	platforms := h.users[c.userID]
	list := platforms[c.platformID]
	kept := list[:0]
	for _, item := range list {
		if item != c {
			kept = append(kept, item)
		}
	}
	if len(kept) == 0 {
		delete(platforms, c.platformID)
	} else {
		platforms[c.platformID] = kept
	}
	if len(platforms) == 0 {
		delete(h.users, c.userID)
	}
	h.dropWatchLocked(c)
}

func (h *Hub) Clients(userID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*Client
	for _, list := range h.users[userID] {
		out = append(out, list...)
	}
	return out
}

func (h *Hub) Platforms(userID string) []int32 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []int32
	for platformID := range h.users[userID] {
		out = append(out, int32(platformID))
	}
	return out
}

func (h *Hub) ConnCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, platforms := range h.users {
		for _, list := range platforms {
			n += len(list)
		}
	}
	return n
}

func (h *Hub) Users(ids []string) map[string][]*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := map[string][]*Client{}
	if len(ids) == 0 {
		for userID, platforms := range h.users {
			out[userID] = clientsOf(platforms)
		}
		return out
	}
	for _, userID := range ids {
		if platforms, ok := h.users[userID]; ok {
			out[userID] = clientsOf(platforms)
		}
	}
	return out
}

func (h *Hub) SetSubs(c *Client, subscribe, unsubscribe []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.subs == nil {
		c.subs = map[string]struct{}{}
	}
	for _, userID := range unsubscribe {
		delete(c.subs, userID)
		if set := h.watch[userID]; set != nil {
			delete(set, c)
			if len(set) == 0 {
				delete(h.watch, userID)
			}
		}
	}
	for _, userID := range subscribe {
		if userID == "" {
			continue
		}
		c.subs[userID] = struct{}{}
		if h.watch[userID] == nil {
			h.watch[userID] = map[*Client]struct{}{}
		}
		h.watch[userID][c] = struct{}{}
	}
}

func (h *Hub) Watchers(userID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	set := h.watch[userID]
	out := make([]*Client, 0, len(set))
	for client := range set {
		out = append(out, client)
	}
	return out
}

func (h *Hub) dropWatchLocked(c *Client) {
	for userID := range c.subs {
		set := h.watch[userID]
		delete(set, c)
		if len(set) == 0 {
			delete(h.watch, userID)
		}
	}
	c.subs = nil
}

func clientsOf(platforms map[int][]*Client) []*Client {
	var out []*Client
	for _, list := range platforms {
		out = append(out, list...)
	}
	return out
}
