package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClient creates a test client not bound to a real WebSocket connection.
func fakeClient(hub *Hub, userID string) *Client {
	return &Client{
		hub:           hub,
		userID:        userID,
		send:          make(chan OutboundMessage, sendBufSize),
		subscriptions: make(map[string]struct{}),
	}
}

func startHub(t *testing.T) (*Hub, context.CancelFunc) {
	t.Helper()
	hub := NewHub(20)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	return hub, cancel
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	c := fakeClient(hub, "user-1")
	ok := hub.Register(c)
	require.True(t, ok)

	// Wait for registration to be processed
	time.Sleep(10 * time.Millisecond)

	hub.Unregister(c)
	time.Sleep(10 * time.Millisecond)

	// send channel should be closed after unregister
	_, open := <-c.send
	assert.False(t, open, "send channel should be closed")
}

func TestHub_SubscribeAndBroadcast(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	c1 := fakeClient(hub, "user-1")
	c2 := fakeClient(hub, "user-2")
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(10 * time.Millisecond)

	hub.Subscribe(c1, "test-channel")
	hub.Subscribe(c2, "test-channel")

	assert.Equal(t, 2, hub.SubscriberCount("test-channel"))

	// Broadcast message
	data, _ := json.Marshal("hello")
	hub.Broadcast("test-channel", "message", data)

	// Both clients should receive
	select {
	case msg := <-c1.send:
		assert.Equal(t, "test-channel", msg.Channel)
		assert.Equal(t, "message", msg.Event)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("c1 did not receive broadcast")
	}

	select {
	case msg := <-c2.send:
		assert.Equal(t, "test-channel", msg.Channel)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("c2 did not receive broadcast")
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	c := fakeClient(hub, "user-1")
	hub.Register(c)
	time.Sleep(10 * time.Millisecond)

	hub.Subscribe(c, "ch1")
	assert.Equal(t, 1, hub.SubscriberCount("ch1"))

	hub.Unsubscribe(c, "ch1")
	assert.Equal(t, 0, hub.SubscriberCount("ch1"))

	// Broadcast should not reach unsubscribed client
	data, _ := json.Marshal("nope")
	hub.Broadcast("ch1", "msg", data)

	select {
	case <-c.send:
		t.Fatal("should not receive message after unsubscribe")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestHub_MaxConnectionsPerUser(t *testing.T) {
	const testMax = 3
	hub := NewHub(testMax)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register exactly testMax clients — all should succeed
	for i := 0; i < testMax; i++ {
		c := fakeClient(hub, "user-1")
		ok := hub.Register(c)
		require.True(t, ok, "should allow connection %d", i)
	}
	time.Sleep(10 * time.Millisecond)

	// Next one should be rejected
	extra := fakeClient(hub, "user-1")
	ok := hub.Register(extra)
	assert.False(t, ok, "should reject connection over limit")
}

func TestHub_BroadcastToWrongChannel(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	c := fakeClient(hub, "user-1")
	hub.Register(c)
	time.Sleep(10 * time.Millisecond)

	hub.Subscribe(c, "ch-a")

	data, _ := json.Marshal("test")
	hub.Broadcast("ch-b", "msg", data)

	select {
	case <-c.send:
		t.Fatal("should not receive message from different channel")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}
