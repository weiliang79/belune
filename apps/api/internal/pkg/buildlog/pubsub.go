package buildlog

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const channelPrefix = "build-logs:"
const doneSentinel = "__done__"

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

// Publish sends a single log line to the channel.
func (p *Publisher) Publish(ctx context.Context, line string) error {
	return p.rdb.Publish(ctx, p.channel, line).Err()
}

// Close publishes the done sentinel to signal end of stream.
func (p *Publisher) Close(ctx context.Context) error {
	return p.rdb.Publish(ctx, p.channel, doneSentinel).Err()
}

// Subscriber subscribes to build log lines from a Redis Pub/Sub channel.
type Subscriber struct {
	rdb    *redis.Client
	pubsub *redis.PubSub
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
				if msg.Payload == doneSentinel {
					return
				}
				out <- msg.Payload
			}
		}
	}()
	return out
}

// Close unsubscribes and closes the underlying pub/sub connection.
func (s *Subscriber) Close() error {
	return s.pubsub.Close()
}
