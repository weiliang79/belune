package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/metrics"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
)

const labelManagedBy = "managed-by"
const labelValue = "selfhost-paas"

func (c *Client) CreateContainer(ctx context.Context, cfg runtime.ContainerConfig) (id string, err error) {
	defer func() { metrics.RecordDockerOp("create_container", err) }()
	// Convert env map to Docker format
	var env []string
	for k, v := range cfg.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Convert port map to Docker format
	hostBindIP := cfg.HostBindIP
	if hostBindIP == "" {
		hostBindIP = "0.0.0.0"
	}
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for hostPort, containerPort := range cfg.Ports {
		cp := nat.Port(containerPort + "/tcp")
		exposedPorts[cp] = struct{}{}
		portBindings[cp] = []nat.PortBinding{
			{HostIP: hostBindIP, HostPort: hostPort},
		}
	}

	// Convert volume map to Docker format
	var binds []string
	for hostPath, containerPath := range cfg.Volumes {
		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}
	// Read-only file/config mounts: bind a managed host file at a container path.
	for _, b := range cfg.ReadOnlyBinds {
		binds = append(binds, fmt.Sprintf("%s:%s:ro", b.Source, b.Target))
	}

	labels := map[string]string{
		labelManagedBy: labelValue,
	}
	for k, v := range cfg.Labels {
		labels[k] = v
	}

	// Apply optional resource limits
	resources := container.Resources{}
	if cfg.CPULimit > 0 {
		resources.NanoCPUs = int64(cfg.CPULimit * 1e9)
	}
	if cfg.MemoryLimit > 0 {
		resources.Memory = cfg.MemoryLimit
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
			PortBindings:   portBindings,
			Binds:          binds,
			RestartPolicy:  container.RestartPolicy{Name: "unless-stopped"},
			Resources:      resources,
			CapDrop:        cfg.CapDrop,
			CapAdd:         cfg.CapAdd,
			SecurityOpt:    cfg.SecurityOpt,
			ReadonlyRootfs: cfg.ReadonlyRootfs,
			Tmpfs:          cfg.Tmpfs,
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

func (c *Client) StartContainer(ctx context.Context, id string) (err error) {
	defer func() { metrics.RecordDockerOp("start_container", err) }()
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) StopContainer(ctx context.Context, id string) (err error) {
	defer func() { metrics.RecordDockerOp("stop_container", err) }()
	timeout := 30
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) RemoveContainer(ctx context.Context, id string) (err error) {
	defer func() { metrics.RecordDockerOp("remove_container", err) }()
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

// UpdateContainerResources applies new CPU/memory limits to a running container
// live (no recreate), using the same conversion as create time. A zero value
// means unlimited for that dimension.
func (c *Client) UpdateContainerResources(ctx context.Context, id string, cpuCores float64, memoryBytes int64) (err error) {
	defer func() { metrics.RecordDockerOp("update_container", err) }()
	resources := container.Resources{}
	if cpuCores > 0 {
		resources.NanoCPUs = int64(cpuCores * 1e9)
	}
	if memoryBytes > 0 {
		resources.Memory = memoryBytes
	}
	_, err = c.cli.ContainerUpdate(ctx, id, container.UpdateConfig{Resources: resources})
	return err
}

func (c *Client) ContainerLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
	})
}

func (c *Client) ContainerLogsSince(ctx context.Context, id string, since time.Time) (io.ReadCloser, error) {
	// An empty Since tells Docker to return the full buffered log history.
	// A non-zero time resumes from that point onward.
	sinceStr := ""
	if !since.IsZero() {
		sinceStr = since.Format(time.RFC3339Nano)
	}
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Since:      sinceStr,
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
	return mapContainerSummaries(containers), nil
}

// ListAllContainers lists every container on the host (running and stopped),
// including ones not managed by the platform. Read-only; used by the admin
// Docker inspect page. It intentionally omits the managed-by label filter that
// ListContainers applies for health probes and metrics.
func (c *Client) ListAllContainers(ctx context.Context) (result []runtime.ContainerInfo, err error) {
	defer func() { metrics.RecordDockerOp("list_all_containers", err) }()
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list all containers: %w", err)
	}
	return mapContainerSummaries(containers), nil
}

// mapContainerSummaries converts Docker container summaries into the runtime DTO.
func mapContainerSummaries(containers []container.Summary) []runtime.ContainerInfo {
	result := make([]runtime.ContainerInfo, 0, len(containers))
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
			ID:        ctr.ID,
			Name:      name,
			Image:     ctr.Image,
			Status:    string(ctr.State),
			Ports:     ports,
			Labels:    ctr.Labels,
			CreatedAt: time.Unix(ctr.Created, 0),
		})
	}
	return result
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

func (c *Client) ContainerEvents(ctx context.Context, eventFilters map[string][]string) (<-chan runtime.ContainerEvent, <-chan error) {
	outCh := make(chan runtime.ContainerEvent, 64)
	errCh := make(chan error, 1)

	f := filters.NewArgs()
	f.Add("type", string(events.ContainerEventType))
	for key, vals := range eventFilters {
		for _, v := range vals {
			f.Add(key, v)
		}
	}

	msgCh, dockerErrCh := c.cli.Events(ctx, events.ListOptions{Filters: f})

	go func() {
		defer close(outCh)
		defer close(errCh)
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-dockerErrCh:
				if !ok {
					return
				}
				errCh <- err
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				name := msg.Actor.Attributes["name"]
				name = strings.TrimPrefix(name, "/")
				outCh <- runtime.ContainerEvent{
					ContainerID:   msg.Actor.ID,
					ContainerName: name,
					Status:        string(msg.Action),
					Labels:        msg.Actor.Attributes,
					Time:          time.Unix(msg.Time, msg.TimeNano),
				}
			}
		}
	}()

	return outCh, errCh
}
