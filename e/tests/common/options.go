package common

import (
	"net/http"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type sutOptions struct {
	resources     []types.Resource
	samlConnector string
	license       string
	HTTPTransport http.RoundTripper
	clock         clockwork.Clock
}

type option func(*sutOptions)

// WithUser adds a user with the given roles to the SUT.
func WithUser(t *testing.T, user string, roles ...string) func(*sutOptions) {
	userResource, err := types.NewUser(user)
	require.NoError(t, err)
	userResource.SetRoles(roles)

	return func(o *sutOptions) {
		o.resources = append(o.resources, userResource)
	}
}

func WithAllowAccountAssignmentRole(t *testing.T, name string, accountID string, permissionSet string) func(*sutOptions) {
	roleSpec := types.RoleSpecV6{
		Allow: types.RoleConditions{
			AccountAssignments: []types.IdentityCenterAccountAssignment{
				{Account: accountID, PermissionSet: permissionSet},
			},
		},
	}
	return WithRole(t, name, &roleSpec)
}

func WithRole(t *testing.T, name string, roleSpec *types.RoleSpecV6) func(*sutOptions) {
	role, err := types.NewRole(name, *roleSpec)
	require.NoError(t, err)
	return func(o *sutOptions) {
		o.resources = append(o.resources, role)
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

// WithClock sets the clock for the SUT.
func WithClock(clock clockwork.Clock) func(*sutOptions) {
	return func(o *sutOptions) {
		o.clock = clock
	}
}
