package equal

import (
	"sort"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
)

// PrincipalAssignmentEqual returns true if the two PrincipalAssignments are equal.
func PrincipalAssignmentEqual(a, b *identitycenterv1.PrincipalAssignment) bool {
	return metadataEqual(a.GetMetadata(), b.GetMetadata()) &&
		PrincipalAssignmentSpecEqual(a.GetSpec(), b.GetSpec()) &&
		PrincipalAssignmentStatusEqual(a.GetStatus(), b.GetStatus())
}

// PrincipalAssignmentSpecEqual returns true if the two PrincipalAssignmentSpecs are equal.
func PrincipalAssignmentSpecEqual(a, b *identitycenterv1.PrincipalAssignmentSpec) bool {
	return a.GetPrincipalType() == b.GetPrincipalType() &&
		a.GetPrincipalId() == b.GetPrincipalId() &&
		a.GetExternalIdSource() == b.GetExternalIdSource() &&
		a.GetExternalId() == b.GetExternalId()
}

// PrincipalAssignmentStatusEqual returns true if the two PrincipalAssignmentStatuses are equal.
func PrincipalAssignmentStatusEqual(a, b *identitycenterv1.PrincipalAssignmentStatus) bool {
	return AccountAssignmentRefsEqual(a.GetAssignments(), b.GetAssignments()) &&
		compareStringSlices(a.GetApplications(), b.GetApplications())
}

// AccountAssignmentRefsEqual returns true if the two slices of AccountAssignmentRefs are equal.
func AccountAssignmentRefsEqual(a, b []*identitycenterv1.AccountAssignmentRef) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Slice(a, func(i, j int) bool {
		if a[i].GetAccountId() == a[j].GetAccountId() {
			return a[i].GetPermissionSetArn() < a[j].GetPermissionSetArn()
		}
		return a[i].GetAccountId() < a[j].GetAccountId()
	})
	sort.Slice(b, func(i, j int) bool {
		if b[i].GetAccountId() == b[j].GetAccountId() {
			return b[i].GetPermissionSetArn() < b[j].GetPermissionSetArn()
		}
		return b[i].GetAccountId() < b[j].GetAccountId()
	})
	for i := range a {
		if !AccountAssignmentRefEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func AccountAssignmentRefEqual(a, b *identitycenterv1.AccountAssignmentRef) bool {
	return a.GetAccountId() == b.GetAccountId() &&
		a.GetAccountName() == b.GetAccountName() &&
		a.GetPermissionSetArn() == b.GetPermissionSetArn() &&
		a.GetPermissionSetName() == b.GetPermissionSetName()
}
