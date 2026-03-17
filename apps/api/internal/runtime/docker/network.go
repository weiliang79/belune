package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
)

func (c *Client) CreateNetwork(ctx context.Context, name string) error {
	// Check if network already exists
	networks, err := c.cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return fmt.Errorf("list networks: %w", err)
	}
	for _, n := range networks {
		if n.Name == name {
			return nil // Already exists
		}
	}

	_, err = c.cli.NetworkCreate(ctx, name, types.NetworkCreate{
		Driver: "bridge",
		Labels: map[string]string{
			labelManagedBy: labelValue,
		},
	})
	if err != nil {
		return fmt.Errorf("create network %s: %w", name, err)
	}

	return nil
}

func (c *Client) RemoveNetwork(ctx context.Context, name string) error {
	return c.cli.NetworkRemove(ctx, name)
}
