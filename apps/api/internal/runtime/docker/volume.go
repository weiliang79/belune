package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

// labelCache is a per-volume flag that opts the volume out of PruneVolumes.
// Used by CNB/BuildKit cache volumes so layer history survives cleanup runs.
const labelCache = "paas-cache"

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

func (c *Client) CreateCacheVolume(ctx context.Context, name string) error {
	_, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
		Labels: map[string]string{
			labelManagedBy: labelValue,
			labelCache:     "true",
		},
	})
	if err != nil {
		return fmt.Errorf("create cache volume %s: %w", name, err)
	}
	return nil
}

// VolumeSize returns the on-disk usage of a named volume. Uses DiskUsage
// rather than VolumeInspect because size is only populated in the disk-usage
// API (inspect returns -1 unless the daemon was called with ?size=1, which
// the Docker Go SDK does not expose).
func (c *Client) VolumeSize(ctx context.Context, name string) (int64, error) {
	du, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{
		Types: []types.DiskUsageObject{types.VolumeObject},
	})
	if err != nil {
		return 0, fmt.Errorf("disk usage: %w", err)
	}
	for _, v := range du.Volumes {
		if v != nil && v.Name == name && v.UsageData != nil {
			return v.UsageData.Size, nil
		}
	}
	return 0, nil
}

func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	return c.cli.VolumeRemove(ctx, name, true)
}

// PruneVolumes removes dangling volumes except those explicitly tagged as
// caches. Without the label filter the periodic cleanup worker would wipe
// per-app CNB + BuildKit cache volumes between builds, negating Phase 5.
func (c *Client) PruneVolumes(ctx context.Context) error {
	_, err := c.cli.VolumesPrune(ctx, filters.NewArgs(
		filters.Arg("label!", labelCache+"=true"),
	))
	if err != nil {
		return fmt.Errorf("prune volumes: %w", err)
	}
	return nil
}
