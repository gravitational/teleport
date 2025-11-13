package linuxdesktop

import (
	"context"

	"github.com/gravitational/trace"

	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
)

// Client is a client for the Crown Jewel API.
type Client struct {
	grpcClient linuxdesktopv1.LinuxDesktopServiceClient
}

// NewClient creates a new Discovery Config client.
func NewClient(grpcClient linuxdesktopv1.LinuxDesktopServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// ListLinuxDesktops returns a list of Crown Jewels.
func (c *Client) ListLinuxDesktops(ctx context.Context, pageSize int, nextToken string) ([]*linuxdesktopv1.LinuxDesktop, string, error) {
	resp, err := c.grpcClient.ListLinuxDesktops(ctx, &linuxdesktopv1.ListLinuxDesktopsRequest{
		PageSize:  pageSize,
		PageToken: nextToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp.LinuxDesktops, resp.NextPageToken, nil
}

// CreateLinuxDesktop creates a new Crown Jewel.
func (c *Client) CreateLinuxDesktop(ctx context.Context, req *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	rsp, err := c.grpcClient.CreateLinuxDesktop(ctx, &linuxdesktopv1.CreateLinuxDesktopRequest{
		Desktop: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// GetLinuxDesktop returns a Crown Jewel by name.
func (c *Client) GetLinuxDesktop(ctx context.Context, name string) (*linuxdesktopv1.LinuxDesktop, error) {
	rsp, err := c.grpcClient.GetLinuxDesktop(ctx, &linuxdesktopv1.GetLinuxDesktopRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// UpdateLinuxDesktop updates an existing Crown Jewel.
func (c *Client) UpdateLinuxDesktop(ctx context.Context, req *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	rsp, err := c.grpcClient.UpdateLinuxDesktop(ctx, &linuxdesktopv1.UpdateLinuxDesktopRequest{
		LinuxDesktop: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// DeleteLinuxDesktop deletes a Crown Jewel.
func (c *Client) DeleteLinuxDesktop(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteLinuxDesktop(ctx, &linuxdesktopv1.DeleteLinuxDesktopRequest{
		Name: name,
	})
	return trace.Wrap(err)
}
