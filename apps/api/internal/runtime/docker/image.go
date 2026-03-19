package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	imagetypes "github.com/docker/docker/api/types/image"
)

func (c *Client) PullImage(ctx context.Context, image string) error {
	reader, err := c.cli.ImagePull(ctx, image, imagetypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", image, err)
	}
	defer reader.Close()

	// Must drain the reader for the pull to complete
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("reading pull response: %w", err)
	}

	return nil
}

func (c *Client) BuildImage(ctx context.Context, contextDir, dockerfile, tag string) error {
	// Create a tar archive of the context directory
	buf, err := tarDirectory(contextDir)
	if err != nil {
		return fmt.Errorf("tar context dir: %w", err)
	}

	resp, err := c.cli.ImageBuild(ctx, buf, types.ImageBuildOptions{
		Dockerfile: dockerfile,
		Tags:       []string{tag},
		Remove:     true,
	})
	if err != nil {
		return fmt.Errorf("build image: %w", err)
	}
	defer resp.Body.Close()

	// Must drain the reader for the build to complete
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return fmt.Errorf("reading build response: %w", err)
	}

	return nil
}

func (c *Client) RemoveImage(ctx context.Context, img string) error {
	_, err := c.cli.ImageRemove(ctx, img, imagetypes.RemoveOptions{PruneChildren: true})
	if err != nil {
		return fmt.Errorf("remove image %s: %w", img, err)
	}
	return nil
}

func (c *Client) PruneImages(ctx context.Context) error {
	_, err := c.cli.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "true")))
	if err != nil {
		return fmt.Errorf("prune images: %w", err)
	}
	return nil
}

// tarDirectory creates a tar archive from a directory.
func tarDirectory(dir string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = strings.ReplaceAll(relPath, string(filepath.Separator), "/")

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
}
