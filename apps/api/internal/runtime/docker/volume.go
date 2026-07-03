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

// labelData is a per-volume flag that opts a persistent application data
// volume out of PruneVolumes. Unlike caches (disposable), these hold user data
// and must NEVER be reaped by the cleanup worker, even while their app's
// container is absent between deploys (which would otherwise leave the volume
// dangling and eligible for prune).
const labelData = "paas-data"

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

// CreateDataVolume creates (or no-ops on) a Docker volume labelled as
// persistent application data. The data label opts the volume out of
// PruneVolumes so the cleanup worker cannot delete user data when the app's
// container is absent between deploys. Idempotent: VolumeCreate returns the
// existing volume if the name already exists.
func (c *Client) CreateDataVolume(ctx context.Context, name string) error {
	_, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
		Labels: map[string]string{
			labelManagedBy: labelValue,
			labelData:      "true",
		},
	})
	if err != nil {
		return fmt.Errorf("create data volume %s: %w", name, err)
	}
	return nil
}

// VolumeSize returns the on-disk usage of a named volume. Uses DiskUsage
// rather than VolumeInspect because size is only populated in the disk-usage
// API (inspect returns -1 unless the daemon was called with ?size=1, which
// the Docker Go SDK does not expose).
func (c *Client) VolumeSize(ctx context.Context, name string) (int64, error) {
	sizes, err := c.VolumeSizes(ctx, []string{name})
	if err != nil {
		return 0, err
	}
	return sizes[name], nil
}

// VolumeSizes runs a single DiskUsage call and extracts sizes for each named
// volume. DiskUsage is O(all volumes on the host); calling it once per
// lookup in a loop quickly exceeds the request timeout on busy hosts.
func (c *Client) VolumeSizes(ctx context.Context, names []string) (map[string]int64, error) {
	out := make(map[string]int64, len(names))
	if len(names) == 0 {
		return out, nil
	}
	du, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{
		Types: []types.DiskUsageObject{types.VolumeObject},
	})
	if err != nil {
		return nil, fmt.Errorf("disk usage: %w", err)
	}
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		want[n] = struct{}{}
	}
	for _, v := range du.Volumes {
		if v == nil || v.UsageData == nil {
			continue
		}
		if _, ok := want[v.Name]; ok {
			out[v.Name] = v.UsageData.Size
		}
	}
	return out, nil
}

func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	return c.cli.VolumeRemove(ctx, name, true)
}

// PruneVolumes removes dangling volumes except those explicitly tagged as
// caches or persistent application data. Multiple label! filters are ANDed by
// the daemon, so a volume is pruned only when it carries neither tag. Without
// these exclusions the periodic cleanup worker would wipe per-app CNB +
// BuildKit cache volumes between builds (Phase 5) and, worse, delete persistent
// application data volumes whenever an app's container is absent between
// deploys.
func (c *Client) PruneVolumes(ctx context.Context) error {
	_, err := c.cli.VolumesPrune(ctx, filters.NewArgs(
		filters.Arg("label!", labelCache+"=true"),
		filters.Arg("label!", labelData+"=true"),
	))
	if err != nil {
		return fmt.Errorf("prune volumes: %w", err)
	}
	return nil
}
