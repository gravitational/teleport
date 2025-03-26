/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package authz

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/vulcand/predicate/builder"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
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

// DeviceAuthorizationOpts captures Device Trust options for [AuthorizerOpts].
type DeviceAuthorizationOpts struct {
	// DisableGlobalMode disables the global device_trust.mode toggle.
	// See [types.DeviceTrust.Mode].
	DisableGlobalMode bool

	// DisableRoleMode disables the role-based device trust toggle.
	// See [types.RoleOption.DeviceTrustMode].
	DisableRoleMode bool
}

// AuthorizerOpts holds creation options for [NewAuthorizer].
type AuthorizerOpts struct {
	ClusterName         string
	AccessPoint         AuthorizerAccessPoint
	ReadOnlyAccessPoint ReadOnlyAuthorizerAccessPoint
	MFAAuthenticator    MFAAuthenticator
	LockWatcher         *services.LockWatcher
	Logger              logrus.FieldLogger

	// DeviceAuthorization holds Device Trust authorization options.
	//
	// Allows services that either do explicit device authorization or don't (yet)
	// support device trust to disable it.
	// Most services should not set this field.
	DeviceAuthorization DeviceAuthorizationOpts
	// PermitCaching opts into the authorizer setting up its own internal
	// caching when ReadOnlyAccessPoint is not provided.
	PermitCaching bool
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
		logger = logrus.WithFields(logrus.Fields{teleport.ComponentKey: "authorizer"})
	}

	if opts.ReadOnlyAccessPoint == nil {
		// we create the read-only access point if not provided in order to keep our
		// code paths simpler, but the it will not perform ttl-caching unless opts.PermitCaching
		// was set. This is necessary because the vast majority of our test coverage
		// cannot handle caching, and will fail if caching is enabled.
		var err error
		opts.ReadOnlyAccessPoint, err = readonly.NewCache(readonly.CacheConfig{
			Upstream: accessPointWrapper{opts.AccessPoint},
			Disabled: !opts.PermitCaching,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &authorizer{
		clusterName:             opts.ClusterName,
		accessPoint:             opts.AccessPoint,
		readOnlyAccessPoint:     opts.ReadOnlyAccessPoint,
		mfaAuthenticator:        opts.MFAAuthenticator,
		lockWatcher:             opts.LockWatcher,
		logger:                  logger,
		disableGlobalDeviceMode: opts.DeviceAuthorization.DisableGlobalMode,
		disableRoleDeviceMode:   opts.DeviceAuthorization.DisableRoleMode,
	}, nil
}

type accessPointWrapper struct {
	AuthorizerAccessPoint
}

// GetAccessGraphSettings returns the access graph settings.
func (accessPointWrapper) GetAccessGraphSettings(ctx context.Context) (*clusterconfigpb.AccessGraphSettings, error) {
	return nil, trace.NotImplemented("GetAccessGraphSettings is not implemented")
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

	// GetRole returns role by name.
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetUser returns user by name.
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)

	// GetCertAuthority returns cert authority by id.
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities.
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)
}

// ReadOnlyAuthorizerAccessPoint is an additional optional access point interface that permits
// optimized access-control checks by sharing references to frequently accessed configuration
// objects across goroutines.
type ReadOnlyAuthorizerAccessPoint interface {
	// GetReadOnlyAuthPreference returns the cluster authentication configuration.
	GetReadOnlyAuthPreference(ctx context.Context) (readonly.AuthPreference, error)

	// GetReadOnlyClusterNetworkingConfig returns cluster networking configuration.
	GetReadOnlyClusterNetworkingConfig(ctx context.Context) (readonly.ClusterNetworkingConfig, error)

	// GetReadOnlySessionRecordingConfig returns session recording configuration.
	GetReadOnlySessionRecordingConfig(ctx context.Context) (readonly.SessionRecordingConfig, error)
}

// MFAAuthenticator authenticates MFA responses.
type MFAAuthenticator interface {
	// ValidateMFAAuthResponse validates an MFA challenge response.
	ValidateMFAAuthResponse(ctx context.Context, resp *proto.MFAAuthenticateResponse, user string, requiredExtensions *mfav1.ChallengeExtensions) (*MFAAuthData, error)
}

