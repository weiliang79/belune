package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/runtime"
)

const containerTestImage = "busybox:latest"

// newTestDockerClient creates a Client and skips the test if the daemon is unreachable.
func newTestDockerClient(t *testing.T) *Client {
	t.Helper()
	c, err := New()
	if err != nil {
		t.Skipf("docker client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.ListContainers(ctx); err != nil {
		_ = c.Close()
		t.Skipf("docker daemon unreachable: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func uniqueContainerName(t *testing.T) string {
	t.Helper()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return "paas-test-" + hex.EncodeToString(b[:])
}

// TestContainerLifecycle exercises the create→start→list→stop→remove path
// against the real Docker daemon. Requires Docker to be running; skips gracefully
// if the daemon is unreachable.
func TestContainerLifecycle(t *testing.T) {
	c := newTestDockerClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Ensure the test image is present before creating the container.
	require.NoError(t, c.PullImage(ctx, containerTestImage), "pull %s", containerTestImage)

	name := uniqueContainerName(t)
	id, err := c.CreateContainer(ctx, runtime.ContainerConfig{
		Name:   name,
		Image:  containerTestImage,
		Cmd:    []string{"sleep", "3600"},
		Labels: map[string]string{"paas-unit-test": "true"},
	})
	require.NoError(t, err, "create container")
	// Ensure cleanup even if later steps fail.
	t.Cleanup(func() { _ = c.RemoveContainer(context.Background(), id) })

	require.NoError(t, c.StartContainer(ctx, id), "start container")

	// Newly started container must appear in ListContainers (filtered by the
	// managed-by label that CreateContainer stamps on every container).
	listed, err := c.ListContainers(ctx)
	require.NoError(t, err, "list containers after start")
	found := false
	for _, ci := range listed {
		if ci.ID == id {
			found = true
			assert.Equal(t, "running", ci.Status, "container should be running")
		}
	}
	assert.True(t, found, "started container should appear in ListContainers")

	require.NoError(t, c.StopContainer(ctx, id), "stop container")
	require.NoError(t, c.RemoveContainer(ctx, id), "remove container")

	// After removal the container must no longer appear in ListContainers.
	listed, err = c.ListContainers(ctx)
	require.NoError(t, err, "list containers after remove")
	for _, ci := range listed {
		assert.NotEqual(t, id, ci.ID, "removed container should not appear in ListContainers")
	}
}

// TestCreateContainerUnknownImage verifies that CreateContainer returns an error
// synchronously when the image does not exist locally. The call must complete
// promptly without hanging — confirming no background goroutine is leaked on
// the failure path.
func TestCreateContainerUnknownImage(t *testing.T) {
	c := newTestDockerClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan struct{})
	var createErr error
	go func() {
		defer close(done)
		_, createErr = c.CreateContainer(ctx, runtime.ContainerConfig{
			Name:  uniqueContainerName(t),
			Image: "paas-nonexistent-image-for-testing-99999:latest",
		})
	}()

	select {
	case <-done:
		assert.Error(t, createErr, "creating a container with an unknown image should return an error")
	case <-time.After(10 * time.Second):
		t.Fatal("CreateContainer did not return within 10s — possible goroutine hang")
	}
}
