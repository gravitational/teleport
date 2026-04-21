/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
	webui "github.com/gravitational/teleport/lib/web/ui"
)

// UnifiedResourcePrincipals holds per-dimension principal sets for a unified
// resource. Only the fields relevant to the resource kind will be populated.
type UnifiedResourcePrincipals struct {
	// Logins is populated for SSH nodes.
	Logins *webui.PrincipalSet
	// AWSRoleARNs is populated for AWS Console apps.
	AWSRoleARNs *webui.PrincipalSet
	// DBPrincipalsByRole maps Teleport role names to the per-role database
	// principal groups (with requiresRequest metadata). Populated for databases.
	DBPrincipalsByRole map[string]webui.DatabaseRolePrincipalGroup
	// DBAutoUserEnabled indicates that auto-user provisioning is enabled for
	// a database resource, considering both granted and requestable roles.
	DBAutoUserEnabled bool
}

// PrincipalsForUnifiedResourceOpts configures PrincipalsForUnifiedResource.
type PrincipalsForUnifiedResourceOpts struct {
	// Resource is the enriched resource from the unified resource listing.
	Resource *types.EnrichedResource
	// CertPrincipals are the principals from the user's current certificate
	// (used to filter SSH logins to those the cert can actually use).
	CertPrincipals []string
	// AccessChecker is the user's base AccessChecker.
	AccessChecker services.AccessChecker
	// IncludeRequestable indicates the response should distinguish between
	// granted and requestable principals. When false, Granted == All.
	IncludeRequestable bool
	// UseSearchAsRoles indicates the request was made with search_as_roles,
	// meaning enriched logins may include requestable principals.
	UseSearchAsRoles bool
}

// PrincipalsForUnifiedResource computes the granted and requestable principals
// for a unified resource, based on the resource kind.
func PrincipalsForUnifiedResource(opts PrincipalsForUnifiedResourceOpts) (*UnifiedResourcePrincipals, error) {
	result := &UnifiedResourcePrincipals{}

	switch r := opts.Resource.ResourceWithLabels.(type) {
	case types.Server:
		logins, err := sshPrincipals(opts, r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result.Logins = logins
	case types.AppServer:
		arns, err := appPrincipals(opts, r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result.AWSRoleARNs = arns
	case types.DatabaseServer:
		result.DBPrincipalsByRole = databasePrincipalsByRole(opts, r)
		db := r.GetDatabase()
		// Determine auto-user status considering both granted and requestable roles.
		if mode, err := opts.AccessChecker.DatabaseAutoUserMode(db); err == nil {
			result.DBAutoUserEnabled = db.IsAutoUsersEnabled() && mode.IsEnabled()
		}
		// When showing requestable resources/principals, enriched db_roles from
		// search_as_roles indicate auto-create is enabled on the requestable
		// checker (CheckDatabaseRoles only returns non-empty when
		// CreateDatabaseUserMode is enabled on some role in the checker's RoleSet).
		if !result.DBAutoUserEnabled && (opts.IncludeRequestable || opts.UseSearchAsRoles) &&
			db.IsAutoUsersEnabled() && hasDBRolesInAnyEntry(opts.Resource.DatabasePrincipalsByRole) {
			result.DBAutoUserEnabled = true
		}
	}

	return result, nil
}

// sshPrincipals computes login principals for an SSH node.
//
// When search_as_roles is active (UseSearchAsRoles or IncludeRequestable),
// enriched logins may contain requestable logins not in the user's certificate.
// These are returned as-is since they're for display or access-request
// purposes, not direct SSH connections.
//
// When IncludeRequestable is set, granted logins are additionally computed
// using the base access checker and filtered to cert principals.
//
// In the default mode (neither flag set), all logins are filtered to cert
// principals so the connect menu only offers logins that will work.
func sshPrincipals(opts PrincipalsForUnifiedResourceOpts, server types.Server) (*webui.PrincipalSet, error) {
	if opts.UseSearchAsRoles || opts.IncludeRequestable {
		all := set.New(opts.Resource.Logins...)
		ps := &webui.PrincipalSet{All: all}
		if opts.IncludeRequestable {
			granted, err := opts.AccessChecker.GetAllowedLoginsForResource(server)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			ps.Granted = filterByIdentityPrincipals(opts.CertPrincipals, granted)
		} else {
			ps.Granted = all
		}
		return ps, nil
	}

	filtered := filterByIdentityPrincipals(opts.CertPrincipals, opts.Resource.Logins)
	return &webui.PrincipalSet{All: filtered, Granted: filtered}, nil
}

// appPrincipals computes AWS role ARN principals for an app resource.
//
// AccessChecker's [GetAllowedLoginsForResource] is used for backward compatibility
// in case Auth does not support enriched resources.
func appPrincipals(opts PrincipalsForUnifiedResourceOpts, appServer types.AppServer) (*webui.PrincipalSet, error) {
	// Get all visible ARNs (granted ∪ requestable).
	all := opts.Resource.Logins
	if len(all) == 0 {
		var err error
		all, err = opts.AccessChecker.GetAllowedLoginsForResource(appServer.GetApp())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	allSet := set.New(all...)
	ps := &webui.PrincipalSet{All: allSet}

	if opts.IncludeRequestable {
		granted, err := opts.AccessChecker.GetAllowedLoginsForResource(appServer.GetApp())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ps.Granted = set.New(granted...)
	} else {
		ps.Granted = allSet
	}

	return ps, nil
}

// databasePrincipalsByRole converts per-role database principal data from the
// enriched resource into web UI types, marking each role's principals as
// requiring a request if the role is not in the user's base (granted) role set.
func databasePrincipalsByRole(opts PrincipalsForUnifiedResourceOpts, _ types.DatabaseServer) map[string]webui.DatabaseRolePrincipalGroup {
	byRole := opts.Resource.DatabasePrincipalsByRole
	if len(byRole) == 0 {
		return nil
	}

	// Build a set of the user's base (granted) role names to determine
	// which enriched roles are granted vs. requestable.
	grantedRoles := set.New[string]()
	if opts.IncludeRequestable {
		for _, r := range opts.AccessChecker.Roles() {
			grantedRoles.Add(r.GetName())
		}
	}

	result := make(map[string]webui.DatabaseRolePrincipalGroup, len(byRole))
	for roleName, p := range byRole {
		requiresRequest := opts.IncludeRequestable && !grantedRoles.Contains(roleName)
		result[roleName] = webui.DatabaseRolePrincipalGroup{
			RequiresRequest: requiresRequest,
			Users:           p.Users,
			Names:           p.Names,
			Roles:           p.Roles,
		}
	}
	return result
}

// hasDBRolesInAnyEntry checks if any entry in the per-role map has non-empty Roles.
func hasDBRolesInAnyEntry(byRole map[string]types.DatabaseRolePrincipals) bool {
	for _, p := range byRole {
		if len(p.Roles) > 0 {
			return true
		}
	}
	return false
}

// filterByIdentityPrincipals returns the intersection of allowedLogins with
// identityPrincipals as a set. This is equivalent to client.CalculateSSHLogins.
func filterByIdentityPrincipals(identityPrincipals, allowedLogins []string) set.Set[string] {
	allowed := set.New(allowedLogins...)
	result := set.NewWithCapacity[string](len(identityPrincipals))
	for _, principal := range identityPrincipals {
		if allowed.Contains(principal) {
			result.Add(principal)
		}
	}
	return result
}
