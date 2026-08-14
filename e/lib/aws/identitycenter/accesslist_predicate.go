package identitycenter

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/e/lib/provisioning"
)

// makeAccessListAssignmentPredicate creates a new AccessListPredicate that
// filters out AccessLists that do not have any grants affecting Identity Center
// account assignments.
func makeAccessListAssignmentPredicate(rolesSvc RolesService) provisioning.AccessListPredicate {
	return func(ctx context.Context, acl *accesslist.AccessList) (bool, error) {
		if acl.Origin() == common.OriginAWSIdentityCenter {
			// If the ACL is from Identity Center, we always process it,
			// even if it doesn't contain any grants affecting the Identity Center account.
			// For example, an upstream AWS IC group might have been imported without being
			// provisioned with a permission set or group assignment to an AWS account.
			return true, nil
		}
		for _, roleName := range acl.GetGrants().Roles {
			role, err := rolesSvc.GetRole(ctx, roleName)
			if err != nil {
				return false, trace.Wrap(err)
			}

			roleV6, ok := role.(*types.RoleV6)
			if !ok {
				return false, trace.BadParameter("unexpected Role type %T", role)
			}

			if len(roleV6.Spec.Allow.AccountAssignments) > 0 || len(roleV6.Spec.Deny.AccountAssignments) > 0 {
				return true, nil
			}
		}
		return false, nil
	}
}
