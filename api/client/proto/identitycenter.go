package proto

import (
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
)

// PackICAccountAssignment packs an Identity Center Account Assignment in to its
// wire format.
func PackICAccountAssignment(assignment *identitycenterv1.AccountAssignment) isPaginatedResource_Resource {
	return &PaginatedResource_IdentityCenterAccountAssignment{
		IdentityCenterAccountAssignment: &IdentityCenterAccountAssignment{
			Kind:        types.KindIdentityCenterAccountAssignment,
			Version:     assignment.GetVersion(),
			Metadata:    types.Metadata153ToLegacy(assignment.Metadata),
			DisplayName: assignment.GetSpec().GetDisplay(),
			Account: &IdentityCenterAccount{
				AccountName: assignment.GetSpec().GetAccountName(),
				ID:          assignment.GetSpec().GetAccountId(),
			},
			PermissionSet: &IdentityCenterPermissionSet{
				ARN:  assignment.GetSpec().GetPermissionSet().GetArn(),
				Name: assignment.GetSpec().GetPermissionSet().GetName(),
			},
		},
	}
}

// UnpackICAccountAssignment converts a wire-format IdentityCenterAccountAssignment
// resource back into an identitycenterv1.AccountAssignment instance.
func UnpackICAccountAssignment(src *IdentityCenterAccountAssignment) types.ResourceWithLabels {
	dst := &identitycenterv1.AccountAssignment{
		Kind:     types.KindIdentityCenterAccountAssignment,
		Version:  src.Version,
		Metadata: types.LegacyTo153Metadata(src.Metadata),
		Spec: &identitycenterv1.AccountAssignmentSpec{
			AccountId:   src.Account.ID,
			AccountName: src.Account.AccountName,
			Display:     src.DisplayName,
			PermissionSet: &identitycenterv1.PermissionSetInfo{
				Arn:  src.PermissionSet.ARN,
				Name: src.PermissionSet.Name,
			},
		},
	}
	return types.Resource153ToResourceWithLabels(dst)
}
