package auth

import (
	"github.com/gravitational/roundtrip"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

const (
	// MissingNamespaceError indicates that the client failed to
	// provide the namespace in the request.
	MissingNamespaceError = authclient.MissingNamespaceError
)

type APIClient = client.Client
type Client = authclient.Client
type ClientI = authclient.ClientI

// NewClient creates a new API client with a connection to a Teleport server.
//
// The client will use the first credentials and the given dialer. If
// no dialer is given, the first address will be used. This address must
// be an auth server address.
//
// NOTE: This client is being deprecated in favor of the gRPC Client in
// teleport/api/client. This Client should only be used internally, or for
// functionality that hasn't been ported to the new client yet.
func NewClient(cfg client.Config, params ...roundtrip.ClientParam) (*Client, error) {
	return authclient.NewClient(cfg, params...)
}
