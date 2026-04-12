package runtime

import (
	"context"
	"io"
	"time"
)

type ContainerConfig struct {
	Name        string
	Image       string
	Env         map[string]string
	Ports       map[string]string // host:container
	Volumes     map[string]string // host:container
	Network     string
	Cmd         []string
	Labels      map[string]string
	CPULimit        float64 // CPU cores (0 = unlimited, e.g. 0.5 = half a core)
	MemoryLimit     int64   // bytes (0 = unlimited, e.g. 536870912 = 512 MB)
	HealthCheckPath string  // HTTP path for health polling after deploy (e.g. /healthz); empty = skip
}

type ContainerInfo struct {
	ID        string
	Name      string
	Image     string
	Status    string
	Ports     map[string]string
	Labels    map[string]string
	CreatedAt time.Time
}

// ContainerResourceStats holds resource usage stats for a single container.
type ContainerResourceStats struct {
	CPUPercent     float64
	MemoryUsage    int64
	MemoryLimit    int64
	NetworkRxBytes int64
	NetworkTxBytes int64
}

// ContainerRuntime abstracts container operations.
type ContainerRuntime interface {
	CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	RemoveContainer(ctx context.Context, id string) error
	ContainerLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error)
	// ContainerLogsSince streams logs from a container starting at the given time.
	// Pass time.Now() to receive only new log lines (no backlog).
	ContainerLogsSince(ctx context.Context, id string, since time.Time) (io.ReadCloser, error)
	ListContainers(ctx context.Context) ([]ContainerInfo, error)
	PullImage(ctx context.Context, image string) error
	BuildImage(ctx context.Context, contextDir, dockerfile, tag string) error
	CreateNetwork(ctx context.Context, name string) error
	RemoveNetwork(ctx context.Context, name string) error
	ConnectContainerToNetwork(ctx context.Context, containerID, networkName string) error
	CreateVolume(ctx context.Context, name string) error
	RemoveVolume(ctx context.Context, name string) error
	RemoveImage(ctx context.Context, image string) error
	PruneImages(ctx context.Context) error
	PruneVolumes(ctx context.Context) error
	ContainerStats(ctx context.Context, containerID string) (*ContainerResourceStats, error)
	ContainerEvents(ctx context.Context, filters map[string][]string) (<-chan ContainerEvent, <-chan error)
}

// ContainerEvent represents a Docker container lifecycle event.
type ContainerEvent struct {
	ContainerID   string
	ContainerName string
	Status        string // start, stop, die, restart, oom
	Labels        map[string]string
	Time          time.Time
}
