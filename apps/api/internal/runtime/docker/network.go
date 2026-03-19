package docker

import (
	"context"
	"fmt"

	networktypes "github.com/docker/docker/api/types/network"
)

func (c *Client) CreateNetwork(ctx context.Context, name string) error {
	// Check if network already exists
	networks, err := c.cli.NetworkList(ctx, networktypes.ListOptions{})
	if err != nil {
		return fmt.Errorf("list networks: %w", err)
	}
	for _, n := range networks {
		if n.Name == name {
			return nil // Already exists
		}
	}

	_, err = c.cli.NetworkCreate(ctx, name, networktypes.CreateOptions{
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