// MFAAuthData contains a user's MFA authentication data for a validated MFA response.
type MFAAuthData struct {
	// User is the authenticated Teleport User.
	User string
	// Device is the user's MFA device used to authenticate.
	Device *types.MFADevice
	// AllowReuse determines whether the MFA challenge response used to authenticate
	// can be reused. AllowReuse MFAAuthData may be denied for specific actions.
	AllowReuse mfav1.ChallengeAllowReuse
}

// authorizer creates new local authorizer
type authorizer struct {
	clusterName         string
	accessPoint         AuthorizerAccessPoint
	readOnlyAccessPoint ReadOnlyAuthorizerAccessPoint
	mfaAuthenticator    MFAAuthenticator
	lockWatcher         *services.LockWatcher
	logger              logrus.FieldLogger

	disableGlobalDeviceMode bool
	disableRoleDeviceMode   bool
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

	// disableDeviceRoleMode disables role-based device verification.
	// Inherited from the authorizer that creates the context.
	disableDeviceRoleMode bool

	// AdminActionAuthState is the state of admin action authorization for this auth context.
	AdminActionAuthState AdminActionAuthState
}

// AdminActionAuthState is an admin action authorization state.
type AdminActionAuthState int

const (
	// AdminActionAuthUnauthorized admin action is not authorized.
	AdminActionAuthUnauthorized AdminActionAuthState = iota
	// AdminActionAuthNotRequired admin action authorization is not authorized.
	// This state is used for non-user cases, like internal service roles or Machine ID.
	AdminActionAuthNotRequired
	// AdminActionAuthMFAVerified admin action is authorized with MFA verification.
	AdminActionAuthMFAVerified
	// AdminActionAuthMFAVerifiedWithReuse admin action is authorized with MFA verification.
	// The MFA challenged used for verification allows reuse, which may be denied by some
	// admin actions.
	AdminActionAuthMFAVerifiedWithReuse
)

