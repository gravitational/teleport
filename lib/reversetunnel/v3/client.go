package reversetunnel

import (
	"context"
	"log/slog"
)

type Client struct{}

func (c *Client) Run(ctx context.Context, log *slog.Logger) error {
	panic("todo")
}

func (c *Client) Stop(ctx context.Context, log *slog.Logger) {
	panic("todo")
}
