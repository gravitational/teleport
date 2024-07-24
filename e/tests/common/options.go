package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type sutOptions struct {
	resources []types.Resource
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
