package docker

import (
	dockerclient "github.com/docker/docker/client"
)

// Client wraps the Docker SDK client.
type Client struct {
	cli *dockerclient.Client
}

// New creates a new Docker runtime client.
func New() (*Client, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

// Close closes the underlying Docker client.
func (c *Client) Close() error {
	return c.cli.Close()
}
