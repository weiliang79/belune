package testutil

import (
	"context"
	"io"
	"strings"
	"sync"

	"github.com/hibiken/asynq"

	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
)

// MockContainerRuntime implements runtime.ContainerRuntime for testing.
type MockContainerRuntime struct {
	mu              sync.Mutex
	StopCalls       []string
	RemoveCalls     []string
	StartCalls      []string
	CreateCalls     []runtime.ContainerConfig
	ListContainers_ []runtime.ContainerInfo
}

func (m *MockContainerRuntime) CreateContainer(_ context.Context, cfg runtime.ContainerConfig) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreateCalls = append(m.CreateCalls, cfg)
	return "mock-container-id", nil
}

func (m *MockContainerRuntime) StartContainer(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StartCalls = append(m.StartCalls, id)
	return nil
}

func (m *MockContainerRuntime) StopContainer(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StopCalls = append(m.StopCalls, id)
	return nil
}

func (m *MockContainerRuntime) RemoveContainer(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RemoveCalls = append(m.RemoveCalls, id)
	return nil
}

func (m *MockContainerRuntime) ContainerLogs(_ context.Context, _ string, _ bool) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *MockContainerRuntime) ListContainers(_ context.Context) ([]runtime.ContainerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ListContainers_ == nil {
		return []runtime.ContainerInfo{}, nil
	}
	return m.ListContainers_, nil
}

func (m *MockContainerRuntime) PullImage(_ context.Context, _ string) error   { return nil }
func (m *MockContainerRuntime) BuildImage(_ context.Context, _, _, _ string) error {
	return nil
}
func (m *MockContainerRuntime) CreateNetwork(_ context.Context, _ string) error              { return nil }
func (m *MockContainerRuntime) RemoveNetwork(_ context.Context, _ string) error              { return nil }
func (m *MockContainerRuntime) ConnectContainerToNetwork(_ context.Context, _, _ string) error { return nil }
func (m *MockContainerRuntime) CreateVolume(_ context.Context, _ string) error   { return nil }
func (m *MockContainerRuntime) RemoveVolume(_ context.Context, _ string) error   { return nil }
func (m *MockContainerRuntime) RemoveImage(_ context.Context, _ string) error    { return nil }
func (m *MockContainerRuntime) PruneImages(_ context.Context) error  { return nil }
func (m *MockContainerRuntime) PruneVolumes(_ context.Context) error { return nil }
func (m *MockContainerRuntime) ContainerStats(_ context.Context, _ string) (*runtime.ContainerResourceStats, error) {
	return &runtime.ContainerResourceStats{}, nil
}

// MockProxyManager implements proxy.ProxyManager for testing.
type MockProxyManager struct {
	mu            sync.Mutex
	AddedRoutes   []proxy.RouteConfig
	RemovedRoutes []string
}

func (m *MockProxyManager) AddRoute(_ context.Context, cfg proxy.RouteConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AddedRoutes = append(m.AddedRoutes, cfg)
	return nil
}

func (m *MockProxyManager) RemoveRoute(_ context.Context, hostname string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RemovedRoutes = append(m.RemovedRoutes, hostname)
	return nil
}

func (m *MockProxyManager) SetupTLS(_ context.Context, _ string) error { return nil }
func (m *MockProxyManager) ListRoutes(_ context.Context) ([]proxy.RouteConfig, error) {
	return nil, nil
}

// EnqueuedTask records a task that was enqueued.
type EnqueuedTask struct {
	TypeName string
	Payload  []byte
	Queue    string
}

// MockTaskEnqueuer implements handler.TaskEnqueuer for testing.
type MockTaskEnqueuer struct {
	mu    sync.Mutex
	Tasks []EnqueuedTask
}

func (m *MockTaskEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	et := EnqueuedTask{
		TypeName: task.Type(),
		Payload:  task.Payload(),
	}

	// Extract queue name from options
	for _, opt := range opts {
		// asynq.Queue returns an Option; we can't easily inspect it,
		// so we just record the task type and payload
		_ = opt
	}

	m.Tasks = append(m.Tasks, et)
	return &asynq.TaskInfo{}, nil
}
