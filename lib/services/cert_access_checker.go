/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package services

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

// UnscopedCertificateParameters represents a subset of the AccessChecker interface that
// is used during certificate generation to obtain certificate parameters that are only
// meaningful for unscoped identities.
type UnscopedCertificateParameters interface {
	RoleNames() []string
	CertificateFormat() string
	CertificateExtensions() []*types.CertExtension
	CheckDatabaseNamesAndUsers(ttl time.Duration, overrideTTL bool) ([]string, []string, error)
	CheckAWSRoleARNs(ttl time.Duration, overrideTTL bool) ([]string, error)
	CheckAzureIdentities(ttl time.Duration, overrideTTL bool) ([]string, error)
	CheckGCPServiceAccounts(ttl time.Duration, overrideTTL bool) ([]string, error)
	GetAllowedResourceAccessIDs() []types.ResourceAccessID
	CheckAccessToRemoteCluster(rc types.RemoteCluster) error
}

// CertificateParameterContext provides methods for resolving certificate parameters that abstract
// over scoped and unscoped identities. Methods on this type should only be called during certificate
// generation and return parameters that need to be embedded in the certificate at issuance time. For
// unscoped identities these parameters are generally equivalent to those returned by the underlying
// AccessChecker. For scoped identities things get more complex as most certificate parameters cannot
// be determined by scoped roles. Instead, parameters for scoped identities are generally hard-coded for
// the time being, with the intent to revisit them in the future and to provide non-role means of
// configuring them. See the Scopes RFD for more details on how scoped permissions intersect with
// certificate parameters.
type CertificateParameterContext struct {
	ctx *ScopedAccessCheckerContext
}

// UnscopedCertParams returns unscoped-specific certificate parameters if this is an unscoped
// identity, or nil if this is a scoped identity. Use this for certificate parameters
// that are only meaningful for unscoped identities (e.g., kube groups, db users).
func (c *CertificateParameterContext) UnscopedCertParams() UnscopedCertificateParameters {
	return c.ctx.unscopedChecker
}

// GetSSHLoginsForTTL verifies that the requested session TTL is valid and returns
// the list of allowed logins for the certificate.
//   - Unscoped: Returns logins from roles, restricted by role TTL rules
//   - Scoped: Returns all possible logins across all roles in the pin. this behavior is necessary
//     because we cannot determine the effective role without knowing the target resource, but the ssh
//     protocol requires all valid principals to be present in the certificate at issuance time. Subsequent
//     access checks will enforce login restrictions based on the effective role once the target resource
//     is known. Note that this function is *not* safe to determine the logins to be used for OpenSSH agent
//     access certs.
func (c *CertificateParameterContext) GetSSHLoginsForTTL(ctx context.Context, ttl time.Duration) ([]string, error) {
	if !c.ctx.isScoped() {
		return c.ctx.unscopedChecker.CheckLoginDuration(ttl)
	}

	// For scoped identities, enumerate all possible logins across all roles in the pin.
	// We cannot restrict logins based on a single role since we don't know which role will
	// grant access without knowing the target resource.
	loginSet := make(map[string]struct{})

	// Use of riskyEnumerateScopedCheckers is acceptable here because we are deliberately attempting to aggregate
	// information across all roles, rather than making a specific access-control decision.
	for checker, err := range c.ctx.riskyEnumerateScopedCheckers(ctx) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Get logins from this checker. Pass 0 as TTL to get all logins without TTL restriction.
		// We're not enforcing per-role TTL restrictions for scoped certs since the effective role
		// is unknown at cert generation time.
		for _, login := range checker.SSH().getScopedLogins() {
			// Skip placeholder logins when aggregating across roles
			if !strings.HasPrefix(login, constants.NoLoginPrefix) {
				loginSet[login] = struct{}{}
			}
		}
	}

	// Convert map to sorted slice for deterministic output
	logins := make([]string, 0, len(loginSet))
	for login := range loginSet {
		logins = append(logins, login)
	}
	slices.Sort(logins)

	if len(logins) == 0 {
		// User was deliberately configured to have no login capability,
		// but SSH certificates must contain at least one valid principal.
		// We add a single distinctive value which should be unique, and
		// will never be a valid unix login (due to leading '-').
		logins = []string{constants.NoLoginPrefix + uuid.New().String()}
	}

	return logins, nil
}

// AdjustSessionTTL adjusts the requested session TTL based on role/configuration policies.
func (c *CertificateParameterContext) AdjustSessionTTL(ttl time.Duration) time.Duration {
	if !c.ctx.isScoped() {
		return c.ctx.unscopedChecker.AdjustSessionTTL(ttl)
	}
	// Scoped identities: return the requested TTL unchanged. We cannot restrict TTL based on roles
	// since we don't know which role will grant access without knowing the target resource.
	// TODO(fspmarshall/scopes): determine how to handle session TTL restrictions for scoped identities. This will
	// likely involve fully decoupling session TTL and certificate TTL, since scoped cert TTLs will need to
	// be determined by non-role configuration, whereas specific resource access sessions may still be able to
	// be controlled by roles.
	return ttl
}

