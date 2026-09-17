package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"

	"github.com/weiliang79/belune/internal/runtime"
)

// SpawnUpdateHelper launches scripts/update.sh in a detached helper container
// and returns as soon as it has started — it does not attach, stream, wait, or
// remove it. See runtime.ContainerRuntime for why: the whole point is that this
// container outlives the one calling it.
//
// Two departures from RunHelper's helpers, both required by what update.sh
// itself does inside the container:
//   - Runs as root (User "0:0"): update.sh chowns host directories and talks to
//     the mounted Docker socket, the same as it does when a real operator runs
//     it via sudo.
//   - Host networking, not RunHelper's NetworkMode "none": update.sh's own
//     health-wait step curls http://localhost:8080/healthz, which only reaches
//     the recreated belune container from the HOST's network namespace — a
//     helper on its own bridge network would never see it. Host networking also
//     gives the helper the same internet reachability the host has, which
//     update.sh needs to fetch the target release's infra files.
func (c *Client) SpawnUpdateHelper(ctx context.Context, cfg runtime.UpdateHelperConfig) (string, error) {
	created, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image: cfg.Image,
			// Override the belune server entrypoint — this container's job is
			// to run the updater script, not serve traffic.
			Entrypoint: []string{"bash"},
			Cmd:        []string{"scripts/update.sh", cfg.Version},
			Env:        []string{"BELUNE_DIR=" + cfg.WorkingDir},
			User:       "0:0",
			WorkingDir: cfg.WorkingDir,
			Labels: map[string]string{
				labelManagedBy:            labelValue,
				runtime.LabelHelper:       "true",
				runtime.LabelUpdateHelper: "true",
			},
		},
		&container.HostConfig{
			Binds: []string{
				cfg.WorkingDir + ":" + cfg.WorkingDir,
				"/var/run/docker.sock:/var/run/docker.sock",
			},
			NetworkMode: "host",
		},
		nil, nil, "",
	)
	if err != nil {
		return "", fmt.Errorf("create update helper: %w", err)
	}
	if err := c.cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("start update helper: %w", err)
	}
	return created.ID, nil
}