// GetUserMetadata returns information about the authenticated identity
// to be included in audit events.
func (c *Context) GetUserMetadata() apievents.UserMetadata {
	return c.Identity.GetIdentity().GetUserMetadata()
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
func (c *Context) GetAccessState(authPref readonly.AuthPreference) services.AccessState {
	state := c.Checker.GetAccessState(authPref)
	identity := c.Identity.GetIdentity()

	// Builtin services (like proxy_service and kube_service) are not gated
	// on MFA and only need to pass normal RBAC action checks.
	_, isService := c.Identity.(BuiltinRole)
	state.MFAVerified = isService || identity.IsMFAVerified()

	state.EnableDeviceVerification = !c.disableDeviceRoleMode
	state.DeviceVerified = isService || dtauthz.IsTLSDeviceVerified(&identity.DeviceExtensions)

	return state
}

// GetDisconnectCertExpiry calculates the proper value for DisconnectExpiredCert
// based on whether a connection is set to disconnect on cert expiry, and whether
// the cert is a short-lived (<1m) one issued for an MFA verified session. If the session
// doesn't need to be disconnected on cert expiry, it will return a zero [time.Time].
func (c *Context) GetDisconnectCertExpiry(authPref readonly.AuthPreference) time.Time {
	// In the case where both disconnect_expired_cert and require_session_mfa are enabled,
	// the PreviousIdentityExpires value of the certificate will be used, which is the
	// expiry of the certificate used to issue the short-lived MFA verified certificate.
	//
	// See https://github.com/gravitational/teleport/issues/18544

	// If the session doesn't need to be disconnected on cert expiry just return the default value.
	disconnectExpiredCert := authPref.GetDisconnectExpiredCert()
	if c.Checker != nil {
		disconnectExpiredCert = c.Checker.AdjustDisconnectExpiredCert(disconnectExpiredCert)
	}

	if !disconnectExpiredCert {
		return time.Time{}
	}

	identity := c.Identity.GetIdentity()
	if !identity.PreviousIdentityExpires.IsZero() {
		// If this is a short-lived mfa verified cert, return the certificate extension
		// that holds its issuing certificates expiry value.
		return identity.PreviousIdentityExpires
	}

	// Otherwise, return the current certificates expiration
	return identity.Expires
}

// Authorize authorizes user based on identity supplied via context
func (a *authorizer) Authorize(ctx context.Context) (authCtx *Context, err error) {
	defer func() {
		if err != nil {
			err = a.convertAuthorizerError(err)
		}
	}()

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
	authPref, err := a.readOnlyAccessPoint.GetReadOnlyAuthPreference(ctx)
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
	if !a.disableGlobalDeviceMode {
		if err := dtauthz.VerifyTLSUser(authPref.GetDeviceTrust(), authContext.Identity.GetIdentity()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := a.checkAdminActionVerification(ctx, authContext); err != nil {
		return nil, trace.Wrap(err)
	}

	return authContext, nil
}

func (a *authorizer) enforcePrivateKeyPolicy(ctx context.Context, authContext *Context, authPref readonly.AuthPreference) error {
	switch authContext.Identity.(type) {
	case BuiltinRole, RemoteBuiltinRole:
		// built in roles do not need to pass private key policies
		return nil
	}

	// Check that the required private key policy, defined by roles and auth pref,
	// is met by this Identity's tls certificate.
	identityPolicy := authContext.Identity.GetIdentity().PrivateKeyPolicy
	requiredPolicy, err := authContext.Checker.PrivateKeyPolicy(authPref.GetPrivateKeyPolicy())
	if err != nil {
		return trace.Wrap(err)
	}
	if !requiredPolicy.IsSatisfiedBy(identityPolicy) {
		return keys.NewPrivateKeyPolicyError(requiredPolicy)
	}

	return nil
}

func (a *authorizer) fromUser(ctx context.Context, userI interface{}) (*Context, error) {
	switch user := userI.(type) {
	case LocalUser:
		return a.authorizeLocalUser(ctx, user)
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

// checkAdminActionVerification checks if this auth request is verified for admin actions.
func (a *authorizer) checkAdminActionVerification(ctx context.Context, authContext *Context) error {
	required, err := a.isAdminActionAuthorizationRequired(ctx, authContext)
	if err != nil {
		return trace.Wrap(err)
	}

	if !required {
		authContext.AdminActionAuthState = AdminActionAuthNotRequired
		return nil
	}

	if err := a.authorizeAdminAction(ctx, authContext); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *authorizer) isAdminActionAuthorizationRequired(ctx context.Context, authContext *Context) (bool, error) {
	// Provide a way to turn off admin MFA requirements in case expected functionality
	// is disrupted by this requirement, such as for integrations essential to a user
	// which do not yet make use of a machine ID / AdminRole impersonated identity.
	//
	// TODO(Joerger): once we have fully transitioned to requiring machine ID for
	// integrations and ironed out any bugs with admin MFA, this env var should be removed.
	if os.Getenv("TELEPORT_UNSTABLE_DISABLE_MFA_ADMIN_ACTIONS") == "yes" {
		return false, nil
	}

	// Builtin roles do not require MFA to perform admin actions.
	switch authContext.Identity.(type) {
	case BuiltinRole, RemoteBuiltinRole:
		return false, nil
	}

	authpref, err := a.readOnlyAccessPoint.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// Check if this cluster enforces MFA for admin actions.
	if !authpref.IsAdminActionMFAEnforced() {
		return false, nil
	}

	// Skip MFA check if the user is a Bot.
	if user, err := a.accessPoint.GetUser(ctx, authContext.Identity.GetIdentity().Username, false); err == nil && user.IsBot() {
		a.logger.Debugf("Skipping admin action MFA check for bot identity: %v", authContext.Identity.GetIdentity())
		return false, nil
	}

	// Skip MFA if the identity is being impersonated by the Bot or Admin built in role.
	if impersonator := authContext.Identity.GetIdentity().Impersonator; impersonator != "" {
		impersonatorUser, err := a.accessPoint.GetUser(ctx, impersonator, false)
		if err == nil && impersonatorUser.IsBot() {
			a.logger.Debugf("Skipping admin action MFA check for bot-impersonated identity: %v", authContext.Identity.GetIdentity())
			return false, nil
		}

		// If we don't find a user matching the impersonator, it may be the admin role impersonating.
		// Check that the impersonator matches a host service FQDN - <host-id>.<clustername>
		if trace.IsNotFound(err) {
			hostFQDNParts := strings.SplitN(impersonator, ".", 2)
			if hostFQDNParts[1] == a.clusterName {
				if _, err := uuid.Parse(hostFQDNParts[0]); err == nil {
					a.logger.Debugf("Skipping admin action MFA check for admin-impersonated identity: %v", authContext.Identity.GetIdentity())
					return false, nil
				}
			}
		}
	}

	return true, nil
}

func (a *authorizer) authorizeAdminAction(ctx context.Context, authContext *Context) error {
	// MFA is required to be passed through the request context.
	mfaResp, err := mfa.CredentialsFromContext(ctx)
	if err != nil {
		if trace.IsNotFound(err) {
			// missing MFA verification should be a noop.
			return nil
		}
		return trace.Wrap(err)
	}

	if a.mfaAuthenticator == nil {
		return trace.Errorf("failed to validate MFA auth response, authorizer missing mfaAuthenticator field")
	}

	requiredExt := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION}
	mfaData, err := a.mfaAuthenticator.ValidateMFAAuthResponse(ctx, mfaResp, authContext.User.GetName(), requiredExt)
	if err != nil {
		return trace.Wrap(err)
	}

	authContext.AdminActionAuthState = AdminActionAuthMFAVerified
	if mfaData.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES {
		authContext.AdminActionAuthState = AdminActionAuthMFAVerifiedWithReuse
	}

	return nil
}

// convertAuthorizerError will take an authorizer error and convert it into an error easily
// handled by gRPC services.
func (a *authorizer) convertAuthorizerError(err error) error {
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
	case errors.Is(err, ErrIPPinningMissing) || errors.Is(err, ErrIPPinningMismatch) || errors.Is(err, ErrIPPinningNotAllowed):
		a.logger.Warn(err)
		return trace.Wrap(err)
	case trace.IsAccessDenied(err):
		// don't print stack trace, just log the warning
		a.logger.Warn(err)
	case keys.IsPrivateKeyPolicyError(err):
		// private key policy errors should be returned to the client
		// unaltered so that they know to reauthenticate with a valid key.
		return trace.Unwrap(err)
	default:
		a.logger.WithError(err).Warn("Suppressing unknown authz error.")
	}
	return trace.AccessDenied("access denied")
}

// ErrIPPinningMissing is returned when user cert should be pinned but isn't.
var ErrIPPinningMissing = trace.AccessDenied("pinned IP is required for the user, but is not present on identity")

// ErrIPPinningMismatch is returned when user's pinned IP doesn't match observed IP.
var ErrIPPinningMismatch = trace.AccessDenied("pinned IP doesn't match observed client IP")

// ErrIPPinningNotAllowed is returned when user's pinned IP doesn't match observed IP.
var ErrIPPinningNotAllowed = trace.AccessDenied("IP pinning is not allowed for connections behind L4 load balancers with " +
	"PROXY protocol enabled without explicitly setting 'proxy_protocol: on' in the proxy_service and/or auth_service config.")

// CheckIPPinning verifies IP pinning for the identity, using the client IP taken from context.
// Check is considered successful if no error is returned.
func CheckIPPinning(ctx context.Context, identity tlsca.Identity, pinSourceIP bool, log logrus.FieldLogger) error {
	if identity.PinnedIP == "" {
		if pinSourceIP {
			return ErrIPPinningMissing
		}
		return nil
	}

	clientSrcAddr, err := ClientSrcAddrFromContext(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	clientIP, clientPort, err := net.SplitHostPort(clientSrcAddr.String())
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
	// If connection has port 0 it means it was marked by multiplexer's 'detect()' function as affected by unexpected PROXY header.
	// For security reason we don't allow such connection for IP pinning because we can't rely on client IP being correct.
	if clientPort == "0" {
		if log != nil {
			log.WithFields(logrus.Fields{
				"client_ip": clientIP,
				"pinned_ip": identity.PinnedIP,
			}).Debug(ErrIPPinningNotAllowed.Error())
		}
		return ErrIPPinningMismatch
	}

	return nil
}

// authorizeLocalUser returns authz context based on the username
func (a *authorizer) authorizeLocalUser(ctx context.Context, u LocalUser) (*Context, error) {
	return ContextForLocalUser(ctx, u, a.accessPoint, a.clusterName, a.disableRoleDeviceMode)
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

	accessInfo, err := services.AccessInfoFromRemoteTLSIdentity(u.Identity, ca.CombinedMapping())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, a.clusterName, a.accessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The user is prefixed with "remote-" and suffixed with cluster name with
	// the hope that it does not match a real local user.
	user, err := types.NewUser(services.UsernameForRemoteCluster(u.Username, u.ClusterName))
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
		User:                  user,
		Checker:               checker,
		Identity:              WrapIdentity(identity),
		UnmappedIdentity:      u,
		disableDeviceRoleMode: a.disableRoleDeviceMode,
	}, nil
}

// authorizeBuiltinRole authorizes builtin role
func (a *authorizer) authorizeBuiltinRole(ctx context.Context, r BuiltinRole) (*Context, error) {
	recConfig, err := a.readOnlyAccessPoint.GetReadOnlySessionRecordingConfig(ctx)
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
		User:                  user,
		Checker:               checker,
		Identity:              r,
		UnmappedIdentity:      r,
		disableDeviceRoleMode: a.disableRoleDeviceMode,
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
				types.NewRule(types.KindKubeWaitingContainer, services.RW()),
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
				types.NewRule(types.KindClusterMaintenanceConfig, services.RO()),
				types.NewRule(types.KindAutoUpdateConfig, services.RO()),
				types.NewRule(types.KindAutoUpdateVersion, services.RO()),
				types.NewRule(types.KindAutoUpdateAgentRollout, services.RO()),
				types.NewRule(types.KindIntegration, append(services.RO(), types.VerbUse)),
				types.NewRule(types.KindAuditQuery, services.RO()),
				types.NewRule(types.KindSecurityReport, services.RO()),
				types.NewRule(types.KindSecurityReportState, services.RO()),
				types.NewRule(types.KindUserTask, services.RO()),
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
func RoleSetForBuiltinRoles(clusterName string, recConfig readonly.SessionRecordingConfig, roles ...types.SystemRole) (services.RoleSet, error) {
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
func definitionForBuiltinRole(clusterName string, recConfig readonly.SessionRecordingConfig, role types.SystemRole) (types.Role, error) {
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
						types.NewRule(types.KindStaticHostUser, services.RO()),
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
						types.NewRule(types.KindDatabaseObjectImportRule, services.RO()),
						types.NewRule(types.KindDatabaseObject, services.RW()),
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
						types.NewRule(types.KindKubeWaitingContainer, services.RW()),
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
						types.NewRule(types.KindNode, services.RW()),
						types.NewRule(types.KindKubeServer, services.RO()),
						types.NewRule(types.KindKubernetesCluster, services.RW()),
						types.NewRule(types.KindDatabase, services.RW()),
						types.NewRule(types.KindServerInfo, services.RW()),
						types.NewRule(types.KindApp, services.RW()),
						types.NewRule(types.KindDiscoveryConfig, services.RO()),
						types.NewRule(types.KindIntegration, append(services.RO(), types.VerbUse)),
						types.NewRule(types.KindSemaphore, services.RW()),
						types.NewRule(types.KindUserTask, services.RW()),
					},
					// Discovery service should only access kubes/apps/dbs that originated from discovery.
					KubernetesLabels: types.Labels{types.OriginLabel: []string{types.OriginCloud}},
					DatabaseLabels:   types.Labels{types.OriginLabel: []string{types.OriginCloud}},
					AppLabels:        types.Labels{types.OriginLabel: []string{types.OriginDiscoveryKubernetes}},
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
						types.NewRule(types.KindUser, services.RW()),
						types.NewRule(types.KindUserGroup, services.RW()),
						types.NewRule(types.KindOktaImportRule, services.RO()),
						types.NewRule(types.KindOktaAssignment, services.RW()),
						types.NewRule(types.KindProxy, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindLock, services.RW()),
						types.NewRule(types.KindSAML, services.ReadNoSecrets()),
						// Okta can manage access lists and roles it creates.
						{
							Resources: []string{types.KindRole},
							Verbs:     services.RW(),
							Where: builder.Equals(
								builder.Identifier(`resource.metadata.labels["`+types.OriginLabel+`"]`),
								builder.String(types.OriginOkta),
							).String(),
						},
						{
							Resources: []string{types.KindAccessList},
							Verbs:     services.RW(),
							Where: builder.Equals(
								builder.Identifier(`resource.metadata.labels["`+types.OriginLabel+`"]`),
								builder.String(types.OriginOkta),
							).String(),
						},
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

	case types.RoleAccessGraphPlugin:
		// RoleAccessGraphPlugin is a special role that is used by the Access Graph plugins
		// to access the semaphore resource.
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindSemaphore, services.RW()),
					},
				},
			})

	}

	return nil, trace.NotFound("builtin role %q is not recognized", role.String())
}

