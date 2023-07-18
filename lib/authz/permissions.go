/*
Copyright 2015-2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package authz

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/vulcand/predicate/builder"

	"github.com/gravitational/teleport"
	authpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// NewAdminContext returns new admin auth context
func NewAdminContext() (*Context, error) {
	return NewBuiltinRoleContext(types.RoleAdmin)
}

// NewBuiltinRoleContext create auth context for the provided builtin role.
func NewBuiltinRoleContext(role types.SystemRole) (*Context, error) {
	authContext, err := ContextForBuiltinRole(BuiltinRole{Role: role, Username: fmt.Sprintf("%v", role)}, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return authContext, nil
}

// AuthorizerOpts holds creation options for [NewAuthorizer].
type AuthorizerOpts struct {
	ClusterName string
	AccessPoint AuthorizerAccessPoint
	LockWatcher *services.LockWatcher
	Logger      logrus.FieldLogger

	// DisableDeviceAuthorization disables device authorization via [Authorizer].
	// It is meant for services that do explicit device authorization, like the
	// Auth Server APIs. Most services should not set this field.
	DisableDeviceAuthorization bool
}

// NewAuthorizer returns new authorizer using backends
func NewAuthorizer(opts AuthorizerOpts) (Authorizer, error) {
	if opts.ClusterName == "" {
		return nil, trace.BadParameter("missing parameter clusterName")
	}
	if opts.AccessPoint == nil {
		return nil, trace.BadParameter("missing parameter accessPoint")
	}
	logger := opts.Logger
	if logger == nil {
		logger = logrus.WithFields(logrus.Fields{trace.Component: "authorizer"})
	}
	return &authorizer{
		clusterName:                opts.ClusterName,
		accessPoint:                opts.AccessPoint,
		lockWatcher:                opts.LockWatcher,
		logger:                     logger,
		disableDeviceAuthorization: opts.DisableDeviceAuthorization,
	}, nil
}

// Authorizer authorizes identity and returns auth context
type Authorizer interface {
	// Authorize authorizes user based on identity supplied via context
	Authorize(ctx context.Context) (*Context, error)
}

// The AuthorizerFunc type is an adapter to allow the use of
// ordinary functions as an Authorizer. If f is a function
// with the appropriate signature, AuthorizerFunc(f) is a
// Authorizer that calls f.
type AuthorizerFunc func(ctx context.Context) (*Context, error)

// Authorize calls f(ctx).
func (f AuthorizerFunc) Authorize(ctx context.Context) (*Context, error) {
	return f(ctx)
}

// AuthorizerAccessPoint is the access point contract required by an Authorizer
type AuthorizerAccessPoint interface {
	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetUser returns a services.User for this cluster.
	GetUser(name string, withSecrets bool) (types.User, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)
}

// authorizer creates new local authorizer
type authorizer struct {
	clusterName                string
	accessPoint                AuthorizerAccessPoint
	lockWatcher                *services.LockWatcher
	disableDeviceAuthorization bool
	logger                     logrus.FieldLogger
}

// Context is authorization context
type Context struct {
	// User is the username
	User types.User
	// Checker is access checker
	Checker services.AccessChecker
	// Identity holds the caller identity:
	// 1. If caller is a user
	//   a. local user identity
	//   b. remote user identity remapped to local identity based on trusted
	//      cluster role mapping.
	// 2. If caller is a teleport instance, Identity holds their identity as-is
	//    (because there's no role mapping for non-human roles)
	Identity IdentityGetter
	// UnmappedIdentity holds the original caller identity. If this is a remote
	// user, UnmappedIdentity holds the data before role mapping. Otherwise,
	// it's identical to Identity.
	UnmappedIdentity IdentityGetter

	// disableDeviceAuthorization disables device verification.
	// Inherited from the authorizer that creates the context.
	disableDeviceAuthorization bool
}

// LockTargets returns a list of LockTargets inferred from the context's
// Identity and UnmappedIdentity.
func (c *Context) LockTargets() []types.LockTarget {
	lockTargets := services.LockTargetsFromTLSIdentity(c.Identity.GetIdentity())
Loop:
	for _, unmappedTarget := range services.LockTargetsFromTLSIdentity(c.UnmappedIdentity.GetIdentity()) {
		// Append a lock target from UnmappedIdentity only if it is not already
		// known from Identity.
		for _, knownTarget := range lockTargets {
			if unmappedTarget.Equals(knownTarget) {
				continue Loop
			}
		}
		lockTargets = append(lockTargets, unmappedTarget)
	}
	if r, ok := c.Identity.(BuiltinRole); ok {
		switch r.Role {
		// Node role is a special case because it was previously suported as a
		// lock target that only locked the `ssh_service`. If the same Teleport server
		// had multiple roles, Node lock would only lock the `ssh_service` while
		// other roles would be able to authenticate into Teleport without a problem.
		// To remove the ambiguity, we now lock the entire Teleport server for
		// all roles using the LockTarget.ServerID field and `Node` field is
		// deprecated.
		// In order to support legacy behavior, we need fill in both `ServerID`
		// and `Node` fields if the role is `Node` so that the previous behavior
		// is preserved.
		// This is a legacy behavior that we need to support for backwards compatibility.
		case types.RoleNode:
			lockTargets = append(lockTargets,
				types.LockTarget{Node: r.GetServerID(), ServerID: r.GetServerID()},
				types.LockTarget{Node: r.Identity.Username, ServerID: r.Identity.Username},
			)
		default:
			lockTargets = append(lockTargets,
				types.LockTarget{ServerID: r.GetServerID()},
				types.LockTarget{ServerID: r.Identity.Username},
			)
		}
	}
	return lockTargets
}

// WithExtraRoles returns a shallow copy of [c], where the users roles have been
// extended with [roles]. It may return [c] unmodified.
func (c *Context) WithExtraRoles(access services.RoleGetter, clusterName string, roles []string) (*Context, error) {
	var newRoleNames []string
	newRoleNames = append(newRoleNames, c.Checker.RoleNames()...)
	newRoleNames = append(newRoleNames, roles...)
	newRoleNames = utils.Deduplicate(newRoleNames)

	// Return early if there are no extra roles.
	if len(newRoleNames) == len(c.Checker.RoleNames()) {
		return c, nil
	}

	accessInfo := &services.AccessInfo{
		Roles:              newRoleNames,
		Traits:             c.User.GetTraits(),
		AllowedResourceIDs: c.Checker.GetAllowedResourceIDs(),
	}
	checker, err := services.NewAccessChecker(accessInfo, clusterName, access)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newContext := *c
	newContext.Checker = checker
	return &newContext, nil
}

// GetAccessState returns the AccessState based on the underlying
// [services.AccessChecker] and [tlsca.Identity].
func (c *Context) GetAccessState(authPref types.AuthPreference) services.AccessState {
	state := c.Checker.GetAccessState(authPref)
	identity := c.Identity.GetIdentity()

	// Builtin services (like proxy_service and kube_service) are not gated
	// on MFA and only need to pass normal RBAC action checks.
	_, isService := c.Identity.(BuiltinRole)
	state.MFAVerified = isService || identity.MFAVerified != ""

	state.EnableDeviceVerification = !c.disableDeviceAuthorization
	state.DeviceVerified = isService || dtauthz.IsTLSDeviceVerified(&identity.DeviceExtensions)

	return state
}

// Authorize authorizes user based on identity supplied via context
func (a *authorizer) Authorize(ctx context.Context) (*Context, error) {
	if ctx == nil {
		return nil, trace.AccessDenied("missing authentication context")
	}
	userI, err := UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authContext, err := a.fromUser(ctx, userI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := CheckIPPinning(ctx, authContext.Identity.GetIdentity(), authContext.Checker.PinSourceIP(), a.logger); err != nil {
		return nil, trace.Wrap(err)
	}

	// Enforce applicable locks.
	authPref, err := a.accessPoint.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if lockErr := a.lockWatcher.CheckLockInForce(
		authContext.Checker.LockingMode(authPref.GetLockingMode()),
		authContext.LockTargets()...); lockErr != nil {
		return nil, trace.Wrap(lockErr)
	}

	// Enforce required private key policy if set.
	if err := a.enforcePrivateKeyPolicy(ctx, authContext, authPref); err != nil {
		return nil, trace.Wrap(err)
	}

	// Device Trust: authorize device extensions.
	if !a.disableDeviceAuthorization {
		if err := dtauthz.VerifyTLSUser(authPref.GetDeviceTrust(), authContext.Identity.GetIdentity()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return authContext, nil
}

func (a *authorizer) enforcePrivateKeyPolicy(ctx context.Context, authContext *Context, authPref types.AuthPreference) error {
	switch authContext.Identity.(type) {
	case BuiltinRole, RemoteBuiltinRole:
		// built in roles do not need to pass private key policies
		return nil
	}

	// Check that the required private key policy, defined by roles and auth pref,
	// is met by this Identity's tls certificate.
	identityPolicy := authContext.Identity.GetIdentity().PrivateKeyPolicy
	requiredPolicy := authContext.Checker.PrivateKeyPolicy(authPref.GetPrivateKeyPolicy())
	if err := requiredPolicy.VerifyPolicy(identityPolicy); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *authorizer) fromUser(ctx context.Context, userI interface{}) (*Context, error) {
	switch user := userI.(type) {
	case LocalUser:
		return a.authorizeLocalUser(user)
	case RemoteUser:
		return a.authorizeRemoteUser(ctx, user)
	case BuiltinRole:
		return a.authorizeBuiltinRole(ctx, user)
	case RemoteBuiltinRole:
		return a.authorizeRemoteBuiltinRole(user)
	default:
		return nil, trace.AccessDenied("unsupported context type %T", userI)
	}
}

// ErrIPPinningMissing is returned when user cert should be pinned but isn't.
var ErrIPPinningMissing = trace.AccessDenied("pinned IP is required for the user, but is not present on identity")

// ErrIPPinningMismatch is returned when user's pinned IP doesn't match observed IP.
var ErrIPPinningMismatch = trace.AccessDenied("pinned IP doesn't match observed client IP")

// CheckIPPinning verifies IP pinning for the identity, using the client IP taken from context.
// Check is considered successful if no error is returned.
func CheckIPPinning(ctx context.Context, identity tlsca.Identity, pinSourceIP bool, log logrus.FieldLogger) error {
	if identity.PinnedIP == "" {
		if pinSourceIP {
			return ErrIPPinningMissing
		}
		return nil
	}

	clientSrcAddr, err := ClientAddrFromContext(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	clientIP, _, err := net.SplitHostPort(clientSrcAddr.String())
	if err != nil {
		return trace.Wrap(err)
	}

	if clientIP != identity.PinnedIP {
		if log != nil {
			log.WithFields(logrus.Fields{
				"client_ip": clientIP,
				"pinned_ip": identity.PinnedIP,
			}).Debug("Pinned IP and client IP mismatch")
		}
		return ErrIPPinningMismatch
	}

	return nil
}

// authorizeLocalUser returns authz context based on the username
func (a *authorizer) authorizeLocalUser(u LocalUser) (*Context, error) {
	return ContextForLocalUser(u, a.accessPoint, a.clusterName, a.disableDeviceAuthorization)
}

// authorizeRemoteUser returns checker based on cert authority roles
func (a *authorizer) authorizeRemoteUser(ctx context.Context, u RemoteUser) (*Context, error) {
	ca, err := a.accessPoint.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.UserCA,
		DomainName: u.ClusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessInfo, err := services.AccessInfoFromRemoteIdentity(u.Identity, ca.CombinedMapping())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, a.clusterName, a.accessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The user is prefixed with "remote-" and suffixed with cluster name with
	// the hope that it does not match a real local user.
	user, err := types.NewUser(fmt.Sprintf("remote-%v-%v", u.Username, u.ClusterName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetTraits(accessInfo.Traits)
	user.SetRoles(accessInfo.Roles)

	// Adjust expiry based on locally mapped roles.
	ttl := time.Until(u.Identity.Expires)
	ttl = checker.AdjustSessionTTL(ttl)
	var previousIdentityExpires time.Time
	if u.Identity.MFAVerified != "" {
		prevIdentityTTL := time.Until(u.Identity.PreviousIdentityExpires)
		prevIdentityTTL = checker.AdjustSessionTTL(prevIdentityTTL)
		previousIdentityExpires = time.Now().Add(prevIdentityTTL)
	}

	kubeUsers, kubeGroups, err := checker.CheckKubeGroupsAndUsers(ttl, false)
	// IsNotFound means that the user has no k8s users or groups, which is fine
	// in many cases. The downstream k8s handler will ensure that users/groups
	// are set if this is a k8s request.
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	principals, err := checker.CheckLoginDuration(ttl)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Convert u.Identity into the mapped local identity.
	//
	// This prevents downstream users from accidentally using the unmapped
	// identity information and confusing who's accessing a resource.
	identity := tlsca.Identity{
		Username:                user.GetName(),
		Groups:                  user.GetRoles(),
		Traits:                  accessInfo.Traits,
		Principals:              principals,
		KubernetesGroups:        kubeGroups,
		KubernetesUsers:         kubeUsers,
		TeleportCluster:         a.clusterName,
		Expires:                 time.Now().Add(ttl),
		PreviousIdentityExpires: previousIdentityExpires,

		// These fields are for routing and restrictions, safe to re-use from
		// unmapped identity.
		Usage:             u.Identity.Usage,
		RouteToCluster:    u.Identity.RouteToCluster,
		KubernetesCluster: u.Identity.KubernetesCluster,
		RouteToApp:        u.Identity.RouteToApp,
		RouteToDatabase:   u.Identity.RouteToDatabase,
		MFAVerified:       u.Identity.MFAVerified,
		LoginIP:           u.Identity.LoginIP,
		PinnedIP:          u.Identity.PinnedIP,
		PrivateKeyPolicy:  u.Identity.PrivateKeyPolicy,
		UserType:          u.Identity.UserType,
	}
	if checker.PinSourceIP() && identity.PinnedIP == "" {
		return nil, ErrIPPinningMissing
	}

	return &Context{
		User:                       user,
		Checker:                    checker,
		Identity:                   WrapIdentity(identity),
		UnmappedIdentity:           u,
		disableDeviceAuthorization: a.disableDeviceAuthorization,
	}, nil
}

// authorizeBuiltinRole authorizes builtin role
func (a *authorizer) authorizeBuiltinRole(ctx context.Context, r BuiltinRole) (*Context, error) {
	recConfig, err := a.accessPoint.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ContextForBuiltinRole(r, recConfig)
}

func (a *authorizer) authorizeRemoteBuiltinRole(r RemoteBuiltinRole) (*Context, error) {
	if r.Role != types.RoleProxy {
		return nil, trace.AccessDenied("access denied for remote %v connecting to cluster", r.Role)
	}
	roleSet, err := services.RoleSetFromSpec(
		string(types.RoleRemoteProxy),
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:       []string{types.Wildcard},
				NodeLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
				AppLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
				KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					types.NewRule(types.KindNode, services.RO()),
					types.NewRule(types.KindProxy, services.RO()),
					types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
					types.NewRule(types.KindNamespace, services.RO()),
					types.NewRule(types.KindUser, services.RO()),
					types.NewRule(types.KindRole, services.RO()),
					types.NewRule(types.KindAuthServer, services.RO()),
					types.NewRule(types.KindReverseTunnel, services.RO()),
					types.NewRule(types.KindTunnelConnection, services.RO()),
					types.NewRule(types.KindClusterName, services.RO()),
					types.NewRule(types.KindClusterAuditConfig, services.RO()),
					types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
					types.NewRule(types.KindSessionRecordingConfig, services.RO()),
					types.NewRule(types.KindClusterAuthPreference, services.RO()),
					types.NewRule(types.KindKubeServer, services.RO()),
					types.NewRule(types.KindInstaller, services.RO()),
					types.NewRule(types.KindUIConfig, services.RO()),
					types.NewRule(types.KindDatabaseService, services.RO()),
					// this rule allows remote proxy to update the cluster's certificate authorities
					// during certificates renewal
					{
						Resources: []string{types.KindCertAuthority},
						// It is important that remote proxy can only rotate
						// existing certificate authority, and not create or update new ones
						Verbs: []string{types.VerbRead, types.VerbRotate},
						// allow administrative access to the certificate authority names
						// matching the cluster name only
						Where: builder.Equals(services.ResourceNameExpr, builder.String(r.ClusterName)).String(),
					},
				},
			},
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := types.NewUser(r.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles := []string{string(types.RoleRemoteProxy)}
	user.SetRoles(roles)
	checker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
		Roles:              roles,
		Traits:             nil,
		AllowedResourceIDs: nil,
	}, a.clusterName, roleSet)
	return &Context{
		User:                       user,
		Checker:                    checker,
		Identity:                   r,
		UnmappedIdentity:           r,
		disableDeviceAuthorization: a.disableDeviceAuthorization,
	}, nil
}

func roleSpecForProxyWithRecordAtProxy(clusterName string) types.RoleSpecV6 {
	base := roleSpecForProxy(clusterName)
	base.Allow.Rules = append(base.Allow.Rules, types.NewRule(types.KindHostCert, services.RW()))
	return base
}

func roleSpecForProxy(clusterName string) types.RoleSpecV6 {
	return types.RoleSpecV6{
		Allow: types.RoleConditions{
			Namespaces:            []string{types.Wildcard},
			ClusterLabels:         types.Labels{types.Wildcard: []string{types.Wildcard}},
			NodeLabels:            types.Labels{types.Wildcard: []string{types.Wildcard}},
			AppLabels:             types.Labels{types.Wildcard: []string{types.Wildcard}},
			DatabaseLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
			DatabaseServiceLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			KubernetesLabels:      types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules: []types.Rule{
				types.NewRule(types.KindProxy, services.RW()),
				types.NewRule(types.KindOIDCRequest, services.RW()),
				types.NewRule(types.KindSSHSession, services.RW()),
				types.NewRule(types.KindSession, services.RO()),
				types.NewRule(types.KindEvent, services.RW()),
				types.NewRule(types.KindSAMLRequest, services.RW()),
				types.NewRule(types.KindOIDC, services.ReadNoSecrets()),
				types.NewRule(types.KindSAML, services.ReadNoSecrets()),
				types.NewRule(types.KindGithub, services.ReadNoSecrets()),
				types.NewRule(types.KindGithubRequest, services.RW()),
				types.NewRule(types.KindNamespace, services.RO()),
				types.NewRule(types.KindNode, services.RO()),
				types.NewRule(types.KindAuthServer, services.RO()),
				types.NewRule(types.KindReverseTunnel, services.RO()),
				types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
				types.NewRule(types.KindUser, services.RO()),
				types.NewRule(types.KindRole, services.RO()),
				types.NewRule(types.KindClusterAuthPreference, services.RO()),
				types.NewRule(types.KindClusterName, services.RO()),
				types.NewRule(types.KindClusterAuditConfig, services.RO()),
				types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
				types.NewRule(types.KindSessionRecordingConfig, services.RO()),
				types.NewRule(types.KindUIConfig, services.RO()),
				types.NewRule(types.KindStaticTokens, services.RO()),
				types.NewRule(types.KindTunnelConnection, services.RW()),
				types.NewRule(types.KindRemoteCluster, services.RO()),
				types.NewRule(types.KindSemaphore, services.RW()),
				types.NewRule(types.KindAppServer, services.RO()),
				types.NewRule(types.KindWebSession, services.RW()),
				types.NewRule(types.KindWebToken, services.RW()),
				types.NewRule(types.KindKubeServer, services.RW()),
				types.NewRule(types.KindDatabaseServer, services.RO()),
				types.NewRule(types.KindLock, services.RO()),
				types.NewRule(types.KindToken, []string{types.VerbRead, types.VerbDelete}),
				types.NewRule(types.KindWindowsDesktopService, services.RO()),
				types.NewRule(types.KindDatabaseCertificate, []string{types.VerbCreate}),
				types.NewRule(types.KindWindowsDesktop, services.RO()),
				types.NewRule(types.KindInstaller, services.RO()),
				types.NewRule(types.KindConnectionDiagnostic, services.RW()),
				types.NewRule(types.KindDatabaseService, services.RO()),
				types.NewRule(types.KindSAMLIdPServiceProvider, services.RO()),
				types.NewRule(types.KindUserGroup, services.RO()),
				types.NewRule(types.KindIntegration, services.RO()),
				// this rule allows cloud proxies to read
				// plugins of `openai` type, since Assist uses the OpenAI API and runs in Proxy.
				{
					Resources: []string{types.KindPlugin},
					Verbs:     []string{types.VerbRead},
					Where: builder.Equals(
						builder.Identifier(`resource.metadata.labels["type"]`),
						builder.String("openai"),
					).String(),
				},
				// this rule allows local proxy to update the remote cluster's host certificate authorities
				// during certificates renewal
				{
					Resources: []string{types.KindCertAuthority},
					Verbs:     []string{types.VerbCreate, types.VerbRead, types.VerbUpdate},
					// allow administrative access to the host certificate authorities
					// matching any cluster name except local
					Where: builder.And(
						builder.Equals(services.CertAuthorityTypeExpr, builder.String(string(types.HostCA))),
						builder.Not(
							builder.Equals(
								services.ResourceNameExpr,
								builder.String(clusterName),
							),
						),
					).String(),
				},
			},
		},
	}
}

// RoleSetForBuiltinRoles returns RoleSet for embedded builtin role
func RoleSetForBuiltinRoles(clusterName string, recConfig types.SessionRecordingConfig, roles ...types.SystemRole) (services.RoleSet, error) {
	var definitions []types.Role
	for _, role := range roles {
		rd, err := definitionForBuiltinRole(clusterName, recConfig, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		definitions = append(definitions, rd)
	}
	return services.NewRoleSet(definitions...), nil
}

// definitionForBuiltinRole constructs the appropriate role definition for a given builtin role.
func definitionForBuiltinRole(clusterName string, recConfig types.SessionRecordingConfig, role types.SystemRole) (types.Role, error) {
	switch role {
	case types.RoleAuth:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindAuthServer, services.RW()),
					},
				},
			})
	case types.RoleProvisionToken:
		return services.RoleFromSpec(role.String(), types.RoleSpecV6{})
	case types.RoleNode:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindNode, services.RW()),
						types.NewRule(types.KindSSHSession, services.RW()),
						types.NewRule(types.KindSession, services.RO()),
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindProxy, services.RO()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindAuthServer, services.RO()),
						types.NewRule(types.KindReverseTunnel, services.RW()),
						types.NewRule(types.KindTunnelConnection, services.RO()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindClusterAuditConfig, services.RO()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindSessionRecordingConfig, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindSemaphore, services.RW()),
						types.NewRule(types.KindLock, services.RO()),
						types.NewRule(types.KindNetworkRestrictions, services.RO()),
						types.NewRule(types.KindConnectionDiagnostic, services.RW()),
					},
				},
			})
	case types.RoleApp:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					AppLabels:  types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindProxy, services.RO()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindAuthServer, services.RO()),
						types.NewRule(types.KindReverseTunnel, services.RW()),
						types.NewRule(types.KindTunnelConnection, services.RO()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindClusterAuditConfig, services.RO()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindSessionRecordingConfig, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindAppServer, services.RW()),
						types.NewRule(types.KindApp, services.RO()),
						types.NewRule(types.KindWebSession, services.RO()),
						types.NewRule(types.KindWebToken, services.RO()),
						types.NewRule(types.KindJWT, services.RW()),
						types.NewRule(types.KindLock, services.RO()),
					},
				},
			})
	case types.RoleDatabase:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces:     []string{types.Wildcard},
					DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindProxy, services.RO()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindAuthServer, services.RO()),
						types.NewRule(types.KindReverseTunnel, services.RW()),
						types.NewRule(types.KindTunnelConnection, services.RO()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindClusterAuditConfig, services.RO()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindSessionRecordingConfig, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindDatabaseServer, services.RW()),
						types.NewRule(types.KindDatabaseService, services.RW()),
						types.NewRule(types.KindDatabase, services.RO()),
						types.NewRule(types.KindSemaphore, services.RW()),
						types.NewRule(types.KindLock, services.RO()),
						types.NewRule(types.KindConnectionDiagnostic, services.RW()),
					},
				},
			})
	case types.RoleProxy:
		// to support connecting to Agentless nodes, proxy needs to be
		// able to generate host certificates.
		return services.RoleFromSpec(
			role.String(),
			roleSpecForProxyWithRecordAtProxy(clusterName),
		)
	case types.RoleSignup:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindAuthServer, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
					},
				},
			})
	case types.RoleAdmin:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Options: types.RoleOptions{
					MaxSessionTTL: types.MaxDuration(),
				},
				Allow: types.RoleConditions{
					Namespaces:            []string{types.Wildcard},
					Logins:                []string{},
					NodeLabels:            types.Labels{types.Wildcard: []string{types.Wildcard}},
					AppLabels:             types.Labels{types.Wildcard: []string{types.Wildcard}},
					GroupLabels:           types.Labels{types.Wildcard: []string{types.Wildcard}},
					KubernetesLabels:      types.Labels{types.Wildcard: []string{types.Wildcard}},
					DatabaseLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
					DatabaseServiceLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					ClusterLabels:         types.Labels{types.Wildcard: []string{types.Wildcard}},
					WindowsDesktopLabels:  types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.Wildcard, services.RW()),
						types.NewRule(types.KindDevice, append(services.RW(), types.VerbCreateEnrollToken, types.VerbEnroll)),
					},
				},
			})
	case types.RoleNop:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces: []string{},
					Rules:      []types.Rule{},
				},
			})
	case types.RoleKube:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces:       []string{types.Wildcard},
					KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindKubeServer, services.RW()),
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindClusterAuditConfig, services.RO()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindSessionRecordingConfig, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindLock, services.RO()),
						types.NewRule(types.KindKubernetesCluster, services.RO()),
						types.NewRule(types.KindSemaphore, services.RW()),
					},
				},
			})
	case types.RoleWindowsDesktop:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces:           []string{types.Wildcard},
					WindowsDesktopLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindClusterAuditConfig, services.RO()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindSessionRecordingConfig, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindLock, services.RO()),
						types.NewRule(types.KindWindowsDesktopService, services.RW()),
						types.NewRule(types.KindWindowsDesktop, services.RW()),
					},
				},
			})
	case types.RoleDiscovery:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindNode, services.RO()),
						types.NewRule(types.KindKubernetesCluster, services.RW()),
						types.NewRule(types.KindDatabase, services.RW()),
						types.NewRule(types.KindServerInfo, services.RW()),
					},
					// wildcard any cluster available.
					KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					DatabaseLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
				},
			})
	case types.RoleOkta:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces:  []string{types.Wildcard},
					AppLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
					GroupLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindSemaphore, services.RW()),
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindAppServer, services.RW()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindUserGroup, services.RW()),
						types.NewRule(types.KindOktaImportRule, services.RO()),
						types.NewRule(types.KindOktaAssignment, services.RW()),
						types.NewRule(types.KindProxy, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindLock, services.RO()),
					},
				},
			})
	case types.RoleMDM:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindDevice, services.RW()),
					},
				},
			})
	}

	return nil, trace.NotFound("builtin role %q is not recognized", role.String())
}

// ContextForBuiltinRole returns a context with the builtin role information embedded.
func ContextForBuiltinRole(r BuiltinRole, recConfig types.SessionRecordingConfig) (*Context, error) {
	var systemRoles []types.SystemRole
	if r.Role == types.RoleInstance {
		// instance certs encode multiple system roles in a separate field
		systemRoles = r.AdditionalSystemRoles
		if len(systemRoles) == 0 {
			// note: previous parsing skipped unknown roles for this field, so its possible that some
			// system roles were defined, but they were all unknown to us.
			return nil, trace.BadParameter("cannot create instance context, no additional system roles recognized")
		}
	} else {
		// all other certs encode a single system role
		systemRoles = []types.SystemRole{r.Role}
	}
	roleSet, err := RoleSetForBuiltinRoles(r.ClusterName, recConfig, systemRoles...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := types.NewUser(r.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var roles []string
	for _, r := range systemRoles {
		roles = append(roles, string(r))
	}
	user.SetRoles(roles)
	checker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
		Roles:              roles,
		Traits:             nil,
		AllowedResourceIDs: nil,
	}, r.ClusterName, roleSet)
	return &Context{
		User:                       user,
		Checker:                    checker,
		Identity:                   r,
		UnmappedIdentity:           r,
		disableDeviceAuthorization: true, // Builtin roles skip device trust.
	}, nil
}

// ContextForLocalUser returns a context with the local user info embedded.
func ContextForLocalUser(u LocalUser, accessPoint AuthorizerAccessPoint, clusterName string, disableDeviceAuthz bool) (*Context, error) {
	// User has to be fetched to check if it's a blocked username
	user, err := accessPoint.GetUser(u.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessInfo, err := services.AccessInfoFromLocalIdentity(u.Identity, accessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessChecker, err := services.NewAccessChecker(accessInfo, clusterName, accessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Override roles and traits from the local user based on the identity roles
	// and traits, this is done to prevent potential conflict. Imagine a scenario
	// when SSO user has left the company, but local user entry remained with old
	// privileged roles. New user with the same name has been onboarded and would
	// have derived the roles from the stale user entry. This code prevents
	// that by extracting up to date identity traits and roles from the user's
	// certificate metadata.
	user.SetRoles(accessInfo.Roles)
	user.SetTraits(accessInfo.Traits)

	return &Context{
		User:                       user,
		Checker:                    accessChecker,
		Identity:                   u,
		UnmappedIdentity:           u,
		disableDeviceAuthorization: disableDeviceAuthz,
	}, nil
}

type contextKey string

const (
	// contextUserCertificate is the X.509 certificate used by the contextUser to
	// establish the mTLS connection.
	// Holds a *x509.Certificate.
	contextUserCertificate contextKey = "teleport-user-cert"

	// contextUser is a user set in the context of the request
	contextUser contextKey = "teleport-user"

	// contextClientAddr is a client address set in the context of the request
	contextClientAddr contextKey = "client-addr"

	// contextMFAResponse is an MFA challenge response set in the context of the request
	contextMFAResponse contextKey = "mfa-response"
)

// WithDelegator alias for backwards compatibility
var WithDelegator = utils.WithDelegator

// ClientUsername returns the username of a remote HTTP client making the call.
// If ctx didn't pass through auth middleware or did not come from an HTTP
// request, teleport.UserSystem is returned.
func ClientUsername(ctx context.Context) string {
	userWithIdentity, err := UserFromContext(ctx)
	if err != nil {
		return teleport.UserSystem
	}
	identity := userWithIdentity.GetIdentity()
	if identity.Username == "" {
		return teleport.UserSystem
	}
	return identity.Username
}

func userIdentityFromContext(ctx context.Context) (*tlsca.Identity, error) {
	userWithIdentity, err := UserFromContext(ctx)
	if err != nil {
		return nil, trace.AccessDenied("missing identity")
	}

	identity := userWithIdentity.GetIdentity()
	if identity.Username == "" {
		return nil, trace.AccessDenied("missing identity username")
	}

	return &identity, nil
}

// GetClientUsername returns the username of a remote HTTP client making the call.
// If ctx didn't pass through auth middleware or did not come from an HTTP
// request, returns an error.
func GetClientUsername(ctx context.Context) (string, error) {
	identity, err := userIdentityFromContext(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return identity.Username, nil
}

// GetClientUserIsSSO extracts the identity of a remote HTTP client and indicates whether that is an SSO user.
// If ctx didn't pass through auth middleware or did not come from an HTTP
// request, returns an error.
func GetClientUserIsSSO(ctx context.Context) (bool, error) {
	identity, err := userIdentityFromContext(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return identity.UserType == types.UserTypeSSO, nil
}

// ClientImpersonator returns the impersonator username of a remote client
// making the call. If not present, returns an empty string
func ClientImpersonator(ctx context.Context) string {
	userWithIdentity, err := UserFromContext(ctx)
	if err != nil {
		return ""
	}
	identity := userWithIdentity.GetIdentity()
	return identity.Impersonator
}

// ClientUserMetadata returns a UserMetadata suitable for events caused by a
// remote client making a call. If ctx didn't pass through auth middleware or
// did not come from an HTTP request, metadata for teleport.UserSystem is
// returned.
func ClientUserMetadata(ctx context.Context) apievents.UserMetadata {
	identityGetter, err := UserFromContext(ctx)
	if err != nil {
		return apievents.UserMetadata{
			User: teleport.UserSystem,
		}
	}
	meta := identityGetter.GetIdentity().GetUserMetadata()
	if meta.User == "" {
		meta.User = teleport.UserSystem
	}
	return meta
}

// ClientUserMetadataWithUser returns a UserMetadata suitable for events caused
// by a remote client making a call, with the specified username overriding the one
// from the remote client.
func ClientUserMetadataWithUser(ctx context.Context, user string) apievents.UserMetadata {
	meta := ClientUserMetadata(ctx)
	meta.User = user
	return meta
}

// ConvertAuthorizerError will take an authorizer error and convert it into an error easily
// handled by gRPC services.
func ConvertAuthorizerError(ctx context.Context, log logrus.FieldLogger, err error) error {
	switch {
	case err == nil:
		return nil
	// propagate connection problem error so we can differentiate
	// between connection failed and access denied
	case trace.IsConnectionProblem(err):
		return trace.ConnectionProblem(err, "failed to connect to the database")
	case trace.IsNotFound(err):
		// user not found, wrap error with access denied
		return trace.Wrap(err, "access denied")
	case errors.Is(err, ErrIPPinningMissing) || errors.Is(err, ErrIPPinningMismatch):
		log.Warn(err)
		return trace.Wrap(err)
	case trace.IsAccessDenied(err):
		// don't print stack trace, just log the warning
		log.Warn(err)
	case keys.IsPrivateKeyPolicyError(err):
		// private key policy errors should be returned to the client
		// unaltered so that they know to reauthenticate with a valid key.
		return trace.Unwrap(err)
	default:
		log.Warn(trace.DebugReport(err))
	}
	return trace.AccessDenied("access denied")
}

// AuthorizeResourceWithVerbs will ensure that the user has access to the given verbs for the given kind.
func AuthorizeResourceWithVerbs(ctx context.Context, log logrus.FieldLogger, authorizer Authorizer, quiet bool, resource types.Resource, verbs ...string) (*Context, error) {
	authCtx, err := authorizer.Authorize(ctx)
	if err != nil {
		return nil, ConvertAuthorizerError(ctx, log, err)
	}

	ruleCtx := &services.Context{
		User:     authCtx.User,
		Resource: resource,
	}

	return authorizeContextWithVerbs(ctx, log, authCtx, quiet, ruleCtx, resource.GetKind(), verbs...)
}

// AuthorizeWithVerbs will ensure that the user has access to the given verbs for the given kind.
func AuthorizeWithVerbs(ctx context.Context, log logrus.FieldLogger, authorizer Authorizer, quiet bool, kind string, verbs ...string) (*Context, error) {
	authCtx, err := authorizer.Authorize(ctx)
	if err != nil {
		return nil, ConvertAuthorizerError(ctx, log, err)
	}

	ruleCtx := &services.Context{
		User: authCtx.User,
	}

	return authorizeContextWithVerbs(ctx, log, authCtx, quiet, ruleCtx, kind, verbs...)
}

// authorizeContextWithVerbs will ensure that the user has access to the given verbs for the given services.context.
func authorizeContextWithVerbs(ctx context.Context, log logrus.FieldLogger, authCtx *Context, quiet bool, ruleCtx *services.Context, kind string, verbs ...string) (*Context, error) {
	errs := make([]error, len(verbs))
	for i, verb := range verbs {
		errs[i] = authCtx.Checker.CheckAccessToRule(ruleCtx, defaults.Namespace, kind, verb, quiet)
	}

	// Convert generic aggregate error to AccessDenied (auth_with_roles also does this).
	if err := trace.NewAggregate(errs...); err != nil {
		return nil, trace.AccessDenied(err.Error())
	}
	return authCtx, nil
}

// LocalUser is a local user
type LocalUser struct {
	// Username is local username
	Username string
	// Identity is x509-derived identity used to build this user
	Identity tlsca.Identity
}

// GetIdentity returns client identity
func (l LocalUser) GetIdentity() tlsca.Identity {
	return l.Identity
}

// IdentityGetter returns the unmapped client identity.
//
// Unmapped means that if the client is a remote cluster user, the returned
// tlsca.Identity contains data from the remote cluster before role mapping is
// applied.
type IdentityGetter interface {
	// GetIdentity  returns x509-derived identity of the user
	GetIdentity() tlsca.Identity
}

// WrapIdentity wraps identity to return identity getter function
type WrapIdentity tlsca.Identity

// GetIdentity returns identity
func (i WrapIdentity) GetIdentity() tlsca.Identity {
	return tlsca.Identity(i)
}

// BuiltinRole is the role of the Teleport service.
type BuiltinRole struct {
	// Role is the primary builtin role this username is associated with
	Role types.SystemRole

	// AdditionalSystemRoles is a collection of additional system roles held by
	// this identity (only currently used by identities with RoleInstance as their
	// primary role).
	AdditionalSystemRoles types.SystemRoles

	// Username is for authentication tracking purposes
	Username string

	// ClusterName is the name of the local cluster
	ClusterName string

	// Identity is source x509 used to build this role
	Identity tlsca.Identity
}

// IsServer returns true if the primary role is either RoleInstance, or one of
// the local service roles (e.g. proxy).
func (r BuiltinRole) IsServer() bool {
	return r.Role == types.RoleInstance || r.Role.IsLocalService()
}

// GetServerID extracts the identity from the full name. The username
// extracted from the node's identity (x.509 certificate) is expected to
// consist of "<server-id>.<cluster-name>" so strip the cluster name suffix
// to get the server id.
//
// Note that as of right now Teleport expects server id to be a UUID4 but
// older Gravity clusters used to override it with strings like
// "192_168_1_1.<cluster-name>" so this code can't rely on it being
// UUID4 to account for clusters upgraded from older versions.
func (r BuiltinRole) GetServerID() string {
	return strings.TrimSuffix(r.Identity.Username, "."+r.ClusterName)
}

// GetIdentity returns client identity
func (r BuiltinRole) GetIdentity() tlsca.Identity {
	return r.Identity
}

// RemoteBuiltinRole is the role of the remote (service connecting via trusted cluster link)
// Teleport service.
type RemoteBuiltinRole struct {
	// Role is the builtin role of the user
	Role types.SystemRole

	// Username is for authentication tracking purposes
	Username string

	// ClusterName is the name of the remote cluster.
	ClusterName string

	// Identity is source x509 used to build this role
	Identity tlsca.Identity
}

// GetIdentity returns client identity
func (r RemoteBuiltinRole) GetIdentity() tlsca.Identity {
	return r.Identity
}

// IsRemoteServer returns true if the primary role is either RoleRemoteProxy, or one of
// the local service roles (e.g. proxy) from the remote cluster.
func (r RemoteBuiltinRole) IsRemoteServer() bool {
	return r.Role == types.RoleInstance || r.Role == types.RoleRemoteProxy || r.Role.IsLocalService()
}

// RemoteUser defines encoded remote user.
type RemoteUser struct {
	// Username is a name of the remote user
	Username string `json:"username"`

	// ClusterName is the name of the remote cluster
	// of the user.
	ClusterName string `json:"cluster_name"`

	// RemoteRoles is optional list of remote roles
	RemoteRoles []string `json:"remote_roles"`

	// Principals is a list of Unix logins.
	Principals []string `json:"principals"`

	// KubernetesGroups is a list of Kubernetes groups
	KubernetesGroups []string `json:"kubernetes_groups"`

	// KubernetesUsers is a list of Kubernetes users
	KubernetesUsers []string `json:"kubernetes_users"`

	// DatabaseNames is a list of database names a user can connect to.
	DatabaseNames []string `json:"database_names"`

	// DatabaseUsers is a list of database users a user can connect as.
	DatabaseUsers []string `json:"database_users"`

	// Identity is source x509 used to build this role
	Identity tlsca.Identity
}

// GetIdentity returns client identity
func (r RemoteUser) GetIdentity() tlsca.Identity {
	return r.Identity
}

// ContextWithUserCertificate returns the context with the user certificate embedded.
func ContextWithUserCertificate(ctx context.Context, cert *x509.Certificate) context.Context {
	return context.WithValue(ctx, contextUserCertificate, cert)
}

// UserCertificateFromContext returns the user certificate from the context.
func UserCertificateFromContext(ctx context.Context) (*x509.Certificate, error) {
	cert, ok := ctx.Value(contextUserCertificate).(*x509.Certificate)
	if !ok {
		return nil, trace.BadParameter("expected type *x509.Certificate, got %T", cert)
	}
	return cert, nil
}

// ContextWithClientAddr returns the context with the address embedded.
func ContextWithClientAddr(ctx context.Context, addr net.Addr) context.Context {
	return context.WithValue(ctx, contextClientAddr, addr)
}

// ClientAddrFromContext returns the client address from the context.
func ClientAddrFromContext(ctx context.Context) (net.Addr, error) {
	addr, ok := ctx.Value(contextClientAddr).(net.Addr)
	if !ok {
		return nil, trace.BadParameter("expected type net.Addr, got %T", addr)
	}
	return addr, nil
}

// ContextWithUser returns the context with the user embedded.
func ContextWithUser(ctx context.Context, user IdentityGetter) context.Context {
	return context.WithValue(ctx, contextUser, user)
}

// UserFromContext returns the user from the context.
func UserFromContext(ctx context.Context) (IdentityGetter, error) {
	user, ok := ctx.Value(contextUser).(IdentityGetter)
	if !ok {
		return nil, trace.BadParameter("expected type IdentityGetter, got %T", user)
	}
	return user, nil
}

// ContextWithMFAResponse returns the context with the user MFA response embedded.
func ContextWithMFAResponse(ctx context.Context, resp *authpb.MFAAuthenticateResponse) context.Context {
	return context.WithValue(ctx, contextMFAResponse, resp)
}

// MFAResponseFromContext returns the MFA response from the context.
func MFAResponseFromContext(ctx context.Context) (*authpb.MFAAuthenticateResponse, error) {
	user, ok := ctx.Value(contextMFAResponse).(*authpb.MFAAuthenticateResponse)
	if !ok {
		return nil, trace.BadParameter("expected type MFAAuthenticateResponse, got %T", user)
	}
	return user, nil
}
