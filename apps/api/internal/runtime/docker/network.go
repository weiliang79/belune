package docker

import (
	"context"
	"fmt"

	networktypes "github.com/docker/docker/api/types/network"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/metrics"
)

func (c *Client) CreateNetwork(ctx context.Context, name string) (err error) {
	defer func() { metrics.RecordDockerOp("create_network", err) }()
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

func (c *Client) ConnectContainerToNetwork(ctx context.Context, containerID, networkName string) (err error) {
	defer func() { metrics.RecordDockerOp("connect_network", err) }()
	if err = c.cli.NetworkConnect(ctx, networkName, containerID, nil); err != nil {
		return fmt.Errorf("connect container %s to network %s: %w", containerID, networkName, err)
	}
	return nil
}
