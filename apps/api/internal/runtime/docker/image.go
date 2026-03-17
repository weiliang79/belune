package docker

import "context"

func (c *Client) PullImage(ctx context.Context, image string) error {
	// TODO: Implement
	return nil
}

func (c *Client) BuildImage(ctx context.Context, contextDir, dockerfile, tag string) error {
	// TODO: Implement
	return nil
}