// ContextForBuiltinRole returns a context with the builtin role information embedded.
func ContextForBuiltinRole(r BuiltinRole, recConfig readonly.SessionRecordingConfig) (*Context, error) {
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
		User:                  user,
		Checker:               checker,
		Identity:              r,
		UnmappedIdentity:      r,
		disableDeviceRoleMode: true,                       // Builtin roles skip device trust.
		AdminActionAuthState:  AdminActionAuthNotRequired, // builtin roles skip mfa for admin actions.
	}, nil
}

// ContextForLocalUser returns a context with the local user info embedded.
func ContextForLocalUser(ctx context.Context, u LocalUser, accessPoint AuthorizerAccessPoint, clusterName string, disableDeviceRoleMode bool) (*Context, error) {
	// User has to be fetched to check if it's a blocked username
	user, err := accessPoint.GetUser(ctx, u.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessInfo, err := services.AccessInfoFromLocalTLSIdentity(u.Identity, accessPoint)
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
		User:                  user,
		Checker:               accessChecker,
		Identity:              u,
		UnmappedIdentity:      u,
		disableDeviceRoleMode: disableDeviceRoleMode,
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

	// contextClientSrcAddr is a client source address set in the context of the request
	contextClientSrcAddr contextKey = "teleport-client-src-addr"

	// contextClientDstAddr is a client destination address set in the context of the request
	contextClientDstAddr contextKey = "teleport-client-dst-addr"

	// contextConn is a connection in the context associated with the request
	contextConn contextKey = "teleport-connection"
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

// CheckAccessToKind will ensure that the user has access to the given verbs for the given kind.
func (c *Context) CheckAccessToKind(kind string, verb string, additionalVerbs ...string) error {
	ruleCtx := &services.Context{
		User: c.User,
	}

	return c.CheckAccessToRule(ruleCtx, kind, verb, additionalVerbs...)
}

// CheckAccessToResource will ensure that the user has access to the given verbs for the given resource.
func (c *Context) CheckAccessToResource(resource types.Resource, verb string, additionalVerbs ...string) error {
	ruleCtx := &services.Context{
		User:     c.User,
		Resource: resource,
	}

	return c.CheckAccessToRule(ruleCtx, resource.GetKind(), verb, additionalVerbs...)
}

// CheckAccessToRule will ensure that the user has access to the given verbs for the given [services.Context] and kind.
// Prefer to use [Context.CheckAccessToKind] or [Context.CheckAccessToResource] for common checks.
func (c *Context) CheckAccessToRule(ruleCtx *services.Context, kind string, verb string, additionalVerbs ...string) error {
	var errs []error
	for _, verb := range append(additionalVerbs, verb) {
		if err := c.Checker.CheckAccessToRule(ruleCtx, defaults.Namespace, kind, verb); err != nil {
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
}

// AuthorizeAdminAction will ensure that the user is authorized to perform admin actions.
func (c *Context) AuthorizeAdminAction() error {
	switch c.AdminActionAuthState {
	case AdminActionAuthMFAVerified, AdminActionAuthNotRequired:
		return nil
	}
	return trace.Wrap(&mfa.ErrAdminActionMFARequired)
}

// AuthorizeAdminActionAllowReusedMFA will ensure that the user is authorized to perform
// admin actions. Additionally, MFA challenges that allow reuse will be accepted.
func (c *Context) AuthorizeAdminActionAllowReusedMFA() error {
	if c.AdminActionAuthState == AdminActionAuthMFAVerifiedWithReuse {
		return nil
	}
	return c.AuthorizeAdminAction()
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
	if ctx == nil {
		return nil, trace.BadParameter("context is nil")
	}
	cert, ok := ctx.Value(contextUserCertificate).(*x509.Certificate)
	if !ok {
		return nil, trace.BadParameter("user certificate was not found in the context")
	}
	return cert, nil
}

// ContextWithClientSrcAddr returns the context with the address embedded.
func ContextWithClientSrcAddr(ctx context.Context, addr net.Addr) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, contextClientSrcAddr, addr)
}

// ClientSrcAddrFromContext returns the client address from the context.
func ClientSrcAddrFromContext(ctx context.Context) (net.Addr, error) {
	if ctx == nil {
		return nil, trace.BadParameter("context is nil")
	}
	addr, ok := ctx.Value(contextClientSrcAddr).(net.Addr)
	if !ok {
		return nil, trace.BadParameter("client source address was not found in the context")
	}
	return addr, nil
}

// ContextWithClientAddrs returns the context with the client source and destination addresses embedded.
func ContextWithClientAddrs(ctx context.Context, src, dst net.Addr) context.Context {
	if ctx == nil {
		return nil
	}
	ctx = context.WithValue(ctx, contextClientSrcAddr, src)
	return context.WithValue(ctx, contextClientDstAddr, dst)
}

// ClientAddrsFromContext returns the client address from the context.
func ClientAddrsFromContext(ctx context.Context) (src net.Addr, dst net.Addr) {
	if ctx == nil {
		return nil, nil
	}
	src, _ = ctx.Value(contextClientSrcAddr).(net.Addr)
	dst, _ = ctx.Value(contextClientDstAddr).(net.Addr)
	return
}

func ContextWithConn(ctx context.Context, conn net.Conn) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, contextConn, conn)
}

