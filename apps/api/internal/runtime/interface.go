package runtime

import (
	"context"
	"io"
	"time"
)

// TerminalExecSession represents an active exec session with TTY in a container.
type TerminalExecSession struct {
	ExecID string
	RWC    io.ReadWriteCloser // combined stdin+stdout stream (TTY mode — no mux header)
}

type ContainerConfig struct {
	Name            string
	Image           string
	Env             map[string]string
	Ports           map[string]string // host:container
	Volumes         map[string]string // host:container
	Network         string
	Cmd             []string
	Labels          map[string]string
	CPULimit        float64 // CPU cores (0 = unlimited, e.g. 0.5 = half a core)
	MemoryLimit     int64   // bytes (0 = unlimited, e.g. 536870912 = 512 MB)
	HealthCheckPath string  // HTTP path for health polling after deploy (e.g. /healthz); empty = skip

	// Security hardening — applied as-is to the Docker HostConfig. Defaults
	// (zero-value) preserve legacy permissive behaviour so this struct can be
	// embedded by managed-database provisioning that has different needs.
	//
	// For user app containers, deploy_task.go fills these with:
	//   CapDrop=["ALL"], SecurityOpt=["no-new-privileges"], ReadonlyRootfs=true,
	//   Tmpfs={"/tmp":"", "/run":""}
	CapDrop        []string          // capabilities to drop (e.g. "ALL")
	CapAdd         []string          // capabilities to add back (e.g. "NET_BIND_SERVICE")
	SecurityOpt    []string          // security options (e.g. "no-new-privileges")
	ReadonlyRootfs bool              // when true, container rootfs is read-only
	Tmpfs          map[string]string // path → mount options (empty options OK)
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
	// UpdateContainerResources applies CPU (cores) / memory (bytes) limits to a
	// running container without recreating it. Zero means unlimited.
	UpdateContainerResources(ctx context.Context, id string, cpuCores float64, memoryBytes int64) error
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
	// CreateCacheVolume creates (or no-ops on) a Docker volume labelled as a
	// build cache. The cache label opts the volume out of PruneVolumes so the
	// periodic cleanup worker cannot wipe layer history between builds.
	CreateCacheVolume(ctx context.Context, name string) error
	// VolumeSize reports the on-disk size in bytes of a named volume, or
	// (0, nil) when the volume does not exist. Used to surface per-app cache
	// usage in the UI.
	VolumeSize(ctx context.Context, name string) (int64, error)
	// VolumeSizes reports on-disk sizes for multiple volumes in a single
	// Docker DiskUsage call. Use this when you need sizes for several volumes
	// at once — DiskUsage computes sizes for every volume on the host, so
	// calling VolumeSize N times does N full scans.
	VolumeSizes(ctx context.Context, names []string) (map[string]int64, error)
	RemoveVolume(ctx context.Context, name string) error
	RemoveImage(ctx context.Context, image string) error
	PruneImages(ctx context.Context) error
	PruneVolumes(ctx context.Context) error
	ContainerStats(ctx context.Context, containerID string) (*ContainerResourceStats, error)
	ContainerEvents(ctx context.Context, filters map[string][]string) (<-chan ContainerEvent, <-chan error)
	// ContainerExecTTY creates a new exec session in the named container with TTY enabled.
	// cmd is the command to run (e.g. ["sh"] or ["bash"]).
	// Returns a TerminalExecSession with an exec ID (for resize) and a combined RWC.
	ContainerExecTTY(ctx context.Context, containerName string, cmd []string) (*TerminalExecSession, error)
	// ContainerExecResize resizes the PTY for the given exec session.
	ContainerExecResize(ctx context.Context, execID string, rows, cols uint) error
}

// ContainerEvent represents a Docker container lifecycle event.
type ContainerEvent struct {
	ContainerID   string
	ContainerName string
	Status        string // start, stop, die, restart, oom
	Labels        map[string]string
	Time          time.Time
}
