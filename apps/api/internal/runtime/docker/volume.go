package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

func (c *Client) CreateVolume(ctx context.Context, name string) error {
	_, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
		Labels: map[string]string{
			labelManagedBy: labelValue,
		},
	})
	if err != nil {
		return fmt.Errorf("create volume %s: %w", name, err)
	}
	return nil
}

func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	return c.cli.VolumeRemove(ctx, name, true)
}

func (c *Client) PruneVolumes(ctx context.Context) error {
	_, err := c.cli.VolumesPrune(ctx, filters.NewArgs())
	if err != nil {
		return fmt.Errorf("prune volumes: %w", err)
	}
	return nil
}
