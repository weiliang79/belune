package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"github.com/ungweiliang/selfhost-paas/internal/runtime"
)

// hijackedRWC adapts a Docker HijackedResponse to io.ReadWriteCloser.
// In TTY mode the stream is raw (no multiplexing header), so we read from the
// buffered Reader and write directly to the underlying Conn.
type hijackedRWC struct {
	resp types.HijackedResponse
}

func (h *hijackedRWC) Read(p []byte) (int, error) {
	return h.resp.Reader.Read(p)
}

func (h *hijackedRWC) Write(p []byte) (int, error) {
	return h.resp.Conn.Write(p)
}

func (h *hijackedRWC) Close() error {
	h.resp.Close() // returns void
	return nil
}

var _ io.ReadWriteCloser = (*hijackedRWC)(nil)

// ContainerExecTTY creates a PTY-enabled exec session in the named container.
func (c *Client) ContainerExecTTY(ctx context.Context, containerName string, cmd []string) (*runtime.TerminalExecSession, error) {
	execResp, err := c.cli.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          cmd,
	})
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	hr, err := c.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{
		Tty: true,
	})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}

	return &runtime.TerminalExecSession{
		ExecID: execResp.ID,
		RWC:    &hijackedRWC{resp: hr},
	}, nil
}

// ContainerExecResize resizes the PTY window for the given exec session.
func (c *Client) ContainerExecResize(ctx context.Context, execID string, rows, cols uint) error {
	return c.cli.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: rows,
		Width:  cols,
	})
}
