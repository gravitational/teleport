package equal

import (
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
)

// AccountAssignmentEqual returns true if the two AccountAssignments are equal.
func AccountAssignmentEqual(a, b *identitycenterv1.AccountAssignment) bool {
	return metadataEqual(a.GetMetadata(), b.GetMetadata()) &&
		AccountAssignmentSpecEqual(a.GetSpec(), b.GetSpec())
}

// AccountAssignmentSpecEqual returns true if the two AccountAssignmentSpecs are equal.
func AccountAssignmentSpecEqual(a, b *identitycenterv1.AccountAssignmentSpec) bool {
	return a.GetDisplay() == b.GetDisplay() &&
		a.GetAccountName() == b.GetAccountName() &&
		a.GetAccountId() == b.GetAccountId() &&
		PermissionSetInfoEqual(a.GetPermissionSet(), b.GetPermissionSet())
}
