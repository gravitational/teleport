package auth

import (
	"net/url"

	"github.com/gravitational/teleport/lib/auth"

	"github.com/gravitational/reporting/types"
	"github.com/gravitational/trace"
)

// Client extends OSS auth client interface with enterprise-specific methods
type Client interface {
	auth.ClientI
	// GetLicenseCheckResult returns the last license check result
	GetLicenseCheckResult() (*types.Heartbeat, error)
}

// client is the enterprise-specific auth client
type client struct {
	// TunClient is the OSS auth client
	*auth.TunClient
}

// NewClient returns a new enterprise auth client
func NewClient(clt *auth.TunClient) (*client, error) {
	return &client{
		TunClient: clt,
	}, nil
}

// GetLicenseCheckResult returns the last license check result
func (c *client) GetLicenseCheckResult() (*types.Heartbeat, error) {
	out, err := c.Get(c.Endpoint("license", "status"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	heartbeat, err := types.UnmarshalHeartbeat(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return heartbeat, nil
}
