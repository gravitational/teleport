package accesslist

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
)

var (
	// ignoreEphemeralFields will be used to ignore fields that are irrelevant to determining
	// equivalence of a resource.
	ignoreEphemeralFields = []cmp.Option{
		cmpopts.IgnoreFields(accesslist.AccessList{}, "Status"),
		// ID is handled by the backend, so it'll be ignored here.
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
		// Ignore the IneligibleStatus field for owners since
		// it's managed by the reconciler.
		cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
		// Threat nil and empty slices as equal.
		cmpopts.EquateEmpty(),
	}

	oktaValidModificationsOptions = append([]cmp.Option{
		cmpopts.IgnoreFields(accesslist.Spec{}, "Owners", "MembershipRequires", "OwnershipRequires", "Audit"),
	}, ignoreEphemeralFields...)

	oktaReviewChangesOptions = append([]cmp.Option{
		cmpopts.IgnoreFields(accesslist.ReviewChanges{}, "RemovedMembers"),
	}, ignoreEphemeralFields...)
)

func accessListEqual(a, b *accesslist.AccessList) bool {
	return cmp.Equal(a, b, ignoreEphemeralFields...)
}

func membersEqual(a, b *accesslist.AccessListMember) bool {
	return cmp.Equal(a, b, ignoreEphemeralFields...)
}

func isOktaAccessListModificationAllowed(a, b *accesslist.AccessList) bool {
	return cmp.Equal(a, b, oktaValidModificationsOptions...)
}

func isReviewChangesAllowed(in accesslist.ReviewChanges) bool {
	var empty accesslist.ReviewChanges
	return cmp.Equal(empty, in, oktaReviewChangesOptions...)
}
