package awsic

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/lib/services"
)

type permissionSetAssertion func(assert.TestingT, *identitycenterv1.PermissionSet) bool

func withPSName(expected string) permissionSetAssertion {
	return func(t assert.TestingT, ps *identitycenterv1.PermissionSet) bool {
		return assert.Equal(t, expected, ps.GetSpec().GetName())
	}
}

func byPSARN(arnText string) services.PermissionSetID {
	id := arnText
	if parsedARN, err := arn.Parse(arnText); err == nil {
		id = parsedARN.Resource
	}
	return services.PermissionSetID(normalizeResourceName(id))
}

func assertPermissionSet(ctx context.Context, t assert.TestingT, permissionSetSvc services.IdentityCenterPermissionSets, id services.PermissionSetID, assertions ...permissionSetAssertion) bool {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	acct, err := permissionSetSvc.GetPermissionSet(ctx, id)
	if !assert.NoError(t, err, "Failed loading IC PermissionSet [%s]", id) {
		return false
	}
	for _, assertion := range assertions {
		if !assertion(t, acct) {
			return false
		}
	}
	return true
}

func requirePermissionSet(ctx context.Context, t require.TestingT, permissionSetSvc services.IdentityCenterPermissionSets, id services.PermissionSetID, assertions ...permissionSetAssertion) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertPermissionSet(ctx, t, permissionSetSvc, id, assertions...) {
		return
	}
	t.FailNow()
}

func assertNoPermissionSet(ctx context.Context, t assert.TestingT, permissionSetSvc services.IdentityCenterPermissionSets, id services.PermissionSetID) bool {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	_, err := permissionSetSvc.GetPermissionSet(ctx, id)
	return assert.ErrorIs(t, err, os.ErrNotExist, "PermissionSet %s must not exist", id)
}

func requireNoPermissionSet(ctx context.Context, t require.TestingT, permissionSetSvc services.IdentityCenterPermissionSets, id services.PermissionSetID) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertNoPermissionSet(ctx, t, permissionSetSvc, id) {
		return
	}
	t.FailNow()
}
