package auth

import (
	"context"
	"net/url"

	"github.com/gravitational/reporting/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
)

// ProClient describes an auth client for Teleport Pro methods
type ProClient interface {
	// GetLicenseCheckResult returns the last license check result
	GetLicenseCheckResult(ctx context.Context) (*types.Heartbeat, error)
}

// client is the Teleport Pro auth client
type proClient struct {
	// Client is the OSS auth client
	*auth.Client
}

// NewProClient returns a client to Teleport Pro the auth service
func NewProClient(clt *auth.Client) (ProClient, error) {
	return &proClient{
		Client: clt,
	}, nil
}

// GetLicenseCheckResult returns the last license check result
func (c *proClient) GetLicenseCheckResult(ctx context.Context) (*types.Heartbeat, error) {
	out, err := c.Get(ctx, c.Endpoint("license", "status"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	heartbeat, err := types.UnmarshalHeartbeat(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return heartbeat, nil
}
