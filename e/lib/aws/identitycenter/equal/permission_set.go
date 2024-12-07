package equal

import (
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
)

func PermissionSetEqual(a, b *identitycenterv1.PermissionSet) bool {
	return metadataEqual(a.GetMetadata(), b.GetMetadata()) &&
		PermissionSetSpecEqual(a.GetSpec(), b.GetSpec())
}

func PermissionSetSpecEqual(a, b *identitycenterv1.PermissionSetSpec) bool {
	return a.GetArn() == b.GetArn() &&
		a.GetName() == b.GetName() &&
		a.GetDescription() == b.GetDescription()
}

func PermissionSetInfoEqual(a *identitycenterv1.PermissionSetInfo, b *identitycenterv1.PermissionSetInfo) bool {
	return a.GetName() == b.GetName() &&
		a.GetArn() == b.GetArn() &&
		a.GetRole() == b.GetRole() &&
		a.GetAssignmentId() == b.GetAssignmentId()
}
