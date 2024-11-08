package principal

import (
	"github.com/gravitational/trace"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/provisioning"
	"github.com/gravitational/teleport/lib/services"
)

// GetID extracts the ID of a PrincipalAssignment from the protobuf record and
// casts it to a PrincipalAssignmentID
func GetID(p *identitycenterv1.PrincipalAssignment) services.PrincipalAssignmentID {
	return services.PrincipalAssignmentID(p.GetMetadata().GetName())
}

// GetExternalID fetches the externalID of the given PrincipalAssignment. May be
// empty if the External ID is not yet known.
func GetExternalID(p *identitycenterv1.PrincipalAssignment) provisioning.ExternalID {
	return provisioning.ExternalID(p.GetSpec().GetExternalId())
}

// GetIDForUser generates a deterministic PrincipalAssignmentID for a given
// user resource
func GetIDForUser(user types.User) services.PrincipalAssignmentID {
	return GetIDForUserName(user.GetName())
}

// GetIDForUserName generates a deterministic PrincipalAssignmentID for a given
// username
func GetIDForUserName(username string) services.PrincipalAssignmentID {
	return services.PrincipalAssignmentID("u-" + username)
}

// GetIDForAccessList generates a deterministic PrincipalAssignmentID for a given
// Access List
func GetIDForAccessList(acl *accesslist.AccessList) services.PrincipalAssignmentID {
	return GetIDForAccessListName(acl.GetName())
}

// GetIDForAccessListName generates a deterministic PrincipalAssignmentID for a
// given Access List name
func GetIDForAccessListName(accesslist string) services.PrincipalAssignmentID {
	return services.PrincipalAssignmentID("acl-" + accesslist)
}

// GetIDForPrincipalResource fetches the appropriate PrincipalAssignmentID for
// the supplied resource, which must represent either a User or AccessList.
// Matching is done by Kind, so that we can accommodate the Resource-header-only
// resources for delete events in the watcher.
func GetIDForPrincipalResource(r types.Resource) (services.PrincipalAssignmentID, error) {
	switch r.GetKind() {
	case types.KindUser:
		return GetIDForUserName(r.GetName()), nil

	case types.KindAccessList:
		return GetIDForAccessListName(r.GetName()), nil

	default:
		return "", trace.BadParameter("Unsupported resource kind %s", r.GetKind())
	}
}
