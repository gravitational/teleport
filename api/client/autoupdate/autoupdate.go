package autoupdate

import (
	"context"

	"github.com/gravitational/trace"

	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

// Client is an access list client that conforms to the following lib/services interfaces:
// * services.AutoUpdates
type Client struct {
	grpcClient autoupdatev1.AutoUpdateServiceClient
}

// NewClient creates a new Access List client.
func NewClient(grpcClient autoupdatev1.AutoUpdateServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// CreateAutoUpdateConfig creates AutoUpdateConfig resource.
func (c *Client) CreateAutoUpdateConfig(ctx context.Context, config *autoupdatev1.AutoUpdateConfig) (*autoupdatev1.AutoUpdateConfig, error) {
	resp, err := c.grpcClient.CreateAutoUpdateConfig(ctx, &autoupdatev1.CreateAutoUpdateConfigRequest{
		Config: config,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// GetAutoUpdateConfig gets AutoUpdateConfig resource.
func (c *Client) GetAutoUpdateConfig(ctx context.Context) (*autoupdatev1.AutoUpdateConfig, error) {
	resp, err := c.grpcClient.GetAutoUpdateConfig(ctx, &autoupdatev1.GetAutoUpdateConfigRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpdateAutoUpdateConfig updates AutoUpdateConfig resource.
func (c *Client) UpdateAutoUpdateConfig(ctx context.Context, config *autoupdatev1.AutoUpdateConfig) (*autoupdatev1.AutoUpdateConfig, error) {
	resp, err := c.grpcClient.UpdateAutoUpdateConfig(ctx, &autoupdatev1.UpdateAutoUpdateConfigRequest{
		Config: config,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpsertAutoUpdateConfig updates or creates AutoUpdateConfig resource.
func (c *Client) UpsertAutoUpdateConfig(ctx context.Context, config *autoupdatev1.AutoUpdateConfig) (*autoupdatev1.AutoUpdateConfig, error) {
	resp, err := c.grpcClient.UpsertAutoUpdateConfig(ctx, &autoupdatev1.UpsertAutoUpdateConfigRequest{
		Config: config,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// DeleteAutoUpdateConfig deletes AutoUpdateConfig resource.
func (c *Client) DeleteAutoUpdateConfig(ctx context.Context) error {
	_, err := c.grpcClient.DeleteAutoUpdateConfig(ctx, &autoupdatev1.DeleteAutoUpdateConfigRequest{})
	return trace.Wrap(err)
}

// CreateAutoUpdateVersion creates AutoUpdateVersion resource.
func (c *Client) CreateAutoUpdateVersion(ctx context.Context, version *autoupdatev1.AutoUpdateVersion) (*autoupdatev1.AutoUpdateVersion, error) {
	resp, err := c.grpcClient.CreateAutoUpdateVersion(ctx, &autoupdatev1.CreateAutoUpdateVersionRequest{
		Version: version,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// GetAutoUpdateVersion gets AutoUpdateVersion resource.
func (c *Client) GetAutoUpdateVersion(ctx context.Context) (*autoupdatev1.AutoUpdateVersion, error) {
	resp, err := c.grpcClient.GetAutoUpdateVersion(ctx, &autoupdatev1.GetAutoUpdateVersionRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpdateAutoUpdateVersion updates AutoUpdateVersion resource.
func (c *Client) UpdateAutoUpdateVersion(ctx context.Context, version *autoupdatev1.AutoUpdateVersion) (*autoupdatev1.AutoUpdateVersion, error) {
	resp, err := c.grpcClient.UpdateAutoUpdateVersion(ctx, &autoupdatev1.UpdateAutoUpdateVersionRequest{
		Version: version,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpsertAutoUpdateVersion updates or creates AutoUpdateVersion resource.
func (c *Client) UpsertAutoUpdateVersion(ctx context.Context, version *autoupdatev1.AutoUpdateVersion) (*autoupdatev1.AutoUpdateVersion, error) {
	resp, err := c.grpcClient.UpsertAutoUpdateVersion(ctx, &autoupdatev1.UpsertAutoUpdateVersionRequest{
		Version: version,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// DeleteAutoUpdateVersion deletes AutoUpdateVersion resource.
func (c *Client) DeleteAutoUpdateVersion(ctx context.Context) error {
	_, err := c.grpcClient.DeleteAutoUpdateVersion(ctx, &autoupdatev1.DeleteAutoUpdateVersionRequest{})
	return trace.Wrap(err)
}