func ConnFromContext(ctx context.Context) (net.Conn, error) {
	if ctx == nil {
		return nil, trace.BadParameter("context is nil")
	}
	conn, ok := ctx.Value(contextConn).(net.Conn)
	if !ok {
		return nil, trace.NotFound("connection was not found in the context")
	}
	return conn, nil
}

// ContextWithUser returns the context with the user embedded.
func ContextWithUser(ctx context.Context, user IdentityGetter) context.Context {
	return context.WithValue(ctx, contextUser, user)
}

// UserFromContext returns the user from the context.
func UserFromContext(ctx context.Context) (IdentityGetter, error) {
	if ctx == nil {
		return nil, trace.BadParameter("context is nil")
	}
	user, ok := ctx.Value(contextUser).(IdentityGetter)
	if !ok {
		return nil, trace.BadParameter("user identity was not found in the context")
	}
	return user, nil
}

// HasBuiltinRole checks if the identity is a builtin role with the matching
// name.
func HasBuiltinRole(authContext Context, name string) bool {
	if _, ok := authContext.Identity.(BuiltinRole); !ok {
		return false
	}
	if !authContext.Checker.HasRole(name) {
		return false
	}

	return true
}

// IsLocalUser checks if the identity is a local user.
func IsLocalUser(authContext Context) bool {
	_, ok := authContext.UnmappedIdentity.(LocalUser)
	return ok
}

