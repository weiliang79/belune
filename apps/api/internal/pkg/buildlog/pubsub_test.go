package buildlog

import (
	"context"
	"testing"
)

// Live streaming is best-effort telemetry sitting on top of the deploy, and the
// durable copy of the log is written to deployments.build_logs either way. A
// Publisher with no Redis client must therefore go quiet rather than panic —
// previously it dereferenced the nil client from inside the build/pull path and
// took the whole deploy task down with it.
func TestPublisherWithoutRedisIsNoOp(t *testing.T) {
	ctx := context.Background()

	p := NewPublisher(nil, "deployment-id")
	if err := p.Publish(ctx, "a line"); err != nil {
		t.Errorf("Publish() = %v, want nil", err)
	}
	if err := p.Close(ctx); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}

	// A nil Publisher is tolerated for the same reason: callers building a log
	// sink should never have to nil-check before recording a line.
	var nilPub *Publisher
	if err := nilPub.Publish(ctx, "a line"); err != nil {
		t.Errorf("nil Publisher Publish() = %v, want nil", err)
	}
	if err := nilPub.Close(ctx); err != nil {
		t.Errorf("nil Publisher Close() = %v, want nil", err)
	}
}

// The writer path is what runs during a build or image pull, so it must survive
// a disabled publisher end to end rather than only at the Publish call.
func TestLogSinkWithoutRedis(t *testing.T) {
	sink := NewLogSink(NewPublisher(nil, "deployment-id"), context.Background())

	if _, err := sink.Writer("stdout").Write([]byte("Pulling image nginx:latest\n")); err != nil {
		t.Fatalf("Write() = %v, want nil", err)
	}
	sink.Flush()

	// The durable NDJSON copy is still produced — that is the part the log
	// viewer reads back, and it must not depend on Redis being present.
	if got := sink.NDJSON(); got == "" {
		t.Error("expected the durable NDJSON log to be recorded without Redis")
	}
}
