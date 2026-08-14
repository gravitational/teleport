// Package principal defines utilites for dealing with Identity Center Principal
// Assignment records for use within `identitycenter` and its various
// sub-packages.
package principal

import (
	"context"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/lib/services"
)

// NewFor creates a new, in-memory PrincipalAssignment record for the given principal
func NewFor(r types.Resource) (*identitycenterv1.PrincipalAssignment, error) {
	var principalID services.PrincipalAssignmentID
	var principalType identitycenterv1.PrincipalType

	switch p := r.(type) {
	case *types.UserV2:
		principalID = GetIDForUserName(p.GetName())
		principalType = identitycenterv1.PrincipalType_PRINCIPAL_TYPE_USER

	case *accesslist.AccessList:
		principalID = GetIDForAccessListName(p.GetName())
		principalType = identitycenterv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST

	default:
		return nil, trace.BadParameter("unsupported resource type %T", r)
	}

	principalAsssignment := identitycenterv1.PrincipalAssignment_builder{
		Kind:    types.KindIdentityCenterPrincipalAssignment,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: string(principalID),
			Labels: map[string]string{
				types.OriginLabel: common.OriginAWSIdentityCenter,
			},
		}.Build(),
		Spec: identitycenterv1.PrincipalAssignmentSpec_builder{
			PrincipalType: principalType,
			PrincipalId:   r.GetName(),
		}.Build(),
		Status: identitycenterv1.PrincipalAssignmentStatus_builder{
			ProvisioningState: identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED,
		}.Build(),
	}.Build()

	return principalAsssignment, nil
}

// CreateFor creates a new PrincipalAssignment record for the given principal and writes it
// to the supplied data service. Returns the resource returned from the backend.
func CreateFor(
	ctx context.Context,
	r types.Resource,
	svc services.IdentityCenterPrincipalAssignments,
) (*identitycenterv1.PrincipalAssignment, error) {
	principalAssignment, err := NewFor(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createdAssignment, err := svc.CreatePrincipalAssignment(ctx, principalAssignment)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createdAssignment, nil
}
