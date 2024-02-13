package integration

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
)

// AuthHelper is the interface one must implement to run the AccessRequestSuite.
// It can be implemented by an OSS Auth server, or an Enterprise auth server
// (in teleport.e).
type AuthHelper interface {
	StartServer(t *testing.T) *client.Client
	ServerAddr() string
	CredentialsForUser(t *testing.T, ctx context.Context, user types.User) client.Credentials
	SignIdentityForUser(t *testing.T, ctx context.Context, user types.User) string
}

// NewAccessRequestClient returns a new integration.Client.
func NewAccessRequestClient(client *client.Client) *Client {
	return &Client{client}
}
