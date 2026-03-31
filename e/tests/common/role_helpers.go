package common

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
)

type RolesUpdater interface {
	services.RoleGetter
	UpdateRole(context.Context, types.Role) (types.Role, error)
}

func UpdateRole(ctx context.Context, rolesSvc RolesUpdater, roleName string, mutateFn func(types.Role)) error {
	refreshRole := func(ctx context.Context, isRetry bool) (types.Role, error) {
		role, err := rolesSvc.GetRole(ctx, roleName)
		return role, trace.Wrap(err)
	}
	updateRole := func(ctx context.Context, role types.Role) error {
		mutateFn(role)
		_, err := rolesSvc.UpdateRole(ctx, role)
		return trace.Wrap(err)
	}
	return trace.Wrap(
		retryutils.UpdateWithRetry(ctx, clockwork.NewRealClock(), refreshRole, updateRole))
}

type RoleAllowDesc RoleConditionsDesc
type RoleDenyDesc RoleConditionsDesc

type RoleConditionsDesc struct {
	groupLabels    types.Labels
	reviewRequests *types.AccessReviewConditions
}

func CreateRole(t *testing.T, sut *SUT, name string, allow RoleAllowDesc, deny RoleDenyDesc) types.Role {
	t.Helper()
	ctx := t.Context()
	authServer := sut.Teleport.Process.GetAuthServer()

	spec := types.RoleSpecV6{
		Allow: types.RoleConditions{
			GroupLabels:    allow.groupLabels,
			ReviewRequests: allow.reviewRequests,
		},
		Deny: types.RoleConditions{
			GroupLabels:    deny.groupLabels,
			ReviewRequests: deny.reviewRequests,
		},
	}

	role, err := types.NewRole(name, spec)
	require.NoError(t, err, "types.NewRole")

	created, err := authServer.CreateRole(ctx, role)
	require.NoError(t, err, "authServer.CreateRole")

	return created
}
