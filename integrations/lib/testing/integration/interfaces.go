package integration

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
)

type AuthHelper interface {
	StartServer(t *testing.T) *client.Client
	ServerAddr() string
	CredentialsForUser(t *testing.T, ctx context.Context, user types.User) client.Credentials
	SignIdentityForUser(t *testing.T, ctx context.Context, user types.User) string
}

func NewAccessRequestClient(client *client.Client) *Client {
	return &Client{client}
}
