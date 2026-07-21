package accesslist

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// oktaMembersModificationAllowed checks if modifying members of Access Lists is allowed. It
// only considers Access Lists of Okta origin. It considers modification allowed if it is allowed
// in all the supplied Access Lists. Modification is not allowed in bidirectional sync is disabled
// in the Okta plugin.
func oktaMembersModificationAllowed(
	ctx context.Context,
	authCtx *authz.ScopedContext,
	plugins services.Plugins,
	accessList *accesslist.AccessList,
) (bool, error) {
	if !hasOktaOrigin(accessList) {
		return true, nil
	}

	// Services like SCIM and sync from Okta to Teleport has to work regardless of read-only
	// (bidirectional sync) mode.
	if hasAnyUnscopedAllowedSystemRole(authCtx, types.RoleAuth, types.RoleOkta) {
		return true, nil
	}

	plugin, err := oktaplugin.Get(ctx, plugins, false /* withSecrets */)
	if trace.IsNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, trace.Wrap(err, "getting Okta plugin")
	}

	bidirectionalSync := !plugin.Spec.GetOkta().GetSyncSettings().DisableBidirectionalSync
	return bidirectionalSync, nil
}

// oktaModificationAllowed will return true if an Okta modification is allowed. If the access list is not an Okta object,
// this will return true.
func oktaModificationAllowed(authCtx *authz.ScopedContext, oldAccessList, newAccessList *accesslist.AccessList) bool {
	if !hasOktaOrigin(oldAccessList) && !hasOktaOrigin(newAccessList) {
		return true
	}

	if hasAnyUnscopedAllowedSystemRole(authCtx, types.RoleOkta) {
		return true
	}

	return accesslist.EqualAccessLists(oldAccessList, newAccessList, accesslist.WithIgnoreOktaUserManagedFields())
}

// hasOktaOrigin returns true if any of the provides Access Lists is Okta originated.
func hasOktaOrigin(accessList *accesslist.AccessList) bool {
	return accessList != nil && accessList.Origin() == types.OriginOkta
}

// oktaDeletionAllowed returns true if the Access List is Okta-originated and deletion is allowed.
// Deletion is allowed when one of the following conditions is met:
//   - The Access List is not Okta.
//   - The requester has the Okta service role.
//   - There is no Okta plugin configured.
//   - The configured Okta plugin is not syncing Access Lists.
//
// Okta-originated Access Lists are not user-deletable while they're being synced by the Okta integration to prevent
// Access List deletion in Teleport resulting in Okta group unassignment.
func oktaDeletionAllowed(ctx context.Context, authCtx *authz.ScopedContext, plugins services.Plugins, accessList *accesslist.AccessList) (bool, error) {
	if !hasOktaOrigin(accessList) {
		return true, nil
	}

	if hasAnyUnscopedAllowedSystemRole(authCtx, types.RoleOkta) {
		return true, nil
	}

	plugin, err := oktaplugin.Get(ctx, plugins, false)
	if trace.IsNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, trace.Wrap(err)
	}

	return !plugin.Spec.GetOkta().GetSyncSettings().GetEnableAccessListSync(), nil
}

// hasAnyUnscopedAllowedSystemRole returns true if the auth context is unscoped
// and has any of the given allowed system roles. The system roles passed here
// are currently always unscoped. If they ever become scoped, this will fail closed.
func hasAnyUnscopedAllowedSystemRole(authCtx *authz.ScopedContext, allowedSystemRoles ...types.SystemRole) bool {
	unscopedCtx, isUnscoped := authCtx.UnscopedContext()
	if !isUnscoped {
		return false
	}
	for _, allowsSystemRole := range allowedSystemRoles {
		if authz.HasBuiltinRole(*unscopedCtx, string(allowsSystemRole)) {
			return true
		}
	}
	return false
}
