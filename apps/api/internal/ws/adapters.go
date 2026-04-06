package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/v9"
)

// RedisAdapter bridges Redis pub/sub channels to the WebSocket hub.
// Each adapter subscribes to a Redis channel (or pattern) and broadcasts
// received messages to the corresponding WebSocket channel.
type RedisAdapter struct {
	rdb *redis.Client
	hub *Hub
}

func NewRedisAdapter(rdb *redis.Client, hub *Hub) *RedisAdapter {
	return &RedisAdapter{rdb: rdb, hub: hub}
}

// RunBuildLogAdapter forwards build-logs:{deploymentID} messages.
func (a *RedisAdapter) RunBuildLogAdapter(ctx context.Context) {
	pubsub := a.rdb.PSubscribe(ctx, "build-logs:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			// Redis channel: "build-logs:{deploymentID}"
			// WS channel:    "build-logs:{deploymentID}"
			wsChannel := msg.Channel
			data := json.RawMessage(`"` + escapeJSON(msg.Payload) + `"`)
			a.hub.Broadcast(wsChannel, "log", data)
		}
	}
}

// RunHostMetricsAdapter forwards host:metrics:live messages.
func (a *RedisAdapter) RunHostMetricsAdapter(ctx context.Context) {
	pubsub := a.rdb.Subscribe(ctx, "host:metrics:live")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			a.hub.Broadcast("metrics:host", "metrics", json.RawMessage(msg.Payload))
		}
	}
}

// RunRequestLogAdapter forwards requests:live:{appID} messages.
func (a *RedisAdapter) RunRequestLogAdapter(ctx context.Context) {
	pubsub := a.rdb.PSubscribe(ctx, "requests:live:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			// Redis channel: "requests:live:{appID}"
			// WS channel:    "requests:{appID}"
			parts := strings.TrimPrefix(msg.Channel, "requests:live:")
			if parts != "" {
				wsChannel := "requests:" + parts
				a.hub.Broadcast(wsChannel, "request", json.RawMessage(msg.Payload))
			}
			// Also broadcast to global requests channel
			a.hub.Broadcast("requests:all", "request", json.RawMessage(msg.Payload))
		}
	}
}

// RunAppLogAdapter forwards app-logs:{applicationID} messages from Redis.
// The log collector publishes to these channels.
func (a *RedisAdapter) RunAppLogAdapter(ctx context.Context) {
	pubsub := a.rdb.PSubscribe(ctx, "app-logs:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			// Redis channel: "app-logs:{appID}"
			// WS channel:    "app-logs:{appID}"
			a.hub.Broadcast(msg.Channel, "log", json.RawMessage(msg.Payload))
		}
	}
}

// escapeJSON escapes a string for safe embedding in a JSON string value.
func escapeJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return s
	}
	// json.Marshal wraps in quotes; strip them
	return string(b[1 : len(b)-1])
}

// ContainerStatusBroadcaster provides a method for the event watcher to push status updates.
type ContainerStatusBroadcaster struct {
	hub *Hub
}

func NewContainerStatusBroadcaster(hub *Hub) *ContainerStatusBroadcaster {
	return &ContainerStatusBroadcaster{hub: hub}
}

// BroadcastStatus sends a container status update to subscribers.
func (b *ContainerStatusBroadcaster) BroadcastStatus(applicationID, status string) {
	data, _ := json.Marshal(map[string]string{
		"application_id": applicationID,
		"status":         status,
	})
	channel := "container-status:" + applicationID
	b.hub.Broadcast(channel, "status", data)

	slog.Debug("ws: broadcast container status", "app_id", applicationID, "status", status)
}