// PrivateKeyPolicy returns the private key policy to enforce for the certificate.
func (c *CertificateParameterContext) PrivateKeyPolicy(defaultPolicy keys.PrivateKeyPolicy) (keys.PrivateKeyPolicy, error) {
	if !c.ctx.isScoped() {
		return c.ctx.unscopedChecker.PrivateKeyPolicy(defaultPolicy)
	}
	// Scoped roles do not currently support custom private key policies. Return the cluster default.
	// TODO(fspmarshall/scopes): determine what (if any) control should permit setting the private key
	// policy for scoped certificates.
	return defaultPolicy, nil
}

// PinSourceIP returns whether source IP pinning should be enabled in the certificate.
func (c *CertificateParameterContext) PinSourceIP() bool {
	if !c.ctx.isScoped() {
		return c.ctx.unscopedChecker.PinSourceIP()
	}
	// Scoped identities do not support source IP pinning due to scope isolation concerns (we can't allow
	// to affect certificate parameters).
	// TODO(fspmarshall/scopes): determine what (if any) control should permit setting the source IP
	// pinning for scoped certificates. Likely this will need to be a cluster configuration rather than
	// a role-based setting, though perhapes enablement could be cluster-wide but enforcement could be
	// per-role.
	return false
}

// CanPortForward returns whether port forwarding should be permitted in the certificate.
func (c *CertificateParameterContext) CanPortForward() bool {
	if !c.ctx.isScoped() {
		return c.ctx.unscopedChecker.CanPortForward()
	}
	// Scoped identities: use unstable env var configuration
	// TODO(fspmarshall/scopes): determine what (if any) control should permit setting the port forwarding
	// permission for scoped certificates.
	return scopedaccess.UnstableGetScopedPortForwarding()
}

// CanForwardAgents returns whether agent forwarding should be permitted in the certificate.
func (c *CertificateParameterContext) CanForwardAgents() bool {
	if !c.ctx.isScoped() {
		return c.ctx.unscopedChecker.CanForwardAgents()
	}
	// Scoped identities: use unstable env var configuration
	// TODO(fspmarshall/scopes): determine what (if any) control should permit setting the agent forwarding
	// extension for scoped certificates.
	return scopedaccess.UnstableGetScopedForwardAgent()
}

// PermitX11Forwarding returns whether X11 forwarding should be permitted in the certificate.
func (c *CertificateParameterContext) PermitX11Forwarding() bool {
	if !c.ctx.isScoped() {
		return c.ctx.unscopedChecker.PermitX11Forwarding()
	}
	// Scoped identities: hard-coded to false (no unstable env var for X11 forwarding)
	// TODO(fspmarshall/scopes): determine what (if any) control should permit setting the X11 forwarding
	// permission for scoped certificates.
	return false
}

// LockingMode returns the locking mode to apply for the certificate.
func (c *CertificateParameterContext) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	if !c.ctx.isScoped() {
		return c.ctx.unscopedChecker.LockingMode(defaultMode)
	}
	// Scoped roles do not currently support custom locking modes. Return the default/cluster mode.
	// TODO(fspmarshall/scopes): determine how to handle locking mode for scoped certificates given that
	// role-affected locking behavior during certificate creation doesn't map well to pinned certificates.
	return defaultMode
}

// GetKubeGroupsAndUsersForTTL verifies that the requested session TTL is valid and returns the list of
// allowed groups and users for the certificate.
//   - Unscoped: Returns groups and users from roles, restricted by role TTL rules
//   - Scoped: Returns all possible groups and users across all roles in the pin. This behavior is necessary
//     because we cannot determine the effective role without knowing the target resource. Subsequent access
//     checks will enforce group and user restrictions based on the effective role once the target resource
//     is known.
func (c *CertificateParameterContext) GetKubeGroupsAndUsersForTTL(ctx context.Context, ttl time.Duration, overrideTTL bool, matchers ...RoleMatcher) ([]string, []string, error) {
	if !c.ctx.isScoped() {
		return c.ctx.unscopedChecker.CheckKubeGroupsAndUsers(ttl, overrideTTL, matchers...)
	}

	groupSet := make(map[string]struct{})
	userSet := make(map[string]struct{})
	for checker, err := range c.ctx.riskyEnumerateScopedCheckers(ctx) {
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		groups, users, err := checker.Kube().GetGroupsAndUsers(ttl, overrideTTL, matchers...)
		if err != nil {
			if !trace.IsAccessDenied(err) {
				continue
			}
			return nil, nil, trace.Wrap(err)
		}

		for _, group := range groups {
			groupSet[group] = struct{}{}
		}

		for _, user := range users {
			userSet[user] = struct{}{}
		}
	}

	return slices.Collect(maps.Keys(groupSet)), slices.Collect(maps.Keys(userSet)), nil
}
