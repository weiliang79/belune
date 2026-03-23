package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"

	"github.com/ungweiliang/selfhost-paas/internal/runtime"
)

const labelManagedBy = "managed-by"
const labelValue = "selfhost-paas"

func (c *Client) CreateContainer(ctx context.Context, cfg runtime.ContainerConfig) (string, error) {
	// Convert env map to Docker format
	var env []string
	for k, v := range cfg.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Convert port map to Docker format
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for hostPort, containerPort := range cfg.Ports {
		cp := nat.Port(containerPort + "/tcp")
		exposedPorts[cp] = struct{}{}
		portBindings[cp] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: hostPort},
		}
	}

	// Convert volume map to Docker format
	var binds []string
	for hostPath, containerPath := range cfg.Volumes {
		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	labels := map[string]string{
		labelManagedBy: labelValue,
	}
	for k, v := range cfg.Labels {
		labels[k] = v
	}

	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image:        cfg.Image,
			Env:          env,
			Cmd:          cfg.Cmd,
			ExposedPorts: exposedPorts,
			Labels:       labels,
		},
		&container.HostConfig{
			PortBindings:  portBindings,
			Binds:         binds,
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		},
		&network.NetworkingConfig{
			EndpointsConfig: func() map[string]*network.EndpointSettings {
				if cfg.Network == "" {
					return nil
				}
				return map[string]*network.EndpointSettings{
					cfg.Network: {},
				}
			}(),
		},
		nil,
		cfg.Name,
	)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	return resp.ID, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	timeout := 30
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (c *Client) ContainerLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
	})
}

func (c *Client) ListContainers(ctx context.Context) ([]runtime.ContainerInfo, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("%s=%s", labelManagedBy, labelValue)),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	var result []runtime.ContainerInfo
	for _, ctr := range containers {
		name := ""
		if len(ctr.Names) > 0 {
			name = ctr.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}

		ports := make(map[string]string)
		for _, p := range ctr.Ports {
			if p.PublicPort > 0 {
				ports[fmt.Sprintf("%d", p.PublicPort)] = fmt.Sprintf("%d", p.PrivatePort)
			}
		}

		result = append(result, runtime.ContainerInfo{
			ID:     ctr.ID,
			Name:   name,
			Image:  ctr.Image,
			Status: ctr.State,
			Ports:  ports,
			Labels: ctr.Labels,
		})
	}

	return result, nil
}

func (c *Client) ContainerStats(ctx context.Context, containerID string) (*runtime.ContainerResourceStats, error) {
	resp, err := c.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("container stats: %w", err)
	}
	defer resp.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("decode stats: %w", err)
	}

	// Calculate CPU percentage
	var cpuPercent float64
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(stats.CPUStats.OnlineCPUs) * 100.0
	}

	// Calculate network rx/tx
	var rxBytes, txBytes int64
	for _, netStats := range stats.Networks {
		rxBytes += int64(netStats.RxBytes)
		txBytes += int64(netStats.TxBytes)
	}

	return &runtime.ContainerResourceStats{
		CPUPercent:     cpuPercent,
		MemoryUsage:    int64(stats.MemoryStats.Usage),
		MemoryLimit:    int64(stats.MemoryStats.Limit),
		NetworkRxBytes: rxBytes,
		NetworkTxBytes: txBytes,
	}, nil
}
