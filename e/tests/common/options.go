package common

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type sutOptions struct {
	resources     []types.Resource
	samlConnector string
	license       string
	HTTPTransport http.RoundTripper
}

type option func(*sutOptions)

// WithUser adds a user with the given roles to the SUT.
func WithUser(t *testing.T, user string, roles ...string) func(*sutOptions) {
	aliceUser, err := types.NewUser(user)
	require.NoError(t, err)
	aliceUser.SetRoles(roles)

	return func(o *sutOptions) {
		o.resources = append(o.resources, aliceUser)
	}
}

// WithHTTPClient  adds an HTTP client to the SUT.
func WithHTTPClient(tr http.RoundTripper) func(*sutOptions) {
	return func(o *sutOptions) {
		o.HTTPTransport = tr
	}
}

// WithSAMLConnector add a SAML connector to the SUT.
func WithSAMLConnector(connector string) func(*sutOptions) {
	return func(o *sutOptions) {
		o.samlConnector = connector
	}
}

// WithLicense adds a license to the SUT.
func WithLicense(license string) func(*sutOptions) {
	return func(o *sutOptions) {
		o.license = license
	}
}