// IsLocalOrRemoteUser checks if the identity is either a local or remote user.
func IsLocalOrRemoteUser(authContext Context) bool {
	switch authContext.UnmappedIdentity.(type) {
	case LocalUser, RemoteUser:
		return true
	default:
		return false
	}
}

// IsLocalOrRemoteService checks if the identity is either a local or remote service.
func IsLocalOrRemoteService(authContext Context) bool {
	switch authContext.UnmappedIdentity.(type) {
	case BuiltinRole, RemoteBuiltinRole:
		return true
	default:
		return false
	}
}

// IsCurrentUser checks if the identity is a local user matching the given username
func IsCurrentUser(authContext Context, username string) bool {
	return IsLocalUser(authContext) && authContext.User.GetName() == username
}

// IsRemoteUser checks if the identity is a remote user.
func IsRemoteUser(authContext Context) bool {
	_, ok := authContext.UnmappedIdentity.(RemoteUser)
	return ok
}

// ConnectionMetadata returns a ConnectionMetadata suitable for events caused by
// a remote client making a call. If ctx didn't pass through auth middleware or
// did not come from an HTTP request, empty metadata is returned.
func ConnectionMetadata(ctx context.Context) apievents.ConnectionMetadata {
	remoteAddr, err := ClientSrcAddrFromContext(ctx)
	if err != nil {
		return apievents.ConnectionMetadata{}
	}

	return apievents.ConnectionMetadata{
		RemoteAddr: remoteAddr.String(),
	}
}
