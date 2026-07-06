package testutil

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

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
	PullCalls       []string // image tags passed to PullImage
	ListContainers_ []runtime.ContainerInfo

	// Read-only admin Docker inspect fixtures (nil → empty result).
	ListAllContainers_ []runtime.ContainerInfo
	ListImages_        []runtime.ImageInfo
	ListVolumes_       []runtime.VolumeInfo
	ListNetworks_      []runtime.NetworkInfo
	SystemInfo_        *runtime.DockerSystemInfo
	SystemDiskUsage_   *runtime.DockerDiskUsage

	// ResolveImageDigest_ is returned by ResolveImageDigest; ResolveDigestCalls
	// records the refs it was asked to resolve.
	ResolveImageDigest_ string
	ResolveDigestCalls  []string

	// ExecFunc, when set, backs ContainerExec — lets tests simulate dump output
	// and exit codes. When nil, ContainerExec is a no-op returning exit 0.
	ExecFunc func(ctx context.Context, container string, cmd []string, stdin io.Reader, stdout, stderr io.Writer) (int, error)
	// RunHelperFunc, when set, backs RunHelper (volume tar snapshot/restore).
	// When nil, RunHelper is a no-op returning exit 0.
	RunHelperFunc func(ctx context.Context, cfg runtime.ContainerConfig, stdin io.Reader, stdout, stderr io.Writer) (int, error)
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

func (m *MockContainerRuntime) UpdateContainerResources(_ context.Context, _ string, _ float64, _ int64) error {
	return nil
}

func (m *MockContainerRuntime) ContainerLogs(_ context.Context, _ string, _ bool) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *MockContainerRuntime) ContainerLogsSince(_ context.Context, _ string, _ time.Time) (io.ReadCloser, error) {
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

func (m *MockContainerRuntime) ListAllContainers(_ context.Context) ([]runtime.ContainerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ListAllContainers_ == nil {
		return []runtime.ContainerInfo{}, nil
	}
	return m.ListAllContainers_, nil
}

func (m *MockContainerRuntime) ListImages(_ context.Context) ([]runtime.ImageInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ListImages_ == nil {
		return []runtime.ImageInfo{}, nil
	}
	return m.ListImages_, nil
}

func (m *MockContainerRuntime) ListVolumes(_ context.Context) ([]runtime.VolumeInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ListVolumes_ == nil {
		return []runtime.VolumeInfo{}, nil
	}
	return m.ListVolumes_, nil
}

func (m *MockContainerRuntime) ListNetworks(_ context.Context) ([]runtime.NetworkInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ListNetworks_ == nil {
		return []runtime.NetworkInfo{}, nil
	}
	return m.ListNetworks_, nil
}

func (m *MockContainerRuntime) SystemInfo(_ context.Context) (*runtime.DockerSystemInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SystemInfo_ == nil {
		return &runtime.DockerSystemInfo{}, nil
	}
	return m.SystemInfo_, nil
}

func (m *MockContainerRuntime) SystemDiskUsage(_ context.Context) (*runtime.DockerDiskUsage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SystemDiskUsage_ == nil {
		return &runtime.DockerDiskUsage{}, nil
	}
	return m.SystemDiskUsage_, nil
}

func (m *MockContainerRuntime) ResolveImageDigest(_ context.Context, ref string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ResolveDigestCalls = append(m.ResolveDigestCalls, ref)
	return m.ResolveImageDigest_, nil
}

func (m *MockContainerRuntime) PullImage(_ context.Context, image string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PullCalls = append(m.PullCalls, image)
	return nil
}
func (m *MockContainerRuntime) BuildImage(_ context.Context, _, _, _ string) error {
	return nil
}
func (m *MockContainerRuntime) CreateNetwork(_ context.Context, _ string) error { return nil }
func (m *MockContainerRuntime) RemoveNetwork(_ context.Context, _ string) error { return nil }
func (m *MockContainerRuntime) ConnectContainerToNetwork(_ context.Context, _, _ string) error {
	return nil
}
func (m *MockContainerRuntime) CreateVolume(_ context.Context, _ string) error      { return nil }
func (m *MockContainerRuntime) CreateCacheVolume(_ context.Context, _ string) error { return nil }
func (m *MockContainerRuntime) CreateDataVolume(_ context.Context, _ string) error  { return nil }
func (m *MockContainerRuntime) VolumeSize(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (m *MockContainerRuntime) VolumeSizes(_ context.Context, _ []string) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (m *MockContainerRuntime) RemoveVolume(_ context.Context, _ string) error { return nil }
func (m *MockContainerRuntime) RemoveImage(_ context.Context, _ string) error  { return nil }
func (m *MockContainerRuntime) PruneImages(_ context.Context) error            { return nil }
func (m *MockContainerRuntime) PruneVolumes(_ context.Context) error           { return nil }
func (m *MockContainerRuntime) PruneBuildCache(_ context.Context) error        { return nil }
func (m *MockContainerRuntime) ContainerStats(_ context.Context, _ string) (*runtime.ContainerResourceStats, error) {
	return &runtime.ContainerResourceStats{}, nil
}

func (m *MockContainerRuntime) ContainerEvents(_ context.Context, _ map[string][]string) (<-chan runtime.ContainerEvent, <-chan error) {
	ch := make(chan runtime.ContainerEvent)
	errCh := make(chan error)
	close(ch)
	close(errCh)
	return ch, errCh
}

func (m *MockContainerRuntime) ContainerExecTTY(_ context.Context, _ string, _ []string) (*runtime.TerminalExecSession, error) {
	return nil, nil
}

func (m *MockContainerRuntime) ContainerExecResize(_ context.Context, _ string, _, _ uint) error {
	return nil
}

func (m *MockContainerRuntime) ContainerExec(ctx context.Context, container string, cmd []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, container, cmd, stdin, stdout, stderr)
	}
	return 0, nil
}

