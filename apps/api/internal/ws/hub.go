package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
)

const maxConnectionsPerUser = 10

// Hub manages WebSocket clients and channel subscriptions.
type Hub struct {
	mu          sync.RWMutex
	clients     map[*Client]struct{}
	channels    map[string]map[*Client]struct{} // channel → clients
	userConns   map[string]int                  // userID → connection count

	registerCh   chan *Client
	unregisterCh chan *Client
	broadcastCh  chan broadcastMsg
}

type broadcastMsg struct {
	channel string
	event   string
	data    json.RawMessage
}

func NewHub() *Hub {
	return &Hub{
		clients:      make(map[*Client]struct{}),
		channels:     make(map[string]map[*Client]struct{}),
		userConns:    make(map[string]int),
		registerCh:   make(chan *Client, 64),
		unregisterCh: make(chan *Client, 64),
		broadcastCh:  make(chan broadcastMsg, 256),
	}
}

// Run processes register/unregister/broadcast events. Must be called in a goroutine.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for c := range h.clients {
				close(c.send)
			}
			h.clients = make(map[*Client]struct{})
			h.channels = make(map[string]map[*Client]struct{})
			h.userConns = make(map[string]int)
			h.mu.Unlock()
			return

		case c := <-h.registerCh:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.userConns[c.userID]++
			h.mu.Unlock()

		case c := <-h.unregisterCh:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				h.userConns[c.userID]--
				if h.userConns[c.userID] <= 0 {
					delete(h.userConns, c.userID)
				}
				// Remove from all channels
				for ch, subs := range h.channels {
					delete(subs, c)
					if len(subs) == 0 {
						delete(h.channels, ch)
					}
				}
				close(c.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcastCh:
			h.mu.RLock()
			subs := h.channels[msg.channel]
			for c := range subs {
				out := OutboundMessage{
					Channel: msg.channel,
					Event:   msg.event,
					Data:    msg.data,
				}
				select {
				case c.send <- out:
				default:
					slog.Debug("ws: dropping message for slow client", "channel", msg.channel)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(c *Client) bool {
	h.mu.RLock()
	count := h.userConns[c.userID]
	h.mu.RUnlock()
	if count >= maxConnectionsPerUser {
		return false
	}
	h.registerCh <- c
	return true
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c *Client) {
	h.unregisterCh <- c
}

// Subscribe adds a client to a channel.
func (h *Hub) Subscribe(c *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.channels[channel] == nil {
		h.channels[channel] = make(map[*Client]struct{})
	}
	h.channels[channel][c] = struct{}{}
	c.subscriptions[channel] = struct{}{}
}

// Unsubscribe removes a client from a channel.
func (h *Hub) Unsubscribe(c *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.channels[channel]; ok {
		delete(subs, c)
		if len(subs) == 0 {
			delete(h.channels, channel)
		}
	}
	delete(c.subscriptions, channel)
}

// Broadcast sends a message to all subscribers of a channel.
func (h *Hub) Broadcast(channel, event string, data json.RawMessage) {
	h.broadcastCh <- broadcastMsg{channel: channel, event: event, data: data}
}

// SubscriberCount returns the number of subscribers for a channel.
func (h *Hub) SubscriberCount(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.channels[channel])
}

// ActiveChannelsWithPrefix returns channel names that have at least one subscriber
// and start with the given prefix. Used by adapters that drive on-demand data collection.
func (h *Hub) ActiveChannelsWithPrefix(prefix string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []string
	for ch, subs := range h.channels {
		if len(subs) > 0 && len(ch) >= len(prefix) && ch[:len(prefix)] == prefix {
			out = append(out, ch)
		}
	}
	return out
}
