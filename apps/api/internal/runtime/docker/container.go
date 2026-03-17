package docker

import (
	"context"
	"io"

	"github.com/ungweiliang/selfhost-paas/internal/runtime"
)

func (c *Client) CreateContainer(ctx context.Context, cfg runtime.ContainerConfig) (string, error) {
	// TODO: Implement
	return "", nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	// TODO: Implement
	return nil
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	// TODO: Implement
	return nil
}

func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	// TODO: Implement
	return nil
}

func (c *Client) ContainerLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error) {
	// TODO: Implement
	return nil, nil
}

func (c *Client) ListContainers(ctx context.Context) ([]runtime.ContainerInfo, error) {
	// TODO: Implement
	return nil, nil
}