func (m *MockContainerRuntime) RunHelper(ctx context.Context, cfg runtime.ContainerConfig, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if m.RunHelperFunc != nil {
		return m.RunHelperFunc(ctx, cfg, stdin, stdout, stderr)
	}
	return 0, nil
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

func (m *MockProxyManager) SetupTLS(_ context.Context, _, _, _, _ string) error { return nil }
func (m *MockProxyManager) ListRoutes(_ context.Context) ([]proxy.RouteConfig, error) {
	return nil, nil
}

// MockReconciler implements handler.ReconcilerStatusProvider for testing.
type MockReconciler struct {
	mu                sync.Mutex
	ReconcileNowCalls int
	Status_           proxy.ReconcilerStatus
	ReconcileErr      error
}

func (m *MockReconciler) Status() proxy.ReconcilerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Status_
}

func (m *MockReconciler) ReconcileNow(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReconcileNowCalls++
	return m.ReconcileErr
}

// MockQueueInspector implements handler.QueueInspector for testing.
type MockQueueInspector struct {
	mu                  sync.Mutex
	Info                map[string]*asynq.QueueInfo // per-queue info; missing → zero-value
	ArchivedByQ         map[string]int              // counts returned by DeleteAllArchivedTasks
	RetryByQ            map[string]int              // counts returned by DeleteAllRetryTasks
	DeleteArchivedCalls []string
	DeleteRetryCalls    []string
}

func (m *MockQueueInspector) GetQueueInfo(queue string) (*asynq.QueueInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if qi, ok := m.Info[queue]; ok {
		return qi, nil
	}
	return &asynq.QueueInfo{Queue: queue}, nil
}

func (m *MockQueueInspector) DeleteAllArchivedTasks(queue string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteArchivedCalls = append(m.DeleteArchivedCalls, queue)
	return m.ArchivedByQ[queue], nil
}

func (m *MockQueueInspector) DeleteAllRetryTasks(queue string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteRetryCalls = append(m.DeleteRetryCalls, queue)
	return m.RetryByQ[queue], nil
}

// EnqueuedTask records a task that was enqueued.
type EnqueuedTask struct {
	TypeName string
	Payload  []byte
	Queue    string
}

// MockTaskEnqueuer implements handler.TaskEnqueuer for testing.
type MockTaskEnqueuer struct {
	mu          sync.Mutex
	Tasks       []EnqueuedTask
	EnqueueErr  error // when set, Enqueue returns this error
	EnqueueOnce bool  // when true, EnqueueErr is cleared after first use
}

func (m *MockTaskEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.EnqueueErr != nil {
		err := m.EnqueueErr
		if m.EnqueueOnce {
			m.EnqueueErr = nil
		}
		return nil, err
	}

	et := EnqueuedTask{
		TypeName: task.Type(),
		Payload:  task.Payload(),
	}
	m.Tasks = append(m.Tasks, et)
	return &asynq.TaskInfo{}, nil
}
