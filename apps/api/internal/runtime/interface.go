package runtime

import (
	"context"
	"io"
)

type ContainerConfig struct {
	Name    string
	Image   string
	Env     map[string]string
	Ports   map[string]string // host:container
	Volumes map[string]string // host:container
	Network string
	Cmd     []string
	Labels  map[string]string
}

type ContainerInfo struct {
	ID     string
	Name   string
	Image  string
	Status string
	Ports  map[string]string
	Labels map[string]string
}

// ContainerRuntime abstracts container operations.
type ContainerRuntime interface {
	CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	RemoveContainer(ctx context.Context, id string) error
	ContainerLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error)
	ListContainers(ctx context.Context) ([]ContainerInfo, error)
	PullImage(ctx context.Context, image string) error
	BuildImage(ctx context.Context, contextDir, dockerfile, tag string) error
	CreateNetwork(ctx context.Context, name string) error
	RemoveNetwork(ctx context.Context, name string) error
	CreateVolume(ctx context.Context, name string) error
	RemoveVolume(ctx context.Context, name string) error
	RemoveImage(ctx context.Context, image string) error
	PruneImages(ctx context.Context) error
	PruneVolumes(ctx context.Context) error
}
