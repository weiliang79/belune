package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 30 * time.Second
	sendBufSize = 64
)

// Client represents a single WebSocket connection.
type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	userID        string
	send          chan OutboundMessage
	subscriptions map[string]struct{}
}

// NewClient creates a new WebSocket client.
func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		userID:        userID,
		send:          make(chan OutboundMessage, sendBufSize),
		subscriptions: make(map[string]struct{}),
	}
}

// ReadPump reads messages from the WebSocket connection.
// It handles subscribe/unsubscribe actions from the client.
func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		var msg InboundMessage
		err := wsjson.Read(ctx, c.conn, &msg)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				return
			}
			slog.Debug("ws: read error", "user", c.userID, "error", err)
			return
		}

		switch msg.Action {
		case "subscribe":
			if msg.Channel != "" {
				c.hub.Subscribe(c, msg.Channel)
				slog.Debug("ws: client subscribed", "user", c.userID, "channel", msg.Channel)
			}
		case "unsubscribe":
			if msg.Channel != "" {
				c.hub.Unsubscribe(c, msg.Channel)
				slog.Debug("ws: client unsubscribed", "user", c.userID, "channel", msg.Channel)
			}
		default:
			slog.Debug("ws: unknown action", "action", msg.Action)
		}
	}
}

// WritePump writes messages to the WebSocket connection.
func (c *Client) WritePump(ctx context.Context) {
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-c.send:
			if !ok {
				// Hub closed the channel.
				c.conn.Close(websocket.StatusNormalClosure, "")
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, writeWait)
			err := wsjson.Write(writeCtx, c.conn, msg)
			cancel()
			if err != nil {
				slog.Debug("ws: write error", "user", c.userID, "error", err)
				return
			}

		case <-pingTicker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				slog.Debug("ws: ping failed", "user", c.userID, "error", err)
				return
			}
		}
	}
}

// SendJSON sends a JSON message to this specific client.
func (c *Client) SendJSON(channel, event string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	select {
	case c.send <- OutboundMessage{Channel: channel, Event: event, Data: raw}:
	default:
	}
}
