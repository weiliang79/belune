package ws

import "encoding/json"

// InboundMessage is sent by the client to subscribe/unsubscribe from channels.
type InboundMessage struct {
	Action  string `json:"action"`  // "subscribe" or "unsubscribe"
	Channel string `json:"channel"` // e.g. "build-logs:{id}", "metrics:host"
}

// OutboundMessage is sent from server to client.
type OutboundMessage struct {
	Channel string          `json:"channel"`
	Event   string          `json:"event"`
	Data    json.RawMessage `json:"data"`
}
