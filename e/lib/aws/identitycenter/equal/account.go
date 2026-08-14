package equal

import (
	"sort"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
)

// AccountEqual compares the identity center account objects.
func AccountEqual(a, b *identitycenterv1.Account) bool {
	return metadataEqual(a.GetMetadata(), b.GetMetadata()) &&
		AccountSpecEqual(a.GetSpec(), b.GetSpec())
}

// AccountSpecEqual compares the identity center account spec objects.
func AccountSpecEqual(s1, s2 *identitycenterv1.AccountSpec) bool {
	return s1.GetId() == s2.GetId() &&
		s1.GetArn() == s2.GetArn() &&
		s1.GetName() == s2.GetName() &&
		s1.GetDescription() == s2.GetDescription() &&
		s1.GetStartUrl() == s2.GetStartUrl() &&
		s1.GetIsOrganizationOwner() == s2.GetIsOrganizationOwner() &&
		PermissionSetsEqual(s1.GetPermissionSetInfo(), s2.GetPermissionSetInfo())
}

// PermissionSetsEqual compares the identity center permission set info objects.
func PermissionSetsEqual(a, b []*identitycenterv1.PermissionSetInfo) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Slice(a, func(i, j int) bool {
		if a[i].GetArn() == a[j].GetArn() {
			return a[i].GetName() < a[j].GetName()
		}
		return a[i].GetArn() < a[j].GetArn()
	})
	sort.Slice(b, func(i, j int) bool {
		if b[i].GetArn() == b[j].GetArn() {
			return b[i].GetName() < b[j].GetName()
		}
		return b[i].GetArn() < b[j].GetArn()
	})
	for i := range a {
		if !PermissionSetInfoEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}
