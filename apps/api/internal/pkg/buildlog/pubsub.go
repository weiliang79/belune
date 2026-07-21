package buildlog

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

const channelPrefix = "build-logs:"

// DoneSentinel is published to the channel to signal end-of-stream. It is an
// exact non-JSON payload so consumers (the WS adapter, the SSE subscriber) can
// distinguish it from an NDJSON log entry.
const DoneSentinel = "__done__"

// Publisher publishes build log lines to a Redis Pub/Sub channel.
type Publisher struct {
	rdb     *redis.Client
	channel string
}

func NewPublisher(rdb *redis.Client, deploymentID string) *Publisher {
	return &Publisher{
		rdb:     rdb,
		channel: fmt.Sprintf("%s%s", channelPrefix, deploymentID),
	}
}

// enabled reports whether there is anywhere to publish to. Streaming build logs
// live is best-effort telemetry layered on top of the deploy — the durable copy
// is written to deployments.build_logs regardless — so a missing Redis client
// must degrade to silence rather than take down the deploy that is producing
// the logs. Before this, a nil client panicked inside the build/pull path and
// killed the whole task.
func (p *Publisher) enabled() bool {
	return p != nil && p.rdb != nil
}

// Publish sends a single log line to the channel. No-op when disabled.
func (p *Publisher) Publish(ctx context.Context, line string) error {
	if !p.enabled() {
		return nil
	}
	return p.rdb.Publish(ctx, p.channel, line).Err()
}

// Close publishes the done sentinel to signal end of stream. No-op when
// disabled — with nothing subscribed there is no stream to terminate.
func (p *Publisher) Close(ctx context.Context) error {
	if !p.enabled() {
		return nil
	}
	return p.rdb.Publish(ctx, p.channel, DoneSentinel).Err()
}

// Subscriber subscribes to build log lines from a Redis Pub/Sub channel.
//
// Lifecycle: Channel() spawns one goroutine that drains the underlying Redis
// pubsub connection until ctx is cancelled, the connection closes, or the
// done sentinel arrives. The goroutine itself releases the Redis subscription
// on exit, so even if a caller forgets to defer Close() the connection is not
// leaked. Close() is safe to call multiple times.
type Subscriber struct {
	rdb       *redis.Client
	pubsub    *redis.PubSub
	closeOnce sync.Once
}

func NewSubscriber(rdb *redis.Client, deploymentID string) *Subscriber {
	channel := fmt.Sprintf("%s%s", channelPrefix, deploymentID)
	return &Subscriber{
		rdb:    rdb,
		pubsub: rdb.Subscribe(context.Background(), channel),
	}
}

// Channel returns a channel that emits log lines until the done sentinel is received.
func (s *Subscriber) Channel(ctx context.Context) <-chan string {
	out := make(chan string, 64)
	go func() {
		// Releasing the redis subscription from inside the goroutine guarantees
		// it happens on every exit path (ctx cancel, sentinel, channel close).
		defer s.Close()
		defer close(out)
		ch := s.pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if msg.Payload == DoneSentinel {
					return
				}
				select {
				case out <- msg.Payload:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

// Close unsubscribes and closes the underlying pub/sub connection. Idempotent.
func (s *Subscriber) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.pubsub.Close()
	})
	return err
}
