package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/crud"
)

type testNormalizeOps struct{}

func (testNormalizeOps) Kind() string {
	return types.KindUser
}

func (testNormalizeOps) NewResource(string) (types.Resource153, error) {
	return nil, nil
}

func (testNormalizeOps) Create(context.Context, types.Resource153) (types.Resource153, error) {
	return nil, nil
}

func (testNormalizeOps) Get153(context.Context, string) (types.Resource153, error) {
	return nil, nil
}

func (testNormalizeOps) List153(context.Context, int, string) ([]types.Resource153, string, error) {
	return nil, "", nil
}

func (testNormalizeOps) Update(context.Context, types.Resource153) (types.Resource153, error) {
	return nil, nil
}

func (testNormalizeOps) Delete(context.Context, string) error {
	return nil
}

func (testNormalizeOps) DeleteAll(context.Context) error {
	return nil
}

func (testNormalizeOps) Clone153(resource types.Resource153) types.Resource153 {
	return crud.CloneLegacyResource[types.User](resource)
}

func (testNormalizeOps) SupportsDeleteAll() bool {
	return false
}

func (testNormalizeOps) Equal(a, b types.Resource153) bool {
	return crud.EqualLegacyResource[types.User](a, b)
}

func TestEqualResourceAfterServerAssignmentsMatchesUserCreateDefaults(t *testing.T) {
	ops := testNormalizeOps{}

	requestedUser, err := types.NewUser("alice")
	require.NoError(t, err)
	requested := types.LegacyToResource153(requestedUser)

	returnedUser := requestedUser.Clone()
	returnedUser.SetRevision("server-revision")
	returnedUser.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: adminIdentity},
		Time: time.Unix(123, 0).UTC(),
	})
	returnedUser.SetPasswordState(types.PasswordState_PASSWORD_STATE_UNSET)
	returnedUser.SetWeakestDevice(types.MFADeviceKind_MFA_DEVICE_KIND_UNSET)
	returned := types.LegacyToResource153(returnedUser)

	require.True(t, equalResourceAfterServerAssignments(ops, requested, returned))
	require.True(t, requestedUser.GetCreatedBy().IsEmpty(), "normalization must not mutate the requested resource")
	require.Equal(t, types.PasswordState_PASSWORD_STATE_UNSPECIFIED, requestedUser.GetPasswordState())
	require.Equal(t, types.MFADeviceKind_MFA_DEVICE_KIND_UNSPECIFIED, requestedUser.GetWeakestDevice())
}
