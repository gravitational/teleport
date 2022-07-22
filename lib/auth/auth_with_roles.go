/*
Copyright 2015-2021 Gravitational, Inc.

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

package auth

import (
	"context"
	"net/url"
	"time"

	"github.com/coreos/go-semver/semver"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// ServerWithRoles is a wrapper around auth service
// methods that focuses on authorizing every request
type ServerWithRoles struct {
	authServer *Server
	sessions   session.Service
	alog       events.IAuditLog
	// context holds authorization context
	context Context
}

func (a *ServerWithRoles) ValidateIdemeumServiceToken(ctx context.Context, ServiceToken string, TenantUrl string) (types.WebSession, error) {
	return a.authServer.ValidateIdemeumServiceToken(ctx, ServiceToken, TenantUrl)
}

// CloseContext is closed when the auth server shuts down
func (a *ServerWithRoles) CloseContext() context.Context {
	return a.authServer.closeCtx
}

func (a *ServerWithRoles) actionWithContext(ctx *services.Context, namespace, resource string, verbs ...string) error {
	if len(verbs) == 0 {
		return trace.BadParameter("no verbs provided for authorization check on resource %q", resource)
	}
	var errs []error
	for _, verb := range verbs {
		errs = append(errs, a.context.Checker.CheckAccessToRule(ctx, namespace, resource, verb, false))
	}
	// Convert generic aggregate error to AccessDenied.
	if err := trace.NewAggregate(errs...); err != nil {
		return trace.AccessDenied(err.Error())
	}
	return nil
}

type actionConfig struct {
	quiet   bool
	context Context
}

type actionOption func(*actionConfig)

func quietAction(quiet bool) actionOption {
	return func(cfg *actionConfig) {
		cfg.quiet = quiet
	}
}

func (a *ServerWithRoles) withOptions(opts ...actionOption) actionConfig {
	cfg := actionConfig{context: a.context}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (c actionConfig) action(namespace, resource string, verbs ...string) error {
	if len(verbs) == 0 {
		return trace.BadParameter("no verbs provided for authorization check on resource %q", resource)
	}
	var errs []error
	for _, verb := range verbs {
		errs = append(errs, c.context.Checker.CheckAccessToRule(&services.Context{User: c.context.User}, namespace, resource, verb, c.quiet))
	}
	// Convert generic aggregate error to AccessDenied.
	if err := trace.NewAggregate(errs...); err != nil {
		return trace.AccessDenied(err.Error())
	}
	return nil
}

func (a *ServerWithRoles) action(namespace, resource string, verbs ...string) error {
	return a.withOptions().action(namespace, resource, verbs...)
}

// currentUserAction is a special checker that allows certain actions for users
// even if they are not admins, e.g. update their own passwords,
// or generate certificates, otherwise it will require admin privileges
func (a *ServerWithRoles) currentUserAction(username string) error {
	if hasLocalUserRole(a.context) && username == a.context.User.GetName() {
		return nil
	}
	return a.context.Checker.CheckAccessToRule(&services.Context{User: a.context.User},
		apidefaults.Namespace, types.KindUser, types.VerbCreate, true)
}

// authConnectorAction is a special checker that grants access to auth
// connectors. It first checks if you have access to the specific connector.
// If not, it checks if the requester has the meta KindAuthConnector access
// (which grants access to all connectors).
func (a *ServerWithRoles) authConnectorAction(namespace string, resource string, verb string) error {
	if err := a.context.Checker.CheckAccessToRule(&services.Context{User: a.context.User}, namespace, resource, verb, true); err != nil {
		if err := a.context.Checker.CheckAccessToRule(&services.Context{User: a.context.User}, namespace, types.KindAuthConnector, verb, false); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// actionForListWithCondition extracts a restrictive filter condition to be
// added to a list query after a simple resource check fails.
func (a *ServerWithRoles) actionForListWithCondition(namespace, resource, identifier string) (*types.WhereExpr, error) {
	origErr := a.withOptions(quietAction(true)).action(namespace, resource, types.VerbList)
	if origErr == nil || !trace.IsAccessDenied(origErr) {
		return nil, trace.Wrap(origErr)
	}
	cond, err := a.context.Checker.ExtractConditionForIdentifier(&services.Context{User: a.context.User}, namespace, resource, types.VerbList, identifier)
	if trace.IsAccessDenied(err) {
		log.WithError(err).Infof("Access to %v %v in namespace %v denied to %v.", types.VerbList, resource, namespace, a.context.Checker)
		// Return the original AccessDenied to avoid leaking information.
		return nil, trace.Wrap(origErr)
	}
	return cond, trace.Wrap(err)
}

// actionWithExtendedContext performs an additional RBAC check with extended
// rule context after a simple resource check fails.
func (a *ServerWithRoles) actionWithExtendedContext(namespace, kind, verb string, extendContext func(*services.Context) error) error {
	ruleCtx := &services.Context{User: a.context.User}
	origErr := a.context.Checker.CheckAccessToRule(ruleCtx, namespace, kind, verb, true)
	if origErr == nil || !trace.IsAccessDenied(origErr) {
		return trace.Wrap(origErr)
	}
	if err := extendContext(ruleCtx); err != nil {
		log.WithError(err).Warning("Failed to extend context for second RBAC check.")
		// Return the original AccessDenied to avoid leaking information.
		return trace.Wrap(origErr)
	}
	return trace.Wrap(a.context.Checker.CheckAccessToRule(ruleCtx, namespace, kind, verb, false))
}

// actionForKindSession is a special checker that grants access to session
// recordings.  It can allow access to a specific recording based on the
// `where` section of the user's access rule for kind `session`.
func (a *ServerWithRoles) actionForKindSession(namespace, verb string, sid session.ID) error {
	extendContext := func(ctx *services.Context) error {
		sessionEnd, err := a.findSessionEndEvent(namespace, sid)
		ctx.Session = sessionEnd
		return trace.Wrap(err)
	}
	return trace.Wrap(a.actionWithExtendedContext(namespace, types.KindSession, verb, extendContext))
}

// actionForKindSSHSession is a special checker that grants access to active SSH
// sessions.  It can allow access to a specific session based on the `where`
// section of the user's access rule for kind `ssh_session`.
func (a *ServerWithRoles) actionForKindSSHSession(namespace, verb string, sid session.ID) error {
	extendContext := func(ctx *services.Context) error {
		session, err := a.sessions.GetSession(namespace, sid)
		ctx.SSHSession = session
		return trace.Wrap(err)
	}
	return trace.Wrap(a.actionWithExtendedContext(namespace, types.KindSSHSession, verb, extendContext))
}

// serverAction returns an access denied error if the role is not one of the builtin server roles.
func (a *ServerWithRoles) serverAction() error {
	role, ok := a.context.Identity.(BuiltinRole)
	if !ok || !role.IsServer() {
		return trace.AccessDenied("this request can be only executed by a teleport built-in server")
	}
	return nil
}

// hasBuiltinRole checks that the attached identity is a builtin role and
// whether any of the given roles match the role set.
func (a *ServerWithRoles) hasBuiltinRole(roles ...types.SystemRole) bool {
	for _, role := range roles {
		if HasBuiltinRole(a.context, string(role)) {
			return true
		}
	}
	return false
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

// HasRemoteBuiltinRole checks if the identity is a remote builtin role with the
// matching name.
func HasRemoteBuiltinRole(authContext Context, name string) bool {
	if _, ok := authContext.UnmappedIdentity.(RemoteBuiltinRole); !ok {
		return false
	}
	if !authContext.Checker.HasRole(name) {
		return false
	}
	return true
}

// hasRemoteBuiltinRole checks if the identity is a remote builtin role and the
// name matches.
func (a *ServerWithRoles) hasRemoteBuiltinRole(name string) bool {
	return HasRemoteBuiltinRole(a.context, name)
}

// hasRemoteUserRole checks if the identity is a remote user or not.
func hasRemoteUserRole(authContext Context) bool {
	_, ok := authContext.UnmappedIdentity.(RemoteUser)
	return ok
}

// hasLocalUserRole checks if the identity is a local user or not.
func hasLocalUserRole(authContext Context) bool {
	_, ok := authContext.UnmappedIdentity.(LocalUser)
	return ok
}

// CreateSessionTracker creates a tracker resource for an active session.
func (a *ServerWithRoles) CreateSessionTracker(ctx context.Context, tracker types.SessionTracker) (types.SessionTracker, error) {
	if err := a.serverAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	tracker, err := a.authServer.CreateSessionTracker(ctx, tracker)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tracker, nil
}

func (a *ServerWithRoles) filterSessionTracker(ctx context.Context, joinerRoles []types.Role, tracker types.SessionTracker) bool {
	evaluator := NewSessionAccessEvaluator(tracker.GetHostPolicySets(), tracker.GetSessionKind(), tracker.GetHostUser())
	modes := evaluator.CanJoin(SessionAccessContext{Username: a.context.User.GetName(), Roles: joinerRoles})

	if len(modes) == 0 {
		return false
	}

	// Apply RFD 45 RBAC rules to the session if it's SSH.
	// This is a bit of a hack. It converts to the old legacy format
	// which we don't have all data for, luckily the fields we don't have aren't made available
	// to the RBAC filter anyway.
	if tracker.GetKind() == types.KindSSHSession {
		ruleCtx := &services.Context{User: a.context.User}
		ruleCtx.SSHSession = &session.Session{
			ID:             session.ID(tracker.GetSessionID()),
			Namespace:      apidefaults.Namespace,
			Login:          tracker.GetLogin(),
			Created:        tracker.GetCreated(),
			LastActive:     a.authServer.GetClock().Now(),
			ServerID:       tracker.GetAddress(),
			ServerAddr:     tracker.GetAddress(),
			ServerHostname: tracker.GetHostname(),
			ClusterName:    tracker.GetClusterName(),
		}

		for _, participant := range tracker.GetParticipants() {
			// We only need to fill in User here since other fields get discarded anyway.
			ruleCtx.SSHSession.Parties = append(ruleCtx.SSHSession.Parties, session.Party{
				User: participant.User,
			})
		}

		// Skip past it if there's a deny rule in place blocking access.
		if err := a.context.Checker.CheckAccessToRule(ruleCtx, apidefaults.Namespace, types.KindSSHSession, types.VerbList, true /* silent */); err != nil {
			return false
		}
	}

	return true
}

// GetSessionTracker returns the current state of a session tracker for an active session.
func (a *ServerWithRoles) GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	tracker, err := a.authServer.GetSessionTracker(ctx, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.serverAction(); err == nil {
		return tracker, nil
	}

	user := a.context.User
	joinerRoles, err := services.FetchRoles(user.GetRoles(), a.authServer, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ok := a.filterSessionTracker(ctx, joinerRoles, tracker)
	if !ok {
		return nil, trace.NotFound("session %v not found", sessionID)
	}

	return tracker, nil
}

// GetActiveSessionTrackers returns a list of active session trackers.
func (a *ServerWithRoles) GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error) {
	sessions, err := a.authServer.GetActiveSessionTrackers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.serverAction(); err == nil {
		return sessions, nil
	}

	var filteredSessions []types.SessionTracker
	user := a.context.User
	joinerRoles, err := services.FetchRoles(user.GetRoles(), a.authServer, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, sess := range sessions {
		ok := a.filterSessionTracker(ctx, joinerRoles, sess)
		if ok {
			filteredSessions = append(filteredSessions, sess)
		}
	}

	return filteredSessions, nil
}

// RemoveSessionTracker removes a tracker resource for an active session.
func (a *ServerWithRoles) RemoveSessionTracker(ctx context.Context, sessionID string) error {
	if err := a.serverAction(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.RemoveSessionTracker(ctx, sessionID)
}

// UpdateSessionTracker updates a tracker resource for an active session.
func (a *ServerWithRoles) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error {
	if err := a.serverAction(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.UpdateSessionTracker(ctx, req)
}

// AuthenticateWebUser authenticates web user, creates and returns a web session
// in case authentication is successful
func (a *ServerWithRoles) AuthenticateWebUser(req AuthenticateUserRequest) (types.WebSession, error) {
	// authentication request has it's own authentication, however this limits the requests
	// types to proxies to make it harder to break
	if !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.AuthenticateWebUser(req)
}

// AuthenticateSSHUser authenticates SSH console user, creates and  returns a pair of signed TLS and SSH
// short lived certificates as a result
func (a *ServerWithRoles) AuthenticateSSHUser(req AuthenticateSSHRequest) (*SSHLoginResponse, error) {
	// authentication request has it's own authentication, however this limits the requests
	// types to proxies to make it harder to break
	if !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.AuthenticateSSHUser(req)
}

func (a *ServerWithRoles) GetSessions(namespace string) ([]session.Session, error) {
	cond, err := a.actionForListWithCondition(namespace, types.KindSSHSession, services.SSHSessionIdentifier)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions, err := a.sessions.GetSessions(namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cond == nil {
		return sessions, nil
	}

	// Filter sessions according to cond.
	filteredSessions := make([]session.Session, 0, len(sessions))
	ruleCtx := &services.Context{User: a.context.User}
	for _, s := range sessions {
		ruleCtx.SSHSession = &s
		if err := a.context.Checker.CheckAccessToRule(ruleCtx, namespace, types.KindSSHSession, types.VerbList, true /* silent */); err != nil {
			continue
		}
		filteredSessions = append(filteredSessions, s)
	}
	return filteredSessions, nil
}

func (a *ServerWithRoles) GetSession(namespace string, id session.ID) (*session.Session, error) {
	if err := a.actionForKindSSHSession(namespace, types.VerbRead, id); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.sessions.GetSession(namespace, id)
}

func (a *ServerWithRoles) CreateSession(s session.Session) error {
	if err := a.action(s.Namespace, types.KindSSHSession, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.CreateSession(s)
}

func (a *ServerWithRoles) UpdateSession(req session.UpdateRequest) error {
	if err := a.actionForKindSSHSession(req.Namespace, types.VerbUpdate, req.ID); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.UpdateSession(req)
}

// DeleteSession removes an active session from the backend.
func (a *ServerWithRoles) DeleteSession(namespace string, id session.ID) error {
	if err := a.actionForKindSSHSession(namespace, types.VerbDelete, id); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.DeleteSession(namespace, id)
}

// CreateCertAuthority not implemented: can only be called locally.
func (a *ServerWithRoles) CreateCertAuthority(ca types.CertAuthority) error {
	return trace.NotImplemented(notImplementedMessage)
}

// RotateCertAuthority starts or restarts certificate authority rotation process.
func (a *ServerWithRoles) RotateCertAuthority(ctx context.Context, req RotateRequest) error {
	if err := req.CheckAndSetDefaults(a.authServer.clock); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.RotateCertAuthority(ctx, req)
}

// RotateExternalCertAuthority rotates external certificate authority,
// this method is called by a remote trusted cluster and is used to update
// only public keys and certificates of the certificate authority.
func (a *ServerWithRoles) RotateExternalCertAuthority(ctx context.Context, ca types.CertAuthority) error {
	if ca == nil {
		return trace.BadParameter("missing certificate authority")
	}
	sctx := &services.Context{User: a.context.User, Resource: ca}
	if err := a.actionWithContext(sctx, apidefaults.Namespace, types.KindCertAuthority, types.VerbRotate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.RotateExternalCertAuthority(ctx, ca)
}

// UpsertCertAuthority updates existing cert authority or updates the existing one.
func (a *ServerWithRoles) UpsertCertAuthority(ca types.CertAuthority) error {
	if ca == nil {
		return trace.BadParameter("missing certificate authority")
	}
	ctx := &services.Context{User: a.context.User, Resource: ca}
	if err := a.actionWithContext(ctx, apidefaults.Namespace, types.KindCertAuthority, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertCertAuthority(ca)
}

// CompareAndSwapCertAuthority updates existing cert authority if the existing cert authority
// value matches the value stored in the backend.
func (a *ServerWithRoles) CompareAndSwapCertAuthority(new, existing types.CertAuthority) error {
	if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CompareAndSwapCertAuthority(new, existing)
}

func (a *ServerWithRoles) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error) {
	if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbList, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if loadKeys {
		if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetCertAuthorities(ctx, caType, loadKeys, opts...)
}

func (a *ServerWithRoles) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
	if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if loadKeys {
		if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetCertAuthority(ctx, id, loadKeys, opts...)
}

func (a *ServerWithRoles) GetDomainName(ctx context.Context) (string, error) {
	// anyone can read it, no harm in that
	return a.authServer.GetDomainName()
}

// getClusterCACert returns the PEM-encoded TLS certs for the local cluster
// without signing keys. If the cluster has multiple TLS certs, they will all
// be concatenated.
func (a *ServerWithRoles) GetClusterCACert(
	ctx context.Context,
) (*proto.GetClusterCACertResponse, error) {
	// Allow all roles to get the CA certs.
	return a.authServer.GetClusterCACert(ctx)
}

func (a *ServerWithRoles) DeleteCertAuthority(id types.CertAuthID) error {
	if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteCertAuthority(id)
}

// ActivateCertAuthority not implemented: can only be called locally.
func (a *ServerWithRoles) ActivateCertAuthority(id types.CertAuthID) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeactivateCertAuthority not implemented: can only be called locally.
func (a *ServerWithRoles) DeactivateCertAuthority(id types.CertAuthID) error {
	return trace.NotImplemented(notImplementedMessage)
}

// GenerateToken generates multi-purpose authentication token.
func (a *ServerWithRoles) GenerateToken(ctx context.Context, req *proto.GenerateTokenRequest) (string, error) {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.GenerateToken(ctx, req)
}

func (a *ServerWithRoles) RegisterUsingToken(ctx context.Context, req *types.RegisterUsingTokenRequest) (*proto.Certs, error) {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterUsingToken(ctx, req)
}

// RegisterUsingIAMMethod registers the caller using the IAM join method and
// returns signed certs to join the cluster.
//
// See (*Server).RegisterUsingIAMMethod for further documentation.
//
// This wrapper does not do any extra authz checks, as the register method has
// its own authz mechanism.
func (a *ServerWithRoles) RegisterUsingIAMMethod(ctx context.Context, challengeResponse client.RegisterChallengeResponseFunc) (*proto.Certs, error) {
	certs, err := a.authServer.RegisterUsingIAMMethod(ctx, challengeResponse)
	return certs, trace.Wrap(err)
}

// GenerateHostCerts generates new host certificates (signed
// by the host certificate authority) for a node.
func (a *ServerWithRoles) GenerateHostCerts(ctx context.Context, req *proto.HostCertsRequest) (*proto.Certs, error) {
	clusterName, err := a.authServer.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// username is hostID + cluster name, so make sure server requests new keys for itself
	if a.context.User.GetName() != HostFQDN(req.HostID, clusterName) {
		return nil, trace.AccessDenied("username mismatch %q and %q", a.context.User.GetName(), HostFQDN(req.HostID, clusterName))
	}

	if req.Role == types.RoleInstance {
		if err := a.checkAdditionalSystemRoles(ctx, req); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if len(req.SystemRoles) != 0 {
			return nil, trace.AccessDenied("additional system role encoding not supported for certs of type %q", req.Role)
		}
	}

	existingRoles, err := types.NewTeleportRoles(a.context.User.GetRoles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// prohibit privilege escalations through role changes (except the instance cert exception, handled above).
	if !a.hasBuiltinRole(req.Role) && req.Role != types.RoleInstance {
		return nil, trace.AccessDenied("roles do not match: %v and %v", existingRoles, req.Role)
	}
	return a.authServer.GenerateHostCerts(ctx, req)
}

// checkAdditionalSystemRoles verifies additional system roles in host cert request.
func (a *ServerWithRoles) checkAdditionalSystemRoles(ctx context.Context, req *proto.HostCertsRequest) error {
	// ensure requesting cert's primary role is a server role.
	role, ok := a.context.Identity.(BuiltinRole)
	if !ok || !role.IsServer() {
		return trace.AccessDenied("additional system roles can only be claimed by a teleport built-in server")
	}

	// check that additional system roles are theoretically valid (distinct from permissibility, which
	// is checked in the following loop).
	for _, r := range req.SystemRoles {
		if r.Check() != nil {
			return trace.AccessDenied("additional system role %q cannot be applied (not a valid system role)", r)
		}
		if !r.IsLocalService() {
			return trace.AccessDenied("additional system role %q cannot be applied (not a builtin service role)", r)
		}
	}

	// load system role assertions if relevant
	var assertions proto.UnstableSystemRoleAssertionSet
	var err error
	if req.UnstableSystemRoleAssertionID != "" {
		assertions, err = a.authServer.UnstableGetSystemRoleAssertions(ctx, req.HostID, req.UnstableSystemRoleAssertionID)
		if err != nil {
			// include this error in the logs, since it might be indicative of a bug if it occurs outside of the context
			// of a general backend outage.
			log.Warnf("Failed to load system role assertion set %q for instance %q: %v", req.UnstableSystemRoleAssertionID, req.HostID, err)
			return trace.AccessDenied("failed to load system role assertion set with ID %q", req.UnstableSystemRoleAssertionID)
		}
	}

	// check if additional system roles are permissible
Outer:
	for _, requestedRole := range req.SystemRoles {
		if a.hasBuiltinRole(requestedRole) {
			// instance is already known to hold this role
			continue Outer
		}

		for _, assertedRole := range assertions.SystemRoles {
			if requestedRole == assertedRole {
				// instance recently demonstrated that it holds this role
				continue Outer
			}
		}

		return trace.AccessDenied("additional system role %q cannot be applied (not authorized)", requestedRole)
	}

	return nil
}

func (a *ServerWithRoles) UnstableAssertSystemRole(ctx context.Context, req proto.UnstableSystemRoleAssertion) error {
	role, ok := a.context.Identity.(BuiltinRole)
	if !ok || !role.IsServer() {
		return trace.AccessDenied("system role assertions can only be executed by a teleport built-in server")
	}

	if req.ServerID != role.GetServerID() {
		return trace.AccessDenied("system role assertions do not support impersonation (%q -> %q)", role.GetServerID(), req.ServerID)
	}

	if !a.hasBuiltinRole(req.SystemRole) {
		return trace.AccessDenied("cannot assert unheld system role %q", req.SystemRole)
	}

	if !req.SystemRole.IsLocalService() {
		return trace.AccessDenied("cannot assert non-service system role %q", req.SystemRole)
	}

	return a.authServer.UnstableAssertSystemRole(ctx, req)
}

func (a *ServerWithRoles) RegisterInventoryControlStream(ics client.UpstreamInventoryControlStream) error {
	// Ensure that caller is a teleport server
	role, ok := a.context.Identity.(BuiltinRole)
	if !ok || !role.IsServer() {
		return trace.AccessDenied("inventory control streams can only be created by a teleport built-in server")
	}

	// wait for upstream hello
	var upstreamHello proto.UpstreamInventoryHello
	select {
	case msg := <-ics.Recv():
		switch m := msg.(type) {
		case proto.UpstreamInventoryHello:
			upstreamHello = m
		default:
			return trace.BadParameter("expected upstream hello, got: %T", m)
		}
	case <-ics.Done():
		return trace.Wrap(ics.Error())
	case <-a.CloseContext().Done():
		return trace.Errorf("auth server shutdown")
	}

	// verify that server is creating stream on behalf of itself.
	if upstreamHello.ServerID != role.GetServerID() {
		return trace.AccessDenied("control streams do not support impersonation (%q -> %q)", role.GetServerID(), upstreamHello.ServerID)
	}

	// in order to reduce sensitivity to downgrades/misconfigurations, we simply filter out
	// services that are unrecognized or unauthorized, rather than rejecting hellos that claim them.
	var filteredServices []types.SystemRole
	for _, service := range upstreamHello.Services {
		if !a.hasBuiltinRole(service) {
			log.Warnf("Omitting service %q for control stream of instance %q (unknown or unauthorized).", service, role.GetServerID())
			continue
		}
		filteredServices = append(filteredServices, service)
	}

	upstreamHello.Services = filteredServices

	return a.authServer.RegisterInventoryControlStream(ics, upstreamHello)
}

func (a *ServerWithRoles) GetInventoryStatus(ctx context.Context, req proto.InventoryStatusRequest) (proto.InventoryStatusSummary, error) {
	// admin-only for now, but we'll eventually want to develop an RBAC syntax for
	// the inventory APIs once they are more developed.
	if !a.hasBuiltinRole(types.RoleAdmin) {
		return proto.InventoryStatusSummary{}, trace.AccessDenied("requires builtin admin role")
	}
	return a.authServer.GetInventoryStatus(ctx, req), nil
}

func (a *ServerWithRoles) PingInventory(ctx context.Context, req proto.InventoryPingRequest) (proto.InventoryPingResponse, error) {
	// admin-only for now, but we'll eventually want to develop an RBAC syntax for
	// the inventory APIs once they are more developed.
	if !a.hasBuiltinRole(types.RoleAdmin) {
		return proto.InventoryPingResponse{}, trace.AccessDenied("requires builtin admin role")
	}
	return a.authServer.PingInventory(ctx, req)
}

func (a *ServerWithRoles) UpsertNode(ctx context.Context, s types.Server) (*types.KeepAlive, error) {
	if err := a.action(s.GetNamespace(), types.KindNode, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.UpsertNode(ctx, s)
}

// DELETE IN: 5.1.0
//
// This logic has moved to KeepAliveServer.
func (a *ServerWithRoles) KeepAliveNode(ctx context.Context, handle types.KeepAlive) error {
	if !a.hasBuiltinRole(types.RoleNode) {
		return trace.AccessDenied("[10] access denied")
	}
	clusterName, err := a.GetDomainName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	serverName, err := ExtractHostID(a.context.User.GetName(), clusterName)
	if err != nil {
		return trace.AccessDenied("[10] access denied")
	}
	if serverName != handle.Name {
		return trace.AccessDenied("[10] access denied")
	}
	if err := a.action(apidefaults.Namespace, types.KindNode, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.KeepAliveNode(ctx, handle)
}

// KeepAliveServer updates expiry time of a server resource.
func (a *ServerWithRoles) KeepAliveServer(ctx context.Context, handle types.KeepAlive) error {
	clusterName, err := a.GetDomainName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	serverName, err := ExtractHostID(a.context.User.GetName(), clusterName)
	if err != nil {
		return trace.AccessDenied("access denied")
	}

	switch handle.GetType() {
	case constants.KeepAliveNode:
		if serverName != handle.Name {
			return trace.AccessDenied("access denied")
		}
		if !a.hasBuiltinRole(types.RoleNode) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(apidefaults.Namespace, types.KindNode, types.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	case constants.KeepAliveApp:
		if handle.HostID != "" {
			if serverName != handle.HostID {
				return trace.AccessDenied("access denied")
			}
		} else { // DELETE IN 9.0. Legacy app server is heartbeating back.
			if serverName != handle.Name {
				return trace.AccessDenied("access denied")
			}
		}
		if !a.hasBuiltinRole(types.RoleApp) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(apidefaults.Namespace, types.KindAppServer, types.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	case constants.KeepAliveDatabase:
		// There can be multiple database servers per host so they send their
		// host ID in a separate field because unlike SSH nodes the resource
		// name cannot be the host ID.
		if serverName != handle.HostID {
			return trace.AccessDenied("access denied")
		}
		if !a.hasBuiltinRole(types.RoleDatabase) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(apidefaults.Namespace, types.KindDatabaseServer, types.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	case constants.KeepAliveWindowsDesktopService:
		if serverName != handle.Name {
			return trace.AccessDenied("access denied")
		}
		if !a.hasBuiltinRole(types.RoleWindowsDesktop) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(apidefaults.Namespace, types.KindWindowsDesktopService, types.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	case constants.KeepAliveKube:
		if serverName != handle.Name || !a.hasBuiltinRole(types.RoleKube) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(apidefaults.Namespace, types.KindKubeService, types.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unknown keep alive type %q", handle.Type)
	}

	return a.authServer.KeepAliveServer(ctx, handle)
}

// NewWatcher returns a new event watcher
func (a *ServerWithRoles) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	if len(watch.Kinds) == 0 {
		return nil, trace.AccessDenied("can't setup global watch")
	}
	for _, kind := range watch.Kinds {
		// Check the permissions for data of each kind. For watching, most
		// kinds of data just need a Read permission, but some have more
		// complicated logic.
		switch kind.Kind {
		case types.KindCertAuthority:
			verb := types.VerbReadNoSecrets
			if kind.LoadSecrets {
				verb = types.VerbRead
			}
			if err := a.action(apidefaults.Namespace, types.KindCertAuthority, verb); err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindAccessRequest:
			var filter types.AccessRequestFilter
			if err := filter.FromMap(kind.Filter); err != nil {
				return nil, trace.Wrap(err)
			}
			if filter.User == "" || a.currentUserAction(filter.User) != nil {
				if err := a.action(apidefaults.Namespace, types.KindAccessRequest, types.VerbRead); err != nil {
					return nil, trace.Wrap(err)
				}
			}
		case types.KindAppServer:
			if err := a.action(apidefaults.Namespace, types.KindAppServer, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindWebSession:
			var filter types.WebSessionFilter
			if err := filter.FromMap(kind.Filter); err != nil {
				return nil, trace.Wrap(err)
			}
			resource := types.KindWebSession
			// Allow reading Snowflake sessions to DB service.
			if kind.SubKind == types.KindSnowflakeSession {
				resource = types.KindDatabase
			}
			if filter.User == "" || a.currentUserAction(filter.User) != nil {
				if err := a.action(apidefaults.Namespace, resource, types.VerbRead); err != nil {
					return nil, trace.Wrap(err)
				}
			}
		case types.KindWebToken:
			if err := a.action(apidefaults.Namespace, types.KindWebToken, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindRemoteCluster:
			if err := a.action(apidefaults.Namespace, types.KindRemoteCluster, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindDatabaseServer:
			if err := a.action(apidefaults.Namespace, types.KindDatabaseServer, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindKubeService:
			if err := a.action(apidefaults.Namespace, types.KindKubeService, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindWindowsDesktopService:
			if err := a.action(apidefaults.Namespace, types.KindWindowsDesktopService, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		default:
			if err := a.action(apidefaults.Namespace, kind.Kind, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	switch {
	case a.hasBuiltinRole(types.RoleProxy):
		watch.QueueSize = defaults.ProxyQueueSize
	case a.hasBuiltinRole(types.RoleNode):
		watch.QueueSize = defaults.NodeQueueSize
	}
	return a.authServer.NewWatcher(ctx, watch)
}

// filterNodes filters nodes based off the role of the logged in user.
func (a *ServerWithRoles) filterNodes(checker *nodeChecker, nodes []types.Server) ([]types.Server, error) {
	// Loop over all nodes and check if the caller has access.
	var filteredNodes []types.Server
	for _, node := range nodes {
		err := checker.CanAccess(node)
		if err != nil {
			if trace.IsAccessDenied(err) {
				continue
			}

			return nil, trace.Wrap(err)
		}

		filteredNodes = append(filteredNodes, node)
	}

	return filteredNodes, nil
}

// DeleteAllNodes deletes all nodes in a given namespace
func (a *ServerWithRoles) DeleteAllNodes(ctx context.Context, namespace string) error {
	if err := a.action(namespace, types.KindNode, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllNodes(ctx, namespace)
}

// DeleteNode deletes node in the namespace
func (a *ServerWithRoles) DeleteNode(ctx context.Context, namespace, node string) error {
	if err := a.action(namespace, types.KindNode, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteNode(ctx, namespace, node)
}

// GetNode gets a node by name and namespace.
func (a *ServerWithRoles) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	if err := a.action(namespace, types.KindNode, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	node, err := a.authServer.GetCache().GetNode(ctx, namespace, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	checker, err := newNodeChecker(a.context, a.authServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Run node through filter to check if it's for the connected identity.
	if filteredNodes, err := a.filterNodes(checker, []types.Server{node}); err != nil {
		return nil, trace.Wrap(err)
	} else if len(filteredNodes) == 0 {
		return nil, trace.NotFound("not found")
	}

	return node, nil
}

func (a *ServerWithRoles) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	if err := a.action(namespace, types.KindNode, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch full list of nodes in the backend.
	startFetch := time.Now()
	nodes, err := a.authServer.GetNodes(ctx, namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	elapsedFetch := time.Since(startFetch)

	checker, err := newNodeChecker(a.context, a.authServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Filter nodes to return the ones for the connected identity.
	startFilter := time.Now()
	filteredNodes, err := a.filterNodes(checker, nodes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	elapsedFilter := time.Since(startFilter)

	log.WithFields(logrus.Fields{
		"user":           a.context.User.GetName(),
		"elapsed_fetch":  elapsedFetch,
		"elapsed_filter": elapsedFilter,
	}).Debugf(
		"GetServers(%v->%v) in %v.",
		len(nodes), len(filteredNodes), elapsedFetch+elapsedFilter)

	return filteredNodes, nil
}

// ListResources returns a paginated list of resources filtered by user access.
func (a *ServerWithRoles) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	if req.UseSearchAsRoles {
		clusterName, err := a.authServer.GetClusterName()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// For search-based access requests, replace the current roles with all
		// roles the user is allowed to search with.
		if err := a.context.UseSearchAsRoles(services.RoleGetter(a.authServer), clusterName.GetClusterName()); err != nil {
			return nil, trace.Wrap(err)
		}
		a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &apievents.AccessRequestResourceSearch{
			Metadata: apievents.Metadata{
				Type: events.AccessRequestResourceSearch,
				Code: events.AccessRequestResourceSearchCode,
			},
			UserMetadata:        ClientUserMetadata(ctx),
			SearchAsRoles:       a.context.Checker.RoleNames(),
			ResourceType:        req.ResourceType,
			Namespace:           req.Namespace,
			Labels:              req.Labels,
			PredicateExpression: req.PredicateExpression,
			SearchKeywords:      req.SearchKeywords,
		})
	}

	// ListResources request coming through this auth layer gets request filters
	// stripped off and saved to be applied later after items go through rbac checks.
	// The list that gets returned from the backend comes back unfiltered and as
	// we apply request filters, we might make multiple trips to get more subsets to
	// reach our limit, which is fine b/c we can start query with our next key.
	//
	// But since sorting and counting totals requires us to work with entire list upfront,
	// special handling is needed in this layer b/c if we try to mimic the "subset" here,
	// we will be making unnecessary trips and doing needless work of deserializing every
	// item for every subset.
	if req.RequiresFakePagination() {
		resp, err := a.listResourcesWithSort(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return resp, nil
	}

	// Start real pagination.
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	limit := int(req.Limit)
	actionVerbs := []string{types.VerbList, types.VerbRead}
	switch req.ResourceType {
	case types.KindNode:
		// We are checking list only for Nodes to keep backwards compatibility.
		// The read verb got added to GetNodes initially in:
		//   https://github.com/gravitational/teleport/pull/1209
		// but got removed shortly afterwards in:
		//   https://github.com/gravitational/teleport/pull/1224
		actionVerbs = []string{types.VerbList}

	case types.KindDatabaseServer, types.KindAppServer, types.KindKubeService, types.KindWindowsDesktop:

	default:
		return nil, trace.NotImplemented("resource type %s does not support pagination", req.ResourceType)
	}

	if err := a.action(req.Namespace, req.ResourceType, actionVerbs...); err != nil {
		return nil, trace.Wrap(err)
	}

	// Perform the label/search/expr filtering here (instead of at the backend
	// `ListResources`) to ensure that it will be applied only to resources
	// the user has access to.
	filter := services.MatchResourceFilter{
		ResourceKind:        req.ResourceType,
		Labels:              req.Labels,
		SearchKeywords:      req.SearchKeywords,
		PredicateExpression: req.PredicateExpression,
	}
	req.Labels = nil
	req.SearchKeywords = nil
	req.PredicateExpression = ""

	resourceChecker, err := a.newResourceAccessChecker(req.ResourceType)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var resp types.ListResourcesResponse
	if err := a.authServer.IterateResources(ctx, req, func(resource types.ResourceWithLabels) error {
		if len(resp.Resources) == limit {
			resp.NextKey = backend.GetPaginationKey(resource)
			return ErrDone
		}

		if err := resourceChecker.CanAccess(resource); err != nil {
			if trace.IsAccessDenied(err) {
				return nil
			}

			return trace.Wrap(err)
		}

		switch match, err := services.MatchResourceByFilters(resource, filter, nil /* ignore dup matches  */); {
		case err != nil:
			return trace.Wrap(err)
		case match:
			resp.Resources = append(resp.Resources, resource)
			return nil
		}

		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &resp, nil
}

// resourceAccessChecker allows access to be checked differently per resource type.
type resourceAccessChecker interface {
	CanAccess(resource types.Resource) error
}

// resourceChecker is a pass through checker that utilizes the provided
// services.AccessChecker to check access
type resourceChecker struct {
	services.AccessChecker
}

// CanAccess handles providing the proper services.AccessCheckable resource
// to the services.AccessChecker
func (r resourceChecker) CanAccess(resource types.Resource) error {
	// MFA is not required for operations on app resources but
	// will be enforced at the connection time.
	mfaParams := services.AccessMFAParams{Verified: true}
	switch rr := resource.(type) {
	case types.AppServer:
		return r.CheckAccess(rr.GetApp(), mfaParams)
	case types.DatabaseServer:
		return r.CheckAccess(rr.GetDatabase(), mfaParams)
	case types.Database:
		return r.CheckAccess(rr, mfaParams)
	case types.WindowsDesktop:
		return r.CheckAccess(rr, mfaParams)
	default:
		return trace.BadParameter("could not check access to resource type %T", r)
	}
}

// nodeChecker is a resourceAccessChecker that checks for access to nodes
type nodeChecker struct {
	accessChecker services.AccessChecker
	builtinRole   bool
}

// newNodeChecker returns a new nodeChecker that checks access to nodes with the
// the provided user if necessary. This prevents the need to load the role set each time
// a node is checked.
func newNodeChecker(authContext Context, authServer *Server) (*nodeChecker, error) {
	// For certain built-in roles, continue to allow full access and return
	// the full set of nodes to not break existing clusters during migration.
	//
	// In addition, allow proxy (and remote proxy) to access all nodes for its
	// smart resolution address resolution. Once the smart resolution logic is
	// moved to the auth server, this logic can be removed.
	builtinRole := HasBuiltinRole(authContext, string(types.RoleAdmin)) ||
		HasBuiltinRole(authContext, string(types.RoleProxy)) ||
		HasRemoteBuiltinRole(authContext, string(types.RoleRemoteProxy))

	return &nodeChecker{
		accessChecker: authContext.Checker,
		builtinRole:   builtinRole,
	}, nil
}

// CanAccess checks if the user has access to the node
func (n *nodeChecker) CanAccess(resource types.Resource) error {
	server, ok := resource.(types.Server)
	if !ok {
		return trace.BadParameter("unexpected resource type %T", resource)
	}

	if n.builtinRole {
		return nil
	}

	// Check if we can access the node with any of our possible logins.
	for _, login := range n.accessChecker.GetAllLogins() {
		err := n.accessChecker.CheckAccess(server, services.AccessMFAParams{Verified: true}, services.NewLoginMatcher(login))
		if err == nil {
			return nil
		}
	}

	return trace.AccessDenied("access to node %q denied", server.GetHostname())
}

// kubeChecker is a resourceAccessChecker that checks for access to kubernetes services
type kubeChecker struct {
	checker   services.AccessChecker
	localUser bool
}

func newKubeChecker(authContext Context) *kubeChecker {
	return &kubeChecker{
		checker:   authContext.Checker,
		localUser: hasLocalUserRole(authContext),
	}
}

// CanAccess checks if a user has access to kubernetes clusters defined
// in the server. Any clusters which aren't allowed will be removed from the
// resource instead of an error being returned.
func (k *kubeChecker) CanAccess(resource types.Resource) error {
	server, ok := resource.(types.Server)
	if !ok {
		return trace.BadParameter("unexpected resource type %T", resource)
	}

	// Filter out agents that don't have support for moderated sessions access
	// checking if the user has any roles that require it.
	if k.localUser {
		roles := k.checker.Roles()
		agentVersion, versionErr := semver.NewVersion(server.GetTeleportVersion())

		hasK8SRequirePolicy := func() bool {
			for _, role := range roles {
				for _, policy := range role.GetSessionRequirePolicies() {
					if ContainsSessionKind(policy.Kinds, types.KubernetesSessionKind) {
						return true
					}
				}
			}
			return false
		}

		if hasK8SRequirePolicy() && (versionErr != nil || agentVersion.LessThan(*MinSupportedModeratedSessionsVersion)) {
			return trace.AccessDenied("cannot use moderated sessions with pre-v9 kubernetes agents")
		}
	}

	filtered := make([]*types.KubernetesCluster, 0, len(server.GetKubernetesClusters()))
	for _, kube := range server.GetKubernetesClusters() {
		k8sV3, err := types.NewKubernetesClusterV3FromLegacyCluster(server.GetNamespace(), kube)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := k.checker.CheckAccess(k8sV3, services.AccessMFAParams{Verified: true}); err != nil {
			if trace.IsAccessDenied(err) {
				continue
			}

			return trace.Wrap(err)
		}

		filtered = append(filtered, kube)
	}

	server.SetKubernetesClusters(filtered)
	return nil
}

// newResourceAccessChecker creates a resourceAccessChecker for the provided resource type
func (a *ServerWithRoles) newResourceAccessChecker(resource string) (resourceAccessChecker, error) {
	switch resource {
	case types.KindAppServer, types.KindDatabaseServer, types.KindWindowsDesktop:
		return &resourceChecker{AccessChecker: a.context.Checker}, nil
	case types.KindNode:
		return newNodeChecker(a.context, a.authServer)
	case types.KindKubeService:
		return newKubeChecker(a.context), nil
	default:
		return nil, trace.BadParameter("could not check access to resource type %s", resource)
	}
}

// listResourcesWithSort retrieves all resources of a certain resource type with rbac applied
// then afterwards applies request sorting and filtering.
func (a *ServerWithRoles) listResourcesWithSort(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var resources []types.ResourceWithLabels
	switch req.ResourceType {
	case types.KindNode:
		nodes, err := a.GetNodes(ctx, req.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		servers := types.Servers(nodes)
		if err := servers.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = servers.AsResources()

	case types.KindAppServer:
		appservers, err := a.GetApplicationServers(ctx, req.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		servers := types.AppServers(appservers)
		if err := servers.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = servers.AsResources()

	case types.KindDatabaseServer:
		dbservers, err := a.GetDatabaseServers(ctx, req.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		servers := types.DatabaseServers(dbservers)
		if err := servers.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = servers.AsResources()

	case types.KindKubernetesCluster:
		kubeservices, err := a.GetKubeServices(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Extract kube clusters into its own list.
		var clusters []types.KubeCluster
		for _, svc := range kubeservices {
			for _, legacyCluster := range svc.GetKubernetesClusters() {
				cluster, err := types.NewKubernetesClusterV3FromLegacyCluster(svc.GetNamespace(), legacyCluster)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				clusters = append(clusters, cluster)
			}
		}

		sortedClusters := types.KubeClusters(clusters)
		if err := sortedClusters.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = sortedClusters.AsResources()

	case types.KindWindowsDesktop:
		windowsdesktops, err := a.GetWindowsDesktops(ctx, req.GetWindowsDesktopFilter())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		desktops := types.WindowsDesktops(windowsdesktops)
		if err := desktops.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = desktops.AsResources()

	default:
		return nil, trace.NotImplemented("resource type %q is not supported for listResourcesWithSort", req.ResourceType)
	}

	// Apply request filters and get pagination info.
	resp, err := local.FakePaginate(resources, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// ListWindowsDesktops not implemented: can only be called locally.
func (a *ServerWithRoles) ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

func (a *ServerWithRoles) UpsertAuthServer(s types.Server) error {
	if err := a.action(apidefaults.Namespace, types.KindAuthServer, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertAuthServer(s)
}

func (a *ServerWithRoles) GetAuthServers() ([]types.Server, error) {
	if err := a.action(apidefaults.Namespace, types.KindAuthServer, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetAuthServers()
}

// DeleteAllAuthServers deletes all auth servers
func (a *ServerWithRoles) DeleteAllAuthServers() error {
	if err := a.action(apidefaults.Namespace, types.KindAuthServer, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllAuthServers()
}

// DeleteAuthServer deletes auth server by name
func (a *ServerWithRoles) DeleteAuthServer(name string) error {
	if err := a.action(apidefaults.Namespace, types.KindAuthServer, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAuthServer(name)
}

func (a *ServerWithRoles) UpsertProxy(s types.Server) error {
	if err := a.action(apidefaults.Namespace, types.KindProxy, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertProxy(s)
}

func (a *ServerWithRoles) GetProxies() ([]types.Server, error) {
	if err := a.action(apidefaults.Namespace, types.KindProxy, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetProxies()
}

// DeleteAllProxies deletes all proxies
func (a *ServerWithRoles) DeleteAllProxies() error {
	if err := a.action(apidefaults.Namespace, types.KindProxy, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllProxies()
}

// DeleteProxy deletes proxy by name
func (a *ServerWithRoles) DeleteProxy(name string) error {
	if err := a.action(apidefaults.Namespace, types.KindProxy, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteProxy(name)
}

func (a *ServerWithRoles) UpsertReverseTunnel(r types.ReverseTunnel) error {
	if err := a.action(apidefaults.Namespace, types.KindReverseTunnel, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertReverseTunnel(r)
}

func (a *ServerWithRoles) GetReverseTunnel(name string, opts ...services.MarshalOption) (types.ReverseTunnel, error) {
	if err := a.action(apidefaults.Namespace, types.KindReverseTunnel, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnel(name, opts...)
}

func (a *ServerWithRoles) GetReverseTunnels(ctx context.Context, opts ...services.MarshalOption) ([]types.ReverseTunnel, error) {
	if err := a.action(apidefaults.Namespace, types.KindReverseTunnel, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnels(ctx, opts...)
}

func (a *ServerWithRoles) DeleteReverseTunnel(domainName string) error {
	if err := a.action(apidefaults.Namespace, types.KindReverseTunnel, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteReverseTunnel(domainName)
}

func (a *ServerWithRoles) DeleteToken(ctx context.Context, token string) error {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteToken(ctx, token)
}

func (a *ServerWithRoles) GetTokens(ctx context.Context) ([]types.ProvisionToken, error) {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetTokens(ctx)
}

func (a *ServerWithRoles) GetToken(ctx context.Context, token string) (types.ProvisionToken, error) {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetToken(ctx, token)
}

func (a *ServerWithRoles) UpsertToken(ctx context.Context, token types.ProvisionToken) error {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertToken(ctx, token)
}

func (a *ServerWithRoles) CreateToken(ctx context.Context, token types.ProvisionToken) error {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateToken(ctx, token)
}

func (a *ServerWithRoles) UpsertPassword(user string, password []byte) error {
	if err := a.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertPassword(user, password)
}

// ChangePassword updates users password based on the old password.
func (a *ServerWithRoles) ChangePassword(req services.ChangePasswordReq) error {
	if err := a.currentUserAction(req.User); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.ChangePassword(req)
}

func (a *ServerWithRoles) CheckPassword(user string, password []byte, otpToken string) error {
	if err := a.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	_, err := a.authServer.checkPassword(user, password, otpToken)
	return trace.Wrap(err)
}

func (a *ServerWithRoles) PreAuthenticatedSignIn(ctx context.Context, user string) (types.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.PreAuthenticatedSignIn(ctx, user, a.context.Identity.GetIdentity())
}

// CreateWebSession creates a new web session for the specified user
func (a *ServerWithRoles) CreateWebSession(ctx context.Context, user string) (types.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateWebSession(ctx, user)
}

// ExtendWebSession creates a new web session for a user based on a valid previous session.
// Additional roles are appended to initial roles if there is an approved access request.
// The new session expiration time will not exceed the expiration time of the old session.
func (a *ServerWithRoles) ExtendWebSession(ctx context.Context, req WebSessionReq) (types.WebSession, error) {
	if err := a.currentUserAction(req.User); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.ExtendWebSession(ctx, req, a.context.Identity.GetIdentity())
}

// GetWebSessionInfo returns the web session for the given user specified with sid.
// The session is stripped of any authentication details.
// Implements auth.WebUIService
func (a *ServerWithRoles) GetWebSessionInfo(ctx context.Context, user, sessionID string) (types.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetWebSessionInfo(ctx, user, sessionID)
}

// GetWebSession returns the web session specified with req.
// Implements auth.ReadAccessPoint.
func (a *ServerWithRoles) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	return a.WebSessions().Get(ctx, req)
}

// WebSessions returns the web session manager.
// Implements services.WebSessionsGetter.
func (a *ServerWithRoles) WebSessions() types.WebSessionInterface {
	return &webSessionsWithRoles{c: a, ws: a.authServer.WebSessions()}
}

// Get returns the web session specified with req.
func (r *webSessionsWithRoles) Get(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	if err := r.c.currentUserAction(req.User); err != nil {
		if err := r.c.action(apidefaults.Namespace, types.KindWebSession, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return r.ws.Get(ctx, req)
}

// List returns the list of all web sessions.
func (r *webSessionsWithRoles) List(ctx context.Context) ([]types.WebSession, error) {
	if err := r.c.action(apidefaults.Namespace, types.KindWebSession, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := r.c.action(apidefaults.Namespace, types.KindWebSession, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.ws.List(ctx)
}

// Upsert creates a new or updates the existing web session from the specified session.
// TODO(dmitri): this is currently only implemented for local invocations. This needs to be
// moved into a more appropriate API
func (*webSessionsWithRoles) Upsert(ctx context.Context, session types.WebSession) error {
	return trace.NotImplemented(notImplementedMessage)
}

// Delete removes the web session specified with req.
func (r *webSessionsWithRoles) Delete(ctx context.Context, req types.DeleteWebSessionRequest) error {
	if err := r.c.canDeleteWebSession(req.User); err != nil {
		return trace.Wrap(err)
	}
	return r.ws.Delete(ctx, req)
}

// DeleteAll removes all web sessions.
func (r *webSessionsWithRoles) DeleteAll(ctx context.Context) error {
	if err := r.c.action(apidefaults.Namespace, types.KindWebSession, types.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := r.c.action(apidefaults.Namespace, types.KindWebSession, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return r.ws.DeleteAll(ctx)
}

// GetWebToken returns the web token specified with req.
// Implements auth.ReadAccessPoint.
func (a *ServerWithRoles) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	return a.WebTokens().Get(ctx, req)
}

type webSessionsWithRoles struct {
	c  accessChecker
	ws types.WebSessionInterface
}

// WebTokens returns the web token manager.
// Implements services.WebTokensGetter.
func (a *ServerWithRoles) WebTokens() types.WebTokenInterface {
	return &webTokensWithRoles{c: a, t: a.authServer.WebTokens()}
}

// Get returns the web token specified with req.
func (r *webTokensWithRoles) Get(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	if err := r.c.currentUserAction(req.User); err != nil {
		if err := r.c.action(apidefaults.Namespace, types.KindWebToken, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return r.t.Get(ctx, req)
}

// List returns the list of all web tokens.
func (r *webTokensWithRoles) List(ctx context.Context) ([]types.WebToken, error) {
	if err := r.c.action(apidefaults.Namespace, types.KindWebToken, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.t.List(ctx)
}

// Upsert creates a new or updates the existing web token from the specified token.
// TODO(dmitri): this is currently only implemented for local invocations. This needs to be
// moved into a more appropriate API
func (*webTokensWithRoles) Upsert(ctx context.Context, session types.WebToken) error {
	return trace.NotImplemented(notImplementedMessage)
}

// Delete removes the web token specified with req.
func (r *webTokensWithRoles) Delete(ctx context.Context, req types.DeleteWebTokenRequest) error {
	if err := r.c.currentUserAction(req.User); err != nil {
		if err := r.c.action(apidefaults.Namespace, types.KindWebToken, types.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
	}
	return r.t.Delete(ctx, req)
}

// DeleteAll removes all web tokens.
func (r *webTokensWithRoles) DeleteAll(ctx context.Context) error {
	if err := r.c.action(apidefaults.Namespace, types.KindWebToken, types.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := r.c.action(apidefaults.Namespace, types.KindWebToken, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return r.t.DeleteAll(ctx)
}

type webTokensWithRoles struct {
	c accessChecker
	t types.WebTokenInterface
}

type accessChecker interface {
	action(namespace, resource string, verbs ...string) error
	currentUserAction(user string) error
	canDeleteWebSession(username string) error
}

func (a *ServerWithRoles) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	// users can always view their own access requests
	if filter.User != "" && a.currentUserAction(filter.User) == nil {
		return a.authServer.GetAccessRequests(ctx, filter)
	}

	// users with read + list permissions can get all requests
	if a.withOptions(quietAction(true)).action(apidefaults.Namespace, types.KindAccessRequest, types.VerbList) == nil {
		if a.withOptions(quietAction(true)).action(apidefaults.Namespace, types.KindAccessRequest, types.VerbRead) == nil {
			return a.authServer.GetAccessRequests(ctx, filter)
		}
	}

	// user does not have read/list permissions and is not specifically requesting only
	// their own requests.  we therefore subselect the filter results to show only those requests
	// that the user *is* allowed to see (specifically, their own requests + requests that they
	// are allowed to review).

	checker, err := services.NewReviewPermissionChecker(ctx, a.authServer, a.context.User.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// unless the user has allow directives for reviewing, they will never be able to
	// see any requests other than their own.
	if !checker.HasAllowDirectives() {
		if filter.User != "" {
			// filter specifies a user, but it wasn't caught by the preceding exception,
			// so just return nothing.
			return nil, nil
		}
		filter.User = a.context.User.GetName()
		return a.authServer.GetAccessRequests(ctx, filter)
	}

	reqs, err := a.authServer.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// filter in place
	filtered := reqs[:0]
	for _, req := range reqs {
		if req.GetUser() == a.context.User.GetName() {
			filtered = append(filtered, req)
			continue
		}

		ok, err := checker.CanReviewRequest(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if ok {
			filtered = append(filtered, req)
			continue
		}
	}
	return filtered, nil
}

func (a *ServerWithRoles) CreateAccessRequest(ctx context.Context, req types.AccessRequest) error {
	// An exception is made to allow users to create access *pending* requests for themselves.
	if !req.GetState().IsPending() || a.currentUserAction(req.GetUser()) != nil {
		if err := a.action(apidefaults.Namespace, types.KindAccessRequest, types.VerbCreate); err != nil {
			return trace.Wrap(err)
		}
	}
	// Ensure that an access request cannot outlive the identity that creates it.
	if req.GetAccessExpiry().Before(a.authServer.GetClock().Now()) || req.GetAccessExpiry().After(a.context.Identity.GetIdentity().Expires) {
		req.SetAccessExpiry(a.context.Identity.GetIdentity().Expires)
	}
	return a.authServer.CreateAccessRequest(ctx, req)
}

func (a *ServerWithRoles) SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) error {
	if err := a.action(apidefaults.Namespace, types.KindAccessRequest, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetAccessRequestState(ctx, params)
}

func (a *ServerWithRoles) SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error) {
	// review author defaults to username of caller.
	if params.Review.Author == "" {
		params.Review.Author = a.context.User.GetName()
	}

	// review author must match calling user, except in the case of the builtin admin role.  we make this
	// exception in order to allow for convenient testing with local tctl connections.
	if !a.hasBuiltinRole(types.RoleAdmin) {
		if params.Review.Author != a.context.User.GetName() {
			return nil, trace.AccessDenied("user %q cannot submit reviews on behalf of %q", a.context.User.GetName(), params.Review.Author)
		}

		// MaybeCanReviewRequests returns false positives, but it will tell us
		// if the user definitely can't review requests, which saves a lot of work.
		if !a.context.Checker.MaybeCanReviewRequests() {
			return nil, trace.AccessDenied("user %q cannot submit reviews", a.context.User.GetName())
		}
	}

	// note that we haven't actually enforced any access-control other than requiring
	// the author field to match the calling user.  fine-grained permissions are evaluated
	// under optimistic locking at the level of the backend service.  the correctness of the
	// author field is all that need be enforced at this level.

	return a.authServer.SubmitAccessReview(ctx, params)
}

func (a *ServerWithRoles) GetAccessCapabilities(ctx context.Context, req types.AccessCapabilitiesRequest) (*types.AccessCapabilities, error) {
	// default to checking the capabilities of the caller
	if req.User == "" {
		req.User = a.context.User.GetName()
	}

	// all users can check their own capabilities
	if a.currentUserAction(req.User) != nil {
		if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := a.action(apidefaults.Namespace, types.KindRole, types.VerbList, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return a.authServer.GetAccessCapabilities(ctx, req)
}

// GetPluginData loads all plugin data matching the supplied filter.
func (a *ServerWithRoles) GetPluginData(ctx context.Context, filter types.PluginDataFilter) ([]types.PluginData, error) {
	switch filter.Kind {
	case types.KindAccessRequest:
		// for backwards compatibility, we allow list/read against access requests to also grant list/read for
		// access request related plugin data.
		if a.withOptions(quietAction(true)).action(apidefaults.Namespace, types.KindAccessRequest, types.VerbList) != nil {
			if err := a.action(apidefaults.Namespace, types.KindAccessPluginData, types.VerbList); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		if a.withOptions(quietAction(true)).action(apidefaults.Namespace, types.KindAccessRequest, types.VerbRead) != nil {
			if err := a.action(apidefaults.Namespace, types.KindAccessPluginData, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return a.authServer.GetPluginData(ctx, filter)
	default:
		return nil, trace.BadParameter("unsupported resource kind %q", filter.Kind)
	}
}

// UpdatePluginData updates a per-resource PluginData entry.
func (a *ServerWithRoles) UpdatePluginData(ctx context.Context, params types.PluginDataUpdateParams) error {
	switch params.Kind {
	case types.KindAccessRequest:
		// for backwards compatibility, we allow update against access requests to also grant update for
		// access request related plugin data.
		if a.withOptions(quietAction(true)).action(apidefaults.Namespace, types.KindAccessRequest, types.VerbUpdate) != nil {
			if err := a.action(apidefaults.Namespace, types.KindAccessPluginData, types.VerbUpdate); err != nil {
				return trace.Wrap(err)
			}
		}
		return a.authServer.UpdatePluginData(ctx, params)
	default:
		return trace.BadParameter("unsupported resource kind %q", params.Kind)
	}
}

// Ping gets basic info about the auth server.
func (a *ServerWithRoles) Ping(ctx context.Context) (proto.PingResponse, error) {
	// The Ping method does not require special permissions since it only returns
	// basic status information.  This is an intentional design choice.  Alternative
	// methods should be used for relaying any sensitive information.
	cn, err := a.authServer.GetClusterName()
	if err != nil {
		return proto.PingResponse{}, trace.Wrap(err)
	}

	return proto.PingResponse{
		ClusterName:     cn.GetClusterName(),
		ServerVersion:   teleport.Version,
		ServerFeatures:  modules.GetModules().Features().ToProto(),
		ProxyPublicAddr: a.getProxyPublicAddr(),
		IsBoring:        modules.GetModules().IsBoringBinary(),
	}, nil
}

// getProxyPublicAddr gets the server's public proxy address.
func (a *ServerWithRoles) getProxyPublicAddr() string {
	if proxies, err := a.authServer.GetProxies(); err == nil {
		for _, p := range proxies {
			addr := p.GetPublicAddr()
			if addr == "" {
				continue
			}
			if _, err := utils.ParseAddr(addr); err != nil {
				log.Warningf("Invalid public address on the proxy %q: %q: %v.", p.GetName(), addr, err)
				continue
			}
			return addr
		}
	}
	return ""
}

func (a *ServerWithRoles) DeleteAccessRequest(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindAccessRequest, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAccessRequest(ctx, name)
}

func (a *ServerWithRoles) GetUsers(withSecrets bool) ([]types.User, error) {
	if withSecrets {
		// TODO(fspmarshall): replace admin requirement with VerbReadWithSecrets once we've
		// migrated to that model.
		if !a.hasBuiltinRole(types.RoleAdmin) {
			err := trace.AccessDenied("user %q requested access to all users with secrets", a.context.User.GetName())
			log.Warning(err)
			if err := a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &apievents.UserLogin{
				Metadata: apievents.Metadata{
					Type: events.UserLoginEvent,
					Code: events.UserLocalLoginFailureCode,
				},
				Method: events.LoginMethodClientCert,
				Status: apievents.Status{
					Success:     false,
					Error:       trace.Unwrap(err).Error(),
					UserMessage: err.Error(),
				},
			}); err != nil {
				log.WithError(err).Warn("Failed to emit local login failure event.")
			}
			return nil, trace.AccessDenied("this request can be only executed by an admin")
		}
	} else {
		if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbList, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetUsers(withSecrets)
}

func (a *ServerWithRoles) GetUser(name string, withSecrets bool) (types.User, error) {
	if withSecrets {
		// TODO(fspmarshall): replace admin requirement with VerbReadWithSecrets once we've
		// migrated to that model.
		if !a.hasBuiltinRole(types.RoleAdmin) {
			err := trace.AccessDenied("user %q requested access to user %q with secrets", a.context.User.GetName(), name)
			log.Warning(err)
			if err := a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &apievents.UserLogin{
				Metadata: apievents.Metadata{
					Type: events.UserLoginEvent,
					Code: events.UserLocalLoginFailureCode,
				},
				Method: events.LoginMethodClientCert,
				Status: apievents.Status{
					Success:     false,
					Error:       trace.Unwrap(err).Error(),
					UserMessage: err.Error(),
				},
			}); err != nil {
				log.WithError(err).Warn("Failed to emit local login failure event.")
			}
			return nil, trace.AccessDenied("this request can be only executed by an admin")
		}
	} else {
		// if secrets are not being accessed, let users always read
		// their own info.
		if err := a.currentUserAction(name); err != nil {
			// not current user, perform normal permission check.
			if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	return a.authServer.Identity.GetUser(name, withSecrets)
}

// GetCurrentUser returns current user as seen by the server.
// Useful especially in the context of remote clusters which perform role and trait mapping.
func (a *ServerWithRoles) GetCurrentUser(ctx context.Context) (types.User, error) {
	// check access to roles
	for _, role := range a.context.User.GetRoles() {
		_, err := a.GetRole(ctx, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	usrRes := a.context.User.WithoutSecrets()
	if usr, ok := usrRes.(types.User); ok {
		return usr, nil
	}
	return nil, trace.BadParameter("expected types.User when fetching current user information, got %T", usrRes)
}

// GetCurrentUserRoles returns current user's roles.
func (a *ServerWithRoles) GetCurrentUserRoles(ctx context.Context) ([]types.Role, error) {
	roleNames := a.context.User.GetRoles()
	roles := make([]types.Role, 0, len(roleNames))
	for _, roleName := range roleNames {
		role, err := a.GetRole(ctx, roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// DeleteUser deletes an existng user in a backend by username.
func (a *ServerWithRoles) DeleteUser(ctx context.Context, user string) error {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteUser(ctx, user)
}

func (a *ServerWithRoles) GenerateHostCert(
	key []byte, hostID, nodeName string, principals []string, clusterName string, role types.SystemRole, ttl time.Duration,
) ([]byte, error) {
	if err := a.action(apidefaults.Namespace, types.KindHostCert, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateHostCert(key, hostID, nodeName, principals, clusterName, role, ttl)
}

// NewKeepAliver not implemented: can only be called locally.
func (a *ServerWithRoles) NewKeepAliver(ctx context.Context) (types.KeepAliver, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// determineDesiredRolesAndTraits inspects the current request to determine
// which roles and traits the requesting user wants to be present on the
// resulting certificate. This does not attempt to determine if the
// user is allowed to assume the returned roles.
func (a *ServerWithRoles) determineDesiredRolesAndTraits(req proto.UserCertsRequest, user types.User) ([]string, wrappers.Traits, error) {
	if req.Username == a.context.User.GetName() {
		// If UseRoleRequests is set, make sure we don't return unusable
		// certs: an identity without roles can't be parsed.
		// DEPRECATED: consider making role requests without UseRoleRequests
		// set an error in V11.
		if req.UseRoleRequests && len(req.RoleRequests) == 0 {
			return nil, nil, trace.BadParameter("at least one role request is required")
		}

		// Otherwise, if no role requests exist, reuse the roles and traits
		// from the current identity.
		if len(req.RoleRequests) == 0 {
			roles, traits, err := services.ExtractFromIdentity(a.authServer, a.context.Identity.GetIdentity())
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			return roles, traits, nil
		}

		// Do not allow combining role impersonation and access requests
		// Note: additional requested roles will fail validation; it may be
		// possible to support this in the future if needed.
		if len(req.AccessRequests) > 0 {
			log.Warningf("User %v tried to issue a cert with both role and access requests. This is not supported.", a.context.User.GetName())
			return nil, nil, trace.AccessDenied("access denied")
		}

		// If role requests are provided, attempt to satisfy them instead of
		// pulling them directly from the logged in identity. Role requests
		// are intended to reduce allowed permissions so we'll accept them
		// as-is for now (and ensure the user is allowed to assume them
		// later).
		// Note: traits are not currently set for role impersonation.
		return req.RoleRequests, nil, nil
	}

	// Otherwise, use the roles and traits of the impersonated user. Note
	// that permissions have not been verified at this point.

	// Do not allow combining impersonation and access requests
	if len(req.AccessRequests) > 0 {
		log.Warningf("User %v tried to issue a cert for %v and added access requests. This is not supported.", a.context.User.GetName(), req.Username)
		return nil, nil, trace.AccessDenied("access denied")
	}

	return user.GetRoles(), user.GetTraits(), nil
}

// GenerateUserCerts generates users certificates
func (a *ServerWithRoles) GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
	return a.generateUserCerts(ctx, req)
}

func (a *ServerWithRoles) generateUserCerts(ctx context.Context, req proto.UserCertsRequest, opts ...certRequestOption) (*proto.Certs, error) {
	var err error

	// this prevents clients who have no chance at getting a cert and impersonating anyone
	// from enumerating local users and hitting database
	if !a.hasBuiltinRole(types.RoleAdmin) && !a.context.Checker.CanImpersonateSomeone() && req.Username != a.context.User.GetName() {
		return nil, trace.AccessDenied("access denied: impersonation is not allowed")
	}

	if a.context.Identity.GetIdentity().DisallowReissue {
		return nil, trace.AccessDenied("access denied: identity is not allowed to reissue certificates")
	}

	// Prohibit recursive impersonation behavior:
	//
	// Alice can impersonate Bob
	// Bob can impersonate Grace <- this code block prohibits the escape
	//
	// Allow cases:
	//
	// Alice can impersonate Bob
	//
	// Bob (impersonated by Alice) can renew the cert with route to cluster
	//
	// Similarly, for role requests, Alice is allowed to request roles `access`
	// and `ci`, however these impersonated identities, Alice(access) and
	// Alice(ci), should not be able to issue any new certificates.
	//
	if a.context.Identity != nil && a.context.Identity.GetIdentity().Impersonator != "" {
		if len(req.AccessRequests) > 0 {
			return nil, trace.AccessDenied("access denied: impersonated user can not request new roles")
		}
		if req.UseRoleRequests || len(req.RoleRequests) > 0 {
			// Note: technically this should never be needed as all role
			// impersonated certs should have the DisallowReissue set.
			return nil, trace.AccessDenied("access denied: impersonated roles can not request other roles")
		}
		if req.Username != a.context.User.GetName() {
			return nil, trace.AccessDenied("access denied: impersonated user can not impersonate anyone else")
		}
	}

	// Extract the user and role set for whom the certificate will be generated.
	// This should be safe since this is typically done against a local user.
	//
	// This call bypasses RBAC check for users read on purpose.
	// Users who are allowed to impersonate other users might not have
	// permissions to read user data.
	user, err := a.authServer.GetUser(req.Username, false)
	if err != nil {
		log.WithError(err).Debugf("Could not impersonate user %v. The user could not be fetched from local store.", req.Username)
		return nil, trace.AccessDenied("access denied")
	}

	// Do not allow SSO users to be impersonated.
	if req.Username != a.context.User.GetName() && user.GetCreatedBy().Connector != nil {
		log.Warningf("User %v tried to issue a cert for externally managed user %v, this is not supported.", a.context.User.GetName(), req.Username)
		return nil, trace.AccessDenied("access denied")
	}

	// For users renewing certificates limit the TTL to the duration of the session, to prevent
	// users renewing certificates forever.
	if req.Username == a.context.User.GetName() {
		identity := a.context.Identity.GetIdentity()
		sessionExpires := identity.Expires
		if sessionExpires.IsZero() {
			log.Warningf("Encountered identity with no expiry: %v and denied request. Must be internal logic error.", a.context.Identity)
			return nil, trace.AccessDenied("access denied")
		}
		if req.Expires.Before(a.authServer.GetClock().Now()) {
			return nil, trace.AccessDenied("access denied: client credentials have expired, please relogin.")
		}

		// if these credentials are not renewable, we limit the TTL to the duration of the session
		// (this prevents users renewing their certificates forever)
		if req.Expires.After(sessionExpires) {
			if !identity.Renewable {
				req.Expires = sessionExpires
			} else if max := a.authServer.GetClock().Now().Add(defaults.MaxRenewableCertTTL); req.Expires.After(max) {
				req.Expires = max
			}
		}
	}

	if req.Username == a.context.User.GetName() {
		// we're going to extend the roles list based on the access requests, so
		// we ensure that all the current requests are added to the new
		// certificate (and are checked again)
		req.AccessRequests = append(req.AccessRequests, a.context.Identity.GetIdentity().ActiveRequests...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	roles, traits, err := a.determineDesiredRolesAndTraits(req, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var allowedResourceIDs []types.ResourceID
	if len(req.AccessRequests) > 0 {
		// add any applicable access request values.
		req.AccessRequests = apiutils.Deduplicate(req.AccessRequests)
		for _, reqID := range req.AccessRequests {
			accessRequest, err := a.authServer.getValidatedAccessRequest(ctx, req.Username, reqID)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if accessRequest.GetAccessExpiry().Before(req.Expires) {
				// cannot generate a cert that would outlive the access request
				req.Expires = accessRequest.GetAccessExpiry()
			}
			roles = append(roles, accessRequest.GetRoles()...)

			if len(allowedResourceIDs) > 0 && len(accessRequest.GetRequestedResourceIDs()) > 0 {
				return nil, trace.BadParameter("cannot generate certificate with multiple search-based access requests")
			}
			allowedResourceIDs = accessRequest.GetRequestedResourceIDs()
		}
		// nothing prevents an access-request from including roles already possessed by the
		// user, so we must make sure to trim duplicate roles.
		roles = apiutils.Deduplicate(roles)
	}

	parsedRoles, err := services.FetchRoleList(roles, a.authServer, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// add implicit roles to the set and build a checker
	roleSet := services.NewRoleSet(parsedRoles...)
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker := services.NewAccessChecker(&services.AccessInfo{
		Roles:              roleSet.RoleNames(),
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
		RoleSet:            roleSet,
	}, clusterName.GetClusterName())

	switch {
	case a.hasBuiltinRole(types.RoleAdmin):
		// builtin admins can impersonate anyone
		// this is required for local tctl commands to work
	case req.Username == a.context.User.GetName():
		// users can impersonate themselves, but role impersonation requests
		// must be checked.

		if len(req.RoleRequests) > 0 {
			// Note: CheckImpersonateRoles() checks against the _stored_
			// impersonate roles for the user rather than the set available
			// to the current identity. If not explicitly denied (as above),
			// this could allow a role-impersonated certificate to request new
			// certificates with alternate RoleRequests.
			err = a.context.Checker.CheckImpersonateRoles(a.context.User, parsedRoles)
			if err != nil {
				log.Warning(err)
				err := trace.AccessDenied("user %q has requested role impersonation for %q", a.context.User.GetName(), roles)
				if err := a.authServer.emitter.EmitAuditEvent(a.CloseContext(), &apievents.UserLogin{
					Metadata: apievents.Metadata{
						Type: events.UserLoginEvent,
						Code: events.UserLocalLoginFailureCode,
					},
					Method: events.LoginMethodClientCert,
					Status: apievents.Status{
						Success:     false,
						Error:       trace.Unwrap(err).Error(),
						UserMessage: err.Error(),
					},
				}); err != nil {
					log.WithError(err).Warn("Failed to emit local login failure event.")
				}
				return nil, trace.Wrap(err)
			}
		}
	default:
		// check if this user is allowed to impersonate other users
		err = a.context.Checker.CheckImpersonate(a.context.User, user, parsedRoles)
		// adjust session TTL based on the impersonated role set limit
		ttl := req.Expires.Sub(a.authServer.GetClock().Now())
		ttl = checker.AdjustSessionTTL(ttl)
		req.Expires = a.authServer.GetClock().Now().Add(ttl)
		if err != nil {
			log.Warning(err)
			err := trace.AccessDenied("user %q has requested to generate certs for %q.", a.context.User.GetName(), roles)
			if err := a.authServer.emitter.EmitAuditEvent(a.CloseContext(), &apievents.UserLogin{
				Metadata: apievents.Metadata{
					Type: events.UserLoginEvent,
					Code: events.UserLocalLoginFailureCode,
				},
				Method: events.LoginMethodClientCert,
				Status: apievents.Status{
					Success:     false,
					Error:       trace.Unwrap(err).Error(),
					UserMessage: err.Error(),
				},
			}); err != nil {
				log.WithError(err).Warn("Failed to emit local login failure event.")
			}
			return nil, trace.Wrap(err)
		}
	}

	// Generate certificate, note that the roles TTL will be ignored because
	// the request is coming from "tctl auth sign" itself.
	certReq := certRequest{
		user:              user,
		ttl:               req.Expires.Sub(a.authServer.GetClock().Now()),
		compatibility:     req.Format,
		publicKey:         req.PublicKey,
		overrideRoleTTL:   a.hasBuiltinRole(types.RoleAdmin),
		routeToCluster:    req.RouteToCluster,
		kubernetesCluster: req.KubernetesCluster,
		dbService:         req.RouteToDatabase.ServiceName,
		dbProtocol:        req.RouteToDatabase.Protocol,
		dbUser:            req.RouteToDatabase.Username,
		dbName:            req.RouteToDatabase.Database,
		appName:           req.RouteToApp.Name,
		appSessionID:      req.RouteToApp.SessionID,
		appPublicAddr:     req.RouteToApp.PublicAddr,
		appClusterName:    req.RouteToApp.ClusterName,
		awsRoleARN:        req.RouteToApp.AWSRoleARN,
		checker:           checker,
		traits:            traits,
		activeRequests: services.RequestIDs{
			AccessRequests: req.AccessRequests,
		},
	}
	if user.GetName() != a.context.User.GetName() {
		certReq.impersonator = a.context.User.GetName()
	} else if req.UseRoleRequests || len(req.RoleRequests) > 0 {
		// Role impersonation uses the user's own name as the impersonator value.
		certReq.impersonator = a.context.User.GetName()

		// Deny reissuing certs to prevent privilege re-escalation.
		certReq.disallowReissue = true
	} else if a.context.Identity != nil && a.context.Identity.GetIdentity().Impersonator != "" {
		// impersonating users can receive new certs
		certReq.impersonator = a.context.Identity.GetIdentity().Impersonator
	}
	switch req.Usage {
	case proto.UserCertsRequest_Database:
		certReq.usage = []string{teleport.UsageDatabaseOnly}
	case proto.UserCertsRequest_App:
		certReq.usage = []string{teleport.UsageAppsOnly}
	case proto.UserCertsRequest_Kubernetes:
		certReq.usage = []string{teleport.UsageKubeOnly}
	case proto.UserCertsRequest_SSH:
		// SSH certs are ssh-only by definition, certReq.usage only applies to
		// TLS certs.
	case proto.UserCertsRequest_All:
		// Unrestricted usage.
	case proto.UserCertsRequest_WindowsDesktop:
		// Desktop certs.
		certReq.usage = []string{teleport.UsageWindowsDesktopOnly}
	default:
		return nil, trace.BadParameter("unsupported cert usage %q", req.Usage)
	}
	for _, o := range opts {
		o(&certReq)
	}

	// If the user is renewing a renewable cert, make sure the renewable flag
	// remains for subsequent requests of the primary certificate. The
	// renewable flag should never be carried over for impersonation, role
	// requests, or when the disallow-reissue flag has already been set.
	if a.context.Identity.GetIdentity().Renewable &&
		req.Username == a.context.User.GetName() &&
		len(req.RoleRequests) == 0 &&
		!req.UseRoleRequests &&
		!certReq.disallowReissue {
		certReq.renewable = true
	}

	// If the cert is renewable, process any certificate generation counter.
	if certReq.renewable {
		currentIdentityGeneration := a.context.Identity.GetIdentity().Generation
		if err := a.authServer.validateGenerationLabel(ctx, user, &certReq, currentIdentityGeneration); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	certs, err := a.authServer.generateUserCert(certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certs, nil
}

// CreateBot creates a new certificate renewal bot and returns a join token.
func (a *ServerWithRoles) CreateBot(ctx context.Context, req *proto.CreateBotRequest) (*proto.CreateBotResponse, error) {
	// Note: this creates a role with role impersonation privileges for all
	// roles listed in the request and doesn't attempt to verify that the
	// current user has permissions for those embedded roles. We assume that
	// "create role" is effectively root already and validate only that.
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbRead, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(apidefaults.Namespace, types.KindRole, types.VerbRead, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbRead, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.createBot(ctx, req)
}

// DeleteBot removes a certificate renewal bot by name.
func (a *ServerWithRoles) DeleteBot(ctx context.Context, botName string) error {
	// Requires read + delete on users and roles. We do verify the user and
	// role are explicitly associated with a bot before doing anything (must
	// match bot-$name and have a matching teleport.dev/bot label set).
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbRead, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(apidefaults.Namespace, types.KindRole, types.VerbRead, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.deleteBot(ctx, botName)
}

// GetBotUsers fetches all users with bot labels. It does not fetch users with
// secrets.
func (a *ServerWithRoles) GetBotUsers(ctx context.Context) ([]types.User, error) {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.getBotUsers(ctx)
}

func (a *ServerWithRoles) CreateResetPasswordToken(ctx context.Context, req CreateUserTokenRequest) (types.UserToken, error) {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateResetPasswordToken(ctx, req)
}

func (a *ServerWithRoles) GetResetPasswordToken(ctx context.Context, tokenID string) (types.UserToken, error) {
	// tokens are their own authz mechanism, no need to double check
	return a.authServer.getResetPasswordToken(ctx, tokenID)
}

func (a *ServerWithRoles) RotateUserTokenSecrets(ctx context.Context, tokenID string) (types.UserTokenSecrets, error) {
	// tokens are their own authz mechanism, no need to double check
	return a.authServer.RotateUserTokenSecrets(ctx, tokenID)
}

// ChangeUserAuthentication is implemented by AuthService.ChangeUserAuthentication.
func (a *ServerWithRoles) ChangeUserAuthentication(ctx context.Context, req *proto.ChangeUserAuthenticationRequest) (*proto.ChangeUserAuthenticationResponse, error) {
	// Token is it's own authentication, no need to double check.
	return a.authServer.ChangeUserAuthentication(ctx, req)
}

// CreateUser inserts a new user entry in a backend.
func (a *ServerWithRoles) CreateUser(ctx context.Context, user types.User) error {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateUser(ctx, user)
}

// UpdateUser updates an existing user in a backend.
// Captures the auth user who modified the user record.
func (a *ServerWithRoles) UpdateUser(ctx context.Context, user types.User) error {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.UpdateUser(ctx, user)
}

func (a *ServerWithRoles) UpsertUser(u types.User) error {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	createdBy := u.GetCreatedBy()
	if createdBy.IsEmpty() {
		u.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{Name: a.context.User.GetName()},
		})
	}
	return a.authServer.UpsertUser(u)
}

// CompareAndSwapUser updates an existing user in a backend, but fails if the
// backend's value does not match the expected value.
// Captures the auth user who modified the user record.
func (a *ServerWithRoles) CompareAndSwapUser(ctx context.Context, new, existing types.User) error {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.CompareAndSwapUser(ctx, new, existing)
}

// UpsertOIDCConnector creates or updates an OIDC connector.
func (a *ServerWithRoles) UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	if modules.GetModules().Features().OIDC == false {
		return trace.AccessDenied("OIDC is only available in enterprise subscriptions")
	}

	return a.authServer.UpsertOIDCConnector(ctx, connector)
}

func (a *ServerWithRoles) GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetOIDCConnector(ctx, id, withSecrets)
}

func (a *ServerWithRoles) GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetOIDCConnectors(ctx, withSecrets)
}

func (a *ServerWithRoles) CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindOIDCRequest, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	// require additional permissions for executing SSO test flow.
	if req.SSOTestFlow {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbCreate); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	oidcReq, err := a.authServer.CreateOIDCAuthRequest(ctx, req)
	if err != nil {
		emitSSOLoginFailureEvent(a.CloseContext(), a.authServer.emitter, events.LoginMethodOIDC, err, req.SSOTestFlow)
		return nil, trace.Wrap(err)
	}

	return oidcReq, nil
}

// GetOIDCAuthRequest returns OIDC auth request if found.
func (a *ServerWithRoles) GetOIDCAuthRequest(ctx context.Context, id string) (*types.OIDCAuthRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindOIDCRequest, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetOIDCAuthRequest(ctx, id)
}

func (a *ServerWithRoles) ValidateOIDCAuthCallback(ctx context.Context, q url.Values) (*OIDCAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	return a.authServer.ValidateOIDCAuthCallback(ctx, q)
}

func (a *ServerWithRoles) DeleteOIDCConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteOIDCConnector(ctx, connectorID)
}

// UpsertSAMLConnector creates or updates a SAML connector.
func (a *ServerWithRoles) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	if modules.GetModules().Features().SAML == false {
		return trace.AccessDenied("SAML is only available in enterprise subscriptions")
	}
	return a.authServer.UpsertSAMLConnector(ctx, connector)
}

func (a *ServerWithRoles) GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetSAMLConnector(ctx, id, withSecrets)
}

func (a *ServerWithRoles) GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetSAMLConnectors(ctx, withSecrets)
}

func (a *ServerWithRoles) CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindSAMLRequest, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	// require additional permissions for executing SSO test flow.
	if req.SSOTestFlow {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbCreate); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	samlReq, err := a.authServer.CreateSAMLAuthRequest(ctx, req)
	if err != nil {
		emitSSOLoginFailureEvent(a.CloseContext(), a.authServer.emitter, events.LoginMethodSAML, err, req.SSOTestFlow)
		return nil, trace.Wrap(err)
	}

	return samlReq, nil
}

// ValidateSAMLResponse validates SAML auth response.
func (a *ServerWithRoles) ValidateSAMLResponse(ctx context.Context, re string) (*SAMLAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	return a.authServer.ValidateSAMLResponse(ctx, re)
}

// GetSAMLAuthRequest returns SAML auth request if found.
func (a *ServerWithRoles) GetSAMLAuthRequest(ctx context.Context, id string) (*types.SAMLAuthRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindSAMLRequest, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetSAMLAuthRequest(ctx, id)
}

// GetSSODiagnosticInfo returns SSO diagnostic info records.
func (a *ServerWithRoles) GetSSODiagnosticInfo(ctx context.Context, authKind string, authRequestID string) (*types.SSODiagnosticInfo, error) {
	var resource string

	switch authKind {
	case types.KindSAML:
		resource = types.KindSAMLRequest
	case types.KindGithub:
		resource = types.KindGithubRequest
	case types.KindOIDC:
		resource = types.KindOIDCRequest
	default:
		return nil, trace.BadParameter("unsupported authKind %q", authKind)
	}

	if err := a.action(apidefaults.Namespace, resource, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetSSODiagnosticInfo(ctx, authKind, authRequestID)
}

// DeleteSAMLConnector deletes a SAML connector by name.
func (a *ServerWithRoles) DeleteSAMLConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteSAMLConnector(ctx, connectorID)
}

func (a *ServerWithRoles) checkGithubConnector(connector types.GithubConnector) error {
	mapping := connector.GetTeamsToLogins()
	for _, team := range mapping {
		if len(team.KubeUsers) != 0 || len(team.KubeGroups) != 0 {
			return trace.BadParameter("since 6.0 teleport uses teams_to_logins to reference a role, use it instead of local kubernetes_users and kubernetes_groups ")
		}
		for _, localRole := range team.Logins {
			_, err := a.GetRole(context.TODO(), localRole)
			if err != nil {
				if trace.IsNotFound(err) {
					return trace.BadParameter("since 6.0 teleport uses teams_to_logins to reference a role, role %q referenced in mapping for organization %q is not found", localRole, team.Organization)
				}
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// UpsertGithubConnector creates or updates a Github connector.
func (a *ServerWithRoles) UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkGithubConnector(connector); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.upsertGithubConnector(ctx, connector)
}

func (a *ServerWithRoles) GetGithubConnector(ctx context.Context, id string, withSecrets bool) (types.GithubConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetGithubConnector(ctx, id, withSecrets)
}

func (a *ServerWithRoles) GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetGithubConnectors(ctx, withSecrets)
}

// DeleteGithubConnector deletes a Github connector by name.
func (a *ServerWithRoles) DeleteGithubConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.deleteGithubConnector(ctx, connectorID)
}

func (a *ServerWithRoles) CreateGithubAuthRequest(ctx context.Context, req types.GithubAuthRequest) (*types.GithubAuthRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindGithubRequest, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	// require additional permissions for executing SSO test flow.
	if req.SSOTestFlow {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbCreate); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	githubReq, err := a.authServer.CreateGithubAuthRequest(ctx, req)
	if err != nil {
		emitSSOLoginFailureEvent(a.authServer.closeCtx, a.authServer.emitter, events.LoginMethodGithub, err, req.SSOTestFlow)
		return nil, trace.Wrap(err)
	}

	return githubReq, nil
}

// GetGithubAuthRequest returns Github auth request if found.
func (a *ServerWithRoles) GetGithubAuthRequest(ctx context.Context, stateToken string) (*types.GithubAuthRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindGithubRequest, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetGithubAuthRequest(ctx, stateToken)
}

func (a *ServerWithRoles) ValidateGithubAuthCallback(ctx context.Context, q url.Values) (*GithubAuthResponse, error) {
	return a.authServer.ValidateGithubAuthCallback(ctx, q)
}

// EmitAuditEvent emits a single audit event
func (a *ServerWithRoles) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	if err := a.action(apidefaults.Namespace, types.KindEvent, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	role, ok := a.context.Identity.(BuiltinRole)
	if !ok || !role.IsServer() {
		return trace.AccessDenied("this request can be only executed by a teleport built-in server")
	}
	err := events.ValidateServerMetadata(event, role.GetServerID())
	if err != nil {
		// TODO: this should be a proper audit event
		// notifying about access violation
		log.Warningf("Rejecting audit event %v(%q) from %q: %v. The client is attempting to "+
			"submit events for an identity other than the one on its x509 certificate.",
			event.GetType(), event.GetID(), role.GetServerID(), err)
		// this message is sparse on purpose to avoid conveying extra data to an attacker
		return trace.AccessDenied("failed to validate event metadata")
	}
	return a.authServer.emitter.EmitAuditEvent(ctx, event)
}

// CreateAuditStream creates audit event stream
func (a *ServerWithRoles) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	if err := a.action(apidefaults.Namespace, types.KindEvent, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	role, ok := a.context.Identity.(BuiltinRole)
	if !ok || !role.IsServer() {
		return nil, trace.AccessDenied("this request can be only executed by proxy, node or auth")
	}
	stream, err := a.authServer.CreateAuditStream(ctx, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &streamWithRoles{
		stream:   stream,
		a:        a,
		serverID: role.GetServerID(),
	}, nil
}

// ResumeAuditStream resumes the stream that has been created
func (a *ServerWithRoles) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	if err := a.action(apidefaults.Namespace, types.KindEvent, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	role, ok := a.context.Identity.(BuiltinRole)
	if !ok || !role.IsServer() {
		return nil, trace.AccessDenied("this request can be only executed by proxy, node or auth")
	}
	stream, err := a.authServer.ResumeAuditStream(ctx, sid, uploadID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &streamWithRoles{
		stream:   stream,
		a:        a,
		serverID: role.GetServerID(),
	}, nil
}

type streamWithRoles struct {
	a        *ServerWithRoles
	serverID string
	stream   apievents.Stream
}

// Status returns channel receiving updates about stream status
// last event index that was uploaded and upload ID
func (s *streamWithRoles) Status() <-chan apievents.StreamStatus {
	return s.stream.Status()
}

// Done returns channel closed when streamer is closed
// should be used to detect sending errors
func (s *streamWithRoles) Done() <-chan struct{} {
	return s.stream.Done()
}

// Complete closes the stream and marks it finalized
func (s *streamWithRoles) Complete(ctx context.Context) error {
	return s.stream.Complete(ctx)
}

// Close flushes non-uploaded flight stream data without marking
// the stream completed and closes the stream instance
func (s *streamWithRoles) Close(ctx context.Context) error {
	return s.stream.Close(ctx)
}

func (s *streamWithRoles) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	err := events.ValidateServerMetadata(event, s.serverID)
	if err != nil {
		// TODO: this should be a proper audit event
		// notifying about access violation
		log.Warningf("Rejecting audit event %v from %v: %v. A node is attempting to "+
			"submit events for an identity other than the one on its x509 certificate.",
			event.GetID(), s.serverID, err)
		// this message is sparse on purpose to avoid conveying extra data to an attacker
		return trace.AccessDenied("failed to validate event metadata")
	}
	return s.stream.EmitAuditEvent(ctx, event)
}

func (a *ServerWithRoles) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	if err := a.actionForKindSession(namespace, types.VerbRead, sid); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionChunk(namespace, sid, offsetBytes, maxBytes)
}

func (a *ServerWithRoles) GetSessionEvents(namespace string, sid session.ID, afterN int, includePrintEvents bool) ([]events.EventFields, error) {
	if err := a.actionForKindSession(namespace, types.VerbRead, sid); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionEvents(namespace, sid, afterN, includePrintEvents)
}

func (a *ServerWithRoles) findSessionEndEvent(namespace string, sid session.ID) (apievents.AuditEvent, error) {
	sessionEvents, _, err := a.alog.SearchSessionEvents(time.Time{}, a.authServer.clock.Now().UTC(),
		defaults.EventsIterationLimit, types.EventOrderAscending, "",
		&types.WhereExpr{Equals: types.WhereExpr2{
			L: &types.WhereExpr{Field: events.SessionEventID},
			R: &types.WhereExpr{Literal: sid.String()},
		}}, sid.String(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(sessionEvents) == 1 {
		return sessionEvents[0], nil
	}

	return nil, trace.NotFound("session end event not found for session ID %q", sid)
}

// GetNamespaces returns a list of namespaces
func (a *ServerWithRoles) GetNamespaces() ([]types.Namespace, error) {
	if err := a.action(apidefaults.Namespace, types.KindNamespace, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNamespaces()
}

// GetNamespace returns namespace by name
func (a *ServerWithRoles) GetNamespace(name string) (*types.Namespace, error) {
	if err := a.action(apidefaults.Namespace, types.KindNamespace, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNamespace(name)
}

// UpsertNamespace upserts namespace
func (a *ServerWithRoles) UpsertNamespace(ns types.Namespace) error {
	if err := a.action(apidefaults.Namespace, types.KindNamespace, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertNamespace(ns)
}

// DeleteNamespace deletes namespace by name
func (a *ServerWithRoles) DeleteNamespace(name string) error {
	if err := a.action(apidefaults.Namespace, types.KindNamespace, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteNamespace(name)
}

// GetRoles returns a list of roles
func (a *ServerWithRoles) GetRoles(ctx context.Context) ([]types.Role, error) {
	if err := a.action(apidefaults.Namespace, types.KindRole, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetRoles(ctx)
}

// CreateRole not implemented: can only be called locally.
func (a *ServerWithRoles) CreateRole(role types.Role) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpsertRole creates or updates role.
func (a *ServerWithRoles) UpsertRole(ctx context.Context, role types.Role) error {
	if err := a.action(apidefaults.Namespace, types.KindRole, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	// Some options are only available with enterprise subscription
	if err := checkRoleFeatureSupport(role); err != nil {
		return trace.Wrap(err)
	}

	// access predicate syntax is not checked as part of normal role validation in order
	// to allow the available namespaces to be extended without breaking compatibility with
	// older nodes/proxies (which do not need to ever evaluate said predicates).
	if err := services.ValidateAccessPredicates(role); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.UpsertRole(ctx, role)
}

func checkRoleFeatureSupport(role types.Role) error {
	features := modules.GetModules().Features()
	options := role.GetOptions()
	allowReq, allowRev := role.GetAccessRequestConditions(types.Allow), role.GetAccessReviewConditions(types.Allow)

	// source IP pinning doesn't have a dedicated feature flag,
	// it is available to all enterprise users
	if modules.GetModules().BuildType() != modules.BuildEnterprise && role.GetOptions().PinSourceIP {
		return trace.AccessDenied("role option pin_source_ip is only available in enterprise subscriptions")
	}

	switch {
	case !features.AccessControls && options.MaxSessions > 0:
		return trace.AccessDenied(
			"role option max_sessions is only available in enterprise subscriptions")
	case !features.AdvancedAccessWorkflows &&
		(options.RequestAccess == types.RequestStrategyReason || options.RequestAccess == types.RequestStrategyAlways):
		return trace.AccessDenied(
			"role option request_access: %v is only available in enterprise subscriptions", options.RequestAccess)
	case !features.AdvancedAccessWorkflows && len(allowReq.Thresholds) != 0:
		return trace.AccessDenied(
			"role field allow.request.thresholds is only available in enterprise subscriptions")
	case !features.AdvancedAccessWorkflows && !allowRev.IsZero():
		return trace.AccessDenied(
			"role field allow.review_requests is only available in enterprise subscriptions")
	case !features.ResourceAccessRequests && len(allowReq.SearchAsRoles) != 0:
		return trace.AccessDenied(
			"role field allow.search_as_roles is only available in enterprise subscriptions licensed for resource access requests")
	default:
		return nil
	}
}

// GetRole returns role by name
func (a *ServerWithRoles) GetRole(ctx context.Context, name string) (types.Role, error) {
	// Current-user exception: we always allow users to read roles
	// that they hold.  This requirement is checked first to avoid
	// misleading denial messages in the logs.
	if !apiutils.SliceContainsStr(a.context.User.GetRoles(), name) {
		if err := a.action(apidefaults.Namespace, types.KindRole, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetRole(ctx, name)
}

// DeleteRole deletes role by name
func (a *ServerWithRoles) DeleteRole(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindRole, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	// DELETE IN (7.0)
	// It's OK to delete this code alongside migrateOSS code in auth.
	// It prevents 6.0 from migrating resources multiple times
	// and the role is used for `tctl users add` code too.
	if modules.GetModules().BuildType() == modules.BuildOSS && name == teleport.AdminRoleName {
		return trace.AccessDenied("can not delete system role %q", name)
	}
	return a.authServer.DeleteRole(ctx, name)
}

// DeleteClusterName deletes cluster name
func (a *ServerWithRoles) DeleteClusterName() error {
	if err := a.action(apidefaults.Namespace, types.KindClusterName, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteClusterName()
}

// GetClusterName gets the name of the cluster.
func (a *ServerWithRoles) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	if err := a.action(apidefaults.Namespace, types.KindClusterName, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetClusterName()
}

// SetClusterName sets the name of the cluster. SetClusterName can only be called once.
func (a *ServerWithRoles) SetClusterName(c types.ClusterName) error {
	if err := a.action(apidefaults.Namespace, types.KindClusterName, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetClusterName(c)
}

// UpsertClusterName sets the name of the cluster.
func (a *ServerWithRoles) UpsertClusterName(c types.ClusterName) error {
	if err := a.action(apidefaults.Namespace, types.KindClusterName, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertClusterName(c)
}

// DeleteStaticTokens deletes static tokens
func (a *ServerWithRoles) DeleteStaticTokens() error {
	if err := a.action(apidefaults.Namespace, types.KindStaticTokens, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteStaticTokens()
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (a *ServerWithRoles) GetStaticTokens() (types.StaticTokens, error) {
	if err := a.action(apidefaults.Namespace, types.KindStaticTokens, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetStaticTokens()
}

// SetStaticTokens sets the list of static tokens used to provision nodes.
func (a *ServerWithRoles) SetStaticTokens(s types.StaticTokens) error {
	if err := a.action(apidefaults.Namespace, types.KindStaticTokens, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetStaticTokens(s)
}

// GetAuthPreference gets cluster auth preference.
func (a *ServerWithRoles) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	if err := a.action(apidefaults.Namespace, types.KindClusterAuthPreference, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetAuthPreference(ctx)
}

// SetAuthPreference sets cluster auth preference.
func (a *ServerWithRoles) SetAuthPreference(ctx context.Context, newAuthPref types.AuthPreference) error {
	storedAuthPref, err := a.authServer.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.action(apidefaults.Namespace, types.KindClusterAuthPreference, verbsToReplaceResourceWithOrigin(storedAuthPref)...); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.SetAuthPreference(ctx, newAuthPref)
}

// ResetAuthPreference resets cluster auth preference to defaults.
func (a *ServerWithRoles) ResetAuthPreference(ctx context.Context) error {
	storedAuthPref, err := a.authServer.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if storedAuthPref.Origin() == types.OriginConfigFile {
		return trace.BadParameter("config-file configuration cannot be reset")
	}

	if err := a.action(apidefaults.Namespace, types.KindClusterAuthPreference, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.SetAuthPreference(ctx, types.DefaultAuthPreference())
}

// DeleteAuthPreference not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAuthPreference(context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// GetClusterAuditConfig gets cluster audit configuration.
func (a *ServerWithRoles) GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error) {
	if err := a.action(apidefaults.Namespace, types.KindClusterAuditConfig, types.VerbRead); err != nil {
		if err2 := a.action(apidefaults.Namespace, types.KindClusterConfig, types.VerbRead); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetClusterAuditConfig(ctx, opts...)
}

// SetClusterAuditConfig not implemented: can only be called locally.
func (a *ServerWithRoles) SetClusterAuditConfig(ctx context.Context, auditConfig types.ClusterAuditConfig) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteClusterAuditConfig not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteClusterAuditConfig(ctx context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// GetClusterNetworkingConfig gets cluster networking configuration.
func (a *ServerWithRoles) GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	if err := a.action(apidefaults.Namespace, types.KindClusterNetworkingConfig, types.VerbRead); err != nil {
		if err2 := a.action(apidefaults.Namespace, types.KindClusterConfig, types.VerbRead); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetClusterNetworkingConfig(ctx, opts...)
}

// SetClusterNetworkingConfig sets cluster networking configuration.
func (a *ServerWithRoles) SetClusterNetworkingConfig(ctx context.Context, newNetConfig types.ClusterNetworkingConfig) error {
	storedNetConfig, err := a.authServer.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.action(apidefaults.Namespace, types.KindClusterNetworkingConfig, verbsToReplaceResourceWithOrigin(storedNetConfig)...); err != nil {
		if err2 := a.action(apidefaults.Namespace, types.KindClusterConfig, verbsToReplaceResourceWithOrigin(storedNetConfig)...); err2 != nil {
			return trace.Wrap(err)
		}
	}

	tst, err := newNetConfig.GetTunnelStrategyType()
	if err != nil {
		return trace.Wrap(err)
	}
	if tst == types.ProxyPeering &&
		modules.GetModules().BuildType() != modules.BuildEnterprise {
		return trace.AccessDenied("proxy peering is an enterprise-only feature")
	}

	return a.authServer.SetClusterNetworkingConfig(ctx, newNetConfig)
}

// ResetClusterNetworkingConfig resets cluster networking configuration to defaults.
func (a *ServerWithRoles) ResetClusterNetworkingConfig(ctx context.Context) error {
	storedNetConfig, err := a.authServer.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if storedNetConfig.Origin() == types.OriginConfigFile {
		return trace.BadParameter("config-file configuration cannot be reset")
	}

	if err := a.action(apidefaults.Namespace, types.KindClusterNetworkingConfig, types.VerbUpdate); err != nil {
		if err2 := a.action(apidefaults.Namespace, types.KindClusterConfig, types.VerbUpdate); err2 != nil {
			return trace.Wrap(err)
		}
	}

	return a.authServer.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
}

// DeleteClusterNetworkingConfig not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteClusterNetworkingConfig(ctx context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// GetSessionRecordingConfig gets session recording configuration.
func (a *ServerWithRoles) GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error) {
	if err := a.action(apidefaults.Namespace, types.KindSessionRecordingConfig, types.VerbRead); err != nil {
		if err2 := a.action(apidefaults.Namespace, types.KindClusterConfig, types.VerbRead); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetSessionRecordingConfig(ctx, opts...)
}

// SetSessionRecordingConfig sets session recording configuration.
func (a *ServerWithRoles) SetSessionRecordingConfig(ctx context.Context, newRecConfig types.SessionRecordingConfig) error {
	storedRecConfig, err := a.authServer.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.action(apidefaults.Namespace, types.KindSessionRecordingConfig, verbsToReplaceResourceWithOrigin(storedRecConfig)...); err != nil {
		if err2 := a.action(apidefaults.Namespace, types.KindClusterConfig, verbsToReplaceResourceWithOrigin(storedRecConfig)...); err2 != nil {
			return trace.Wrap(err)
		}
	}

	return a.authServer.SetSessionRecordingConfig(ctx, newRecConfig)
}

// ResetSessionRecordingConfig resets session recording configuration to defaults.
func (a *ServerWithRoles) ResetSessionRecordingConfig(ctx context.Context) error {
	storedRecConfig, err := a.authServer.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if storedRecConfig.Origin() == types.OriginConfigFile {
		return trace.BadParameter("config-file configuration cannot be reset")
	}

	if err := a.action(apidefaults.Namespace, types.KindSessionRecordingConfig, types.VerbUpdate); err != nil {
		if err2 := a.action(apidefaults.Namespace, types.KindClusterConfig, types.VerbUpdate); err2 != nil {
			return trace.Wrap(err)
		}
	}

	return a.authServer.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
}

// DeleteSessionRecordingConfig not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteSessionRecordingConfig(ctx context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllTokens not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAllTokens() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllCertAuthorities not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAllCertAuthorities(caType types.CertAuthType) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllCertNamespaces not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAllNamespaces() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllReverseTunnels not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAllReverseTunnels() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllRoles not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAllRoles() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllUsers not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAllUsers() error {
	return trace.NotImplemented(notImplementedMessage)
}

func (a *ServerWithRoles) GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error) {
	if err := a.action(apidefaults.Namespace, types.KindTrustedCluster, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetTrustedClusters(ctx)
}

func (a *ServerWithRoles) GetTrustedCluster(ctx context.Context, name string) (types.TrustedCluster, error) {
	if err := a.action(apidefaults.Namespace, types.KindTrustedCluster, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetTrustedCluster(ctx, name)
}

// UpsertTrustedCluster creates or updates a trusted cluster.
func (a *ServerWithRoles) UpsertTrustedCluster(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error) {
	if err := a.action(apidefaults.Namespace, types.KindTrustedCluster, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.UpsertTrustedCluster(ctx, tc)
}

func (a *ServerWithRoles) ValidateTrustedCluster(ctx context.Context, validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	// the token provides it's own authorization and authentication
	return a.authServer.validateTrustedCluster(ctx, validateRequest)
}

// DeleteTrustedCluster deletes a trusted cluster by name.
func (a *ServerWithRoles) DeleteTrustedCluster(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindTrustedCluster, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteTrustedCluster(ctx, name)
}

func (a *ServerWithRoles) UpsertTunnelConnection(conn types.TunnelConnection) error {
	if err := a.action(apidefaults.Namespace, types.KindTunnelConnection, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertTunnelConnection(conn)
}

func (a *ServerWithRoles) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
	if err := a.action(apidefaults.Namespace, types.KindTunnelConnection, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetTunnelConnections(clusterName, opts...)
}

func (a *ServerWithRoles) GetAllTunnelConnections(opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
	if err := a.action(apidefaults.Namespace, types.KindTunnelConnection, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetAllTunnelConnections(opts...)
}

func (a *ServerWithRoles) DeleteTunnelConnection(clusterName string, connName string) error {
	if err := a.action(apidefaults.Namespace, types.KindTunnelConnection, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteTunnelConnection(clusterName, connName)
}

func (a *ServerWithRoles) DeleteTunnelConnections(clusterName string) error {
	if err := a.action(apidefaults.Namespace, types.KindTunnelConnection, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteTunnelConnections(clusterName)
}

func (a *ServerWithRoles) DeleteAllTunnelConnections() error {
	if err := a.action(apidefaults.Namespace, types.KindTunnelConnection, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllTunnelConnections()
}

func (a *ServerWithRoles) CreateRemoteCluster(conn types.RemoteCluster) error {
	if err := a.action(apidefaults.Namespace, types.KindRemoteCluster, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateRemoteCluster(conn)
}

func (a *ServerWithRoles) UpdateRemoteCluster(ctx context.Context, rc types.RemoteCluster) error {
	if err := a.action(apidefaults.Namespace, types.KindRemoteCluster, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpdateRemoteCluster(ctx, rc)
}

func (a *ServerWithRoles) GetRemoteCluster(clusterName string) (types.RemoteCluster, error) {
	if err := a.action(apidefaults.Namespace, types.KindRemoteCluster, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := a.authServer.GetRemoteCluster(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.context.Checker.CheckAccessToRemoteCluster(cluster); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}
	return cluster, nil
}

func (a *ServerWithRoles) GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error) {
	if err := a.action(apidefaults.Namespace, types.KindRemoteCluster, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	remoteClusters, err := a.authServer.GetRemoteClusters(opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.filterRemoteClustersForUser(remoteClusters)
}

// filterRemoteClustersForUser filters remote clusters based on what the current user is authorized to access
func (a *ServerWithRoles) filterRemoteClustersForUser(remoteClusters []types.RemoteCluster) ([]types.RemoteCluster, error) {
	filteredClusters := make([]types.RemoteCluster, 0, len(remoteClusters))
	for _, rc := range remoteClusters {
		if err := a.context.Checker.CheckAccessToRemoteCluster(rc); err != nil {
			if trace.IsAccessDenied(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		filteredClusters = append(filteredClusters, rc)
	}
	return filteredClusters, nil
}

func (a *ServerWithRoles) DeleteRemoteCluster(clusterName string) error {
	if err := a.action(apidefaults.Namespace, types.KindRemoteCluster, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteRemoteCluster(clusterName)
}

func (a *ServerWithRoles) DeleteAllRemoteClusters() error {
	if err := a.action(apidefaults.Namespace, types.KindRemoteCluster, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllRemoteClusters()
}

// AcquireSemaphore acquires lease with requested resources from semaphore.
func (a *ServerWithRoles) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	if err := a.action(apidefaults.Namespace, types.KindSemaphore, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.AcquireSemaphore(ctx, params)
}

// KeepAliveSemaphoreLease updates semaphore lease.
func (a *ServerWithRoles) KeepAliveSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	if err := a.action(apidefaults.Namespace, types.KindSemaphore, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.KeepAliveSemaphoreLease(ctx, lease)
}

// CancelSemaphoreLease cancels semaphore lease early.
func (a *ServerWithRoles) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	if err := a.action(apidefaults.Namespace, types.KindSemaphore, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CancelSemaphoreLease(ctx, lease)
}

// GetSemaphores returns a list of all semaphores matching the supplied filter.
func (a *ServerWithRoles) GetSemaphores(ctx context.Context, filter types.SemaphoreFilter) ([]types.Semaphore, error) {
	if err := a.action(apidefaults.Namespace, types.KindSemaphore, types.VerbReadNoSecrets, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetSemaphores(ctx, filter)
}

// DeleteSemaphore deletes a semaphore matching the supplied filter.
func (a *ServerWithRoles) DeleteSemaphore(ctx context.Context, filter types.SemaphoreFilter) error {
	if err := a.action(apidefaults.Namespace, types.KindSemaphore, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteSemaphore(ctx, filter)
}

// ProcessKubeCSR processes CSR request against Kubernetes CA, returns
// signed certificate if successful.
func (a *ServerWithRoles) ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error) {
	// limits the requests types to proxies to make it harder to break
	if !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.ProcessKubeCSR(req)
}

// GetDatabaseServers returns all registered database servers.
func (a *ServerWithRoles) GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error) {
	if err := a.action(namespace, types.KindDatabaseServer, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := a.authServer.GetDatabaseServers(ctx, namespace, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Filter out databases the caller doesn't have access to.
	var filtered []types.DatabaseServer
	for _, server := range servers {
		err := a.checkAccessToDatabase(server.GetDatabase())
		if err != nil && !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		} else if err == nil {
			filtered = append(filtered, server)
		}
	}
	return filtered, nil
}

// UpsertDatabaseServer creates or updates a new database proxy server.
func (a *ServerWithRoles) UpsertDatabaseServer(ctx context.Context, server types.DatabaseServer) (*types.KeepAlive, error) {
	if err := a.action(server.GetNamespace(), types.KindDatabaseServer, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.UpsertDatabaseServer(ctx, server)
}

// DeleteDatabaseServer removes the specified database proxy server.
func (a *ServerWithRoles) DeleteDatabaseServer(ctx context.Context, namespace, hostID, name string) error {
	if err := a.action(namespace, types.KindDatabaseServer, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteDatabaseServer(ctx, namespace, hostID, name)
}

// DeleteAllDatabaseServers removes all registered database proxy servers.
func (a *ServerWithRoles) DeleteAllDatabaseServers(ctx context.Context, namespace string) error {
	if err := a.action(namespace, types.KindDatabaseServer, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllDatabaseServers(ctx, namespace)
}

// SignDatabaseCSR generates a client certificate used by proxy when talking
// to a remote database service.
func (a *ServerWithRoles) SignDatabaseCSR(ctx context.Context, req *proto.DatabaseCSRRequest) (*proto.DatabaseCSRResponse, error) {
	// Only proxy is allowed to request this certificate when proxying
	// database client connection to a remote database service.
	if !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can only be executed by a proxy service")
	}
	return a.authServer.SignDatabaseCSR(ctx, req)
}

// GenerateDatabaseCert generates a certificate used by a database service
// to authenticate with the database instance.
//
// This certificate can be requested by:
//
//  - Cluster administrator using "tctl auth sign --format=db" command locally
//    on the auth server to produce a certificate for configuring a self-hosted
//    database.
//  - Remote user using "tctl auth sign --format=db" command with a remote
//    proxy (e.g. Teleport Cloud), as long as they can impersonate system
//    role Db.
//  - Database service when initiating connection to a database instance to
//    produce a client certificate.
func (a *ServerWithRoles) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	// Check if this is a local cluster admin, or a database service, or a
	// user that is allowed to impersonate database service.
	if !a.hasBuiltinRole(types.RoleDatabase, types.RoleAdmin) {
		if err := a.canImpersonateBuiltinRole(types.RoleDatabase); err != nil {
			log.WithError(err).Warnf("User %v tried to generate database certificate but is not allowed to impersonate %q system role.",
				a.context.User.GetName(), types.RoleDatabase)
			return nil, trace.AccessDenied(`access denied. The user must be able to impersonate the builtin role and user "Db" in order to generate database certificates, for more info see https://goteleport.com/docs/database-access/reference/cli/#tctl-auth-sign.`)
		}
	}
	return a.authServer.GenerateDatabaseCert(ctx, req)
}

// GenerateSnowflakeJWT generates JWT in the Snowflake required format.
func (a *ServerWithRoles) GenerateSnowflakeJWT(ctx context.Context, req *proto.SnowflakeJWTRequest) (*proto.SnowflakeJWTResponse, error) {
	// Check if this is a local cluster admin, or a database service, or a
	// user that is allowed to impersonate database service.
	if !a.hasBuiltinRole(types.RoleDatabase, types.RoleAdmin) {
		if err := a.canImpersonateBuiltinRole(types.RoleDatabase); err != nil {
			log.WithError(err).Warnf("User %v tried to generate database certificate but is not allowed to impersonate %q system role.",
				a.context.User.GetName(), types.RoleDatabase)
			return nil, trace.AccessDenied(`access denied. The user must be able to impersonate the builtin role and user "Db" in order to generate database certificates, for more info see https://goteleport.com/docs/database-access/reference/cli/#tctl-auth-sign.`)
		}
	}
	return a.authServer.GenerateSnowflakeJWT(ctx, req)
}

// canImpersonateBuiltinRole checks if the current user can impersonate the
// provided system role.
func (a *ServerWithRoles) canImpersonateBuiltinRole(role types.SystemRole) error {
	roleCtx, err := NewBuiltinRoleContext(role)
	if err != nil {
		return trace.Wrap(err)
	}
	roleSet := services.RoleSet(roleCtx.Checker.Roles())
	err = a.context.Checker.CheckImpersonate(a.context.User, roleCtx.User, roleSet.WithoutImplicit())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a *ServerWithRoles) checkAccessToApp(app types.Application) error {
	return a.context.Checker.CheckAccess(
		app,
		// MFA is not required for operations on app resources but
		// will be enforced at the connection time.
		services.AccessMFAParams{Verified: true})
}

// GetApplicationServers returns all registered application servers.
func (a *ServerWithRoles) GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	if err := a.action(namespace, types.KindAppServer, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := a.authServer.GetApplicationServers(ctx, namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Filter out apps the caller doesn't have access to.
	var filtered []types.AppServer
	for _, server := range servers {
		err := a.checkAccessToApp(server.GetApp())
		if err != nil && !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		} else if err == nil {
			filtered = append(filtered, server)
		}
	}
	return filtered, nil
}

// UpsertApplicationServer registers an application server.
func (a *ServerWithRoles) UpsertApplicationServer(ctx context.Context, server types.AppServer) (*types.KeepAlive, error) {
	if err := a.action(server.GetNamespace(), types.KindAppServer, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.UpsertApplicationServer(ctx, server)
}

// DeleteApplicationServer deletes specified application server.
func (a *ServerWithRoles) DeleteApplicationServer(ctx context.Context, namespace, hostID, name string) error {
	if err := a.action(namespace, types.KindAppServer, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteApplicationServer(ctx, namespace, hostID, name)
}

// DeleteAllApplicationServers deletes all registered application servers.
func (a *ServerWithRoles) DeleteAllApplicationServers(ctx context.Context, namespace string) error {
	if err := a.action(namespace, types.KindAppServer, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllApplicationServers(ctx, namespace)
}

// GetAppServers gets all application servers.
//
// DELETE IN 9.0. Deprecated, use GetApplicationServers.
func (a *ServerWithRoles) GetAppServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	if err := a.action(namespace, types.KindAppServer, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := a.authServer.GetAppServers(ctx, namespace, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Loop over all servers, filter out applications on each server and only
	// return the applications the caller has access to.
	//
	// MFA is not required to list the apps, but will be required to connect to
	// them.
	mfaParams := services.AccessMFAParams{Verified: true}
	for _, server := range servers {
		filteredApps := make([]*types.App, 0, len(server.GetApps()))
		for _, app := range server.GetApps() {
			appV3, err := types.NewAppV3FromLegacyApp(app)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			err = a.context.Checker.CheckAccess(appV3, mfaParams)
			if err != nil {
				if trace.IsAccessDenied(err) {
					continue
				}
				return nil, trace.Wrap(err)
			}
			filteredApps = append(filteredApps, app)
		}
		server.SetApps(filteredApps)
	}

	return servers, nil
}

// UpsertAppServer adds an application server.
//
// DELETE IN 9.0. Deprecated, use UpsertApplicationServer.
func (a *ServerWithRoles) UpsertAppServer(ctx context.Context, server types.Server) (*types.KeepAlive, error) {
	if err := a.action(server.GetNamespace(), types.KindAppServer, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.UpsertAppServer(ctx, server)
}

// DeleteAppServer removes an application server.
//
// DELETE IN 9.0. Deprecated, use DeleteApplicationServer.
func (a *ServerWithRoles) DeleteAppServer(ctx context.Context, namespace string, name string) error {
	if err := a.action(namespace, types.KindAppServer, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.DeleteAppServer(ctx, namespace, name); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllAppServers removes all application servers.
//
// DELETE IN 9.0. Deprecated, use DeleteAllApplicationServers.
func (a *ServerWithRoles) DeleteAllAppServers(ctx context.Context, namespace string) error {
	if err := a.action(namespace, types.KindAppServer, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.DeleteAllAppServers(ctx, namespace); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetAppSession gets an application web session.
func (a *ServerWithRoles) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	session, err := a.authServer.GetAppSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Users can only fetch their own app sessions.
	if err := a.currentUserAction(session.GetUser()); err != nil {
		if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return session, nil
}

// GetSnowflakeSession gets a Snowflake web session.
func (a *ServerWithRoles) GetSnowflakeSession(ctx context.Context, req types.GetSnowflakeSessionRequest) (types.WebSession, error) {
	session, err := a.authServer.GetSnowflakeSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if session.GetSubKind() != types.KindSnowflakeSession {
		return nil, trace.AccessDenied("GetSnowflakeSession only allows reading sessions with SubKind Snowflake")
	}
	// Users can only fetch their own app sessions.
	if err := a.currentUserAction(session.GetUser()); err != nil {
		if err := a.action(apidefaults.Namespace, types.KindDatabase, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return session, nil
}

// GetAppSessions gets all application web sessions.
func (a *ServerWithRoles) GetAppSessions(ctx context.Context) ([]types.WebSession, error) {
	if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	sessions, err := a.authServer.GetAppSessions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sessions, nil
}

// GetSnowflakeSessions gets all Snowflake web sessions.
func (a *ServerWithRoles) GetSnowflakeSessions(ctx context.Context) ([]types.WebSession, error) {
	if err := a.action(apidefaults.Namespace, types.KindDatabase, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	sessions, err := a.authServer.GetSnowflakeSessions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sessions, nil
}

// CreateAppSession creates an application web session. Application web
// sessions represent a browser session the client holds.
func (a *ServerWithRoles) CreateAppSession(ctx context.Context, req types.CreateAppSessionRequest) (types.WebSession, error) {
	if err := a.currentUserAction(req.Username); err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := a.authServer.CreateAppSession(ctx, req, a.context.User, a.context.Identity.GetIdentity(), a.context.Checker)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return session, nil
}

// CreateSnowflakeSession creates a Snowflake web session.
func (a *ServerWithRoles) CreateSnowflakeSession(ctx context.Context, req types.CreateSnowflakeSessionRequest) (types.WebSession, error) {
	if err := a.action(apidefaults.Namespace, types.KindDatabase, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	snowflakeSession, err := a.authServer.CreateSnowflakeSession(ctx, req, a.context.Identity.GetIdentity(), a.context.Checker)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return snowflakeSession, nil
}

// UpsertAppSession not implemented: can only be called locally.
func (a *ServerWithRoles) UpsertAppSession(ctx context.Context, session types.WebSession) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpsertSnowflakeSession not implemented: can only be called locally.
func (a *ServerWithRoles) UpsertSnowflakeSession(_ context.Context, _ types.WebSession) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAppSession removes an application web session.
func (a *ServerWithRoles) DeleteAppSession(ctx context.Context, req types.DeleteAppSessionRequest) error {
	session, err := a.authServer.GetAppSession(ctx, types.GetAppSessionRequest(req))
	if err != nil {
		return trace.Wrap(err)
	}
	// Check if user can delete this web session.
	if err := a.canDeleteWebSession(session.GetUser()); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authServer.DeleteAppSession(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteSnowflakeSession removes a Snowflake web session.
func (a *ServerWithRoles) DeleteSnowflakeSession(ctx context.Context, req types.DeleteSnowflakeSessionRequest) error {
	snowflakeSession, err := a.authServer.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest(req))
	if err != nil {
		return trace.Wrap(err)
	}
	// Check if user can delete this web session.
	if err := a.canDeleteWebSession(snowflakeSession.GetUser()); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authServer.DeleteSnowflakeSession(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllSnowflakeSessions removes all Snowflake web sessions.
func (a *ServerWithRoles) DeleteAllSnowflakeSessions(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindDatabase, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.DeleteAllSnowflakeSessions(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllAppSessions removes all application web sessions.
func (a *ServerWithRoles) DeleteAllAppSessions(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.DeleteAllAppSessions(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteUserAppSessions deletes all user’s application sessions.
func (a *ServerWithRoles) DeleteUserAppSessions(ctx context.Context, req *proto.DeleteUserAppSessionsRequest) error {
	// First, check if the current user can delete the request user sessions.
	if err := a.canDeleteWebSession(req.Username); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.DeleteUserAppSessions(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// canDeleteWebSession checks if the current user can delete
// WebSessions from the provided `username`.
func (a *ServerWithRoles) canDeleteWebSession(username string) error {
	if err := a.currentUserAction(username); err != nil {
		if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbList, types.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// GenerateAppToken creates a JWT token with application access.
func (a *ServerWithRoles) GenerateAppToken(ctx context.Context, req types.GenerateAppTokenRequest) (string, error) {
	if err := a.action(apidefaults.Namespace, types.KindJWT, types.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}

	session, err := a.authServer.generateAppToken(ctx, req.Username, req.Roles, req.URI, req.Expires)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return session, nil
}

func (a *ServerWithRoles) Close() error {
	return a.authServer.Close()
}

// UpsertKubeService creates or updates a Server representing a teleport
// kubernetes service.
func (a *ServerWithRoles) UpsertKubeService(ctx context.Context, s types.Server) error {
	if err := a.action(apidefaults.Namespace, types.KindKubeService, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	ap, err := a.authServer.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	_, isService := a.context.Identity.(BuiltinRole)
	isMFAVerified := a.context.Identity.GetIdentity().MFAVerified != ""
	mfaParams := services.AccessMFAParams{
		// MFA requirement only applies to users.
		//
		// Builtin services (like proxy_service and kube_service) are not gated
		// on MFA and only need to pass the RBAC action check above.
		Verified:       isService || isMFAVerified,
		AlwaysRequired: ap.GetRequireSessionMFA(),
	}

	for _, kube := range s.GetKubernetesClusters() {
		k8sV3, err := types.NewKubernetesClusterV3FromLegacyCluster(s.GetNamespace(), kube)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := a.context.Checker.CheckAccess(k8sV3, mfaParams); err != nil {
			return utils.OpaqueAccessDenied(err)
		}
	}
	return a.authServer.UpsertKubeService(ctx, s)
}

// UpsertKubeServiceV2 creates or updates a Server representing a teleport
// kubernetes service.
func (a *ServerWithRoles) UpsertKubeServiceV2(ctx context.Context, s types.Server) (*types.KeepAlive, error) {
	if err := a.action(apidefaults.Namespace, types.KindKubeService, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.UpsertKubeServiceV2(ctx, s)
}

// GetKubeServices returns all Servers representing teleport kubernetes
// services.
func (a *ServerWithRoles) GetKubeServices(ctx context.Context) ([]types.Server, error) {
	if err := a.action(apidefaults.Namespace, types.KindKubeService, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := a.authServer.GetKubeServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	checker := newKubeChecker(a.context)

	for _, server := range servers {
		err = checker.CanAccess(server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return servers, nil
}

// DeleteKubeService deletes a named kubernetes service.
func (a *ServerWithRoles) DeleteKubeService(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindKubeService, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteKubeService(ctx, name)
}

// DeleteAllKubeService deletes all registered kubernetes services.
func (a *ServerWithRoles) DeleteAllKubeServices(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindKubeService, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllKubeServices(ctx)
}

// GetNetworkRestrictions retrieves all the network restrictions (allow/deny lists).
func (a *ServerWithRoles) GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error) {
	if err := a.action(apidefaults.Namespace, types.KindNetworkRestrictions, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNetworkRestrictions(ctx)
}

// SetNetworkRestrictions updates the network restrictions.
func (a *ServerWithRoles) SetNetworkRestrictions(ctx context.Context, nr types.NetworkRestrictions) error {
	if err := a.action(apidefaults.Namespace, types.KindNetworkRestrictions, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetNetworkRestrictions(ctx, nr)
}

// DeleteNetworkRestrictions deletes the network restrictions.
func (a *ServerWithRoles) DeleteNetworkRestrictions(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindNetworkRestrictions, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteNetworkRestrictions(ctx)
}

// GetMFADevices returns a list of MFA devices.
func (a *ServerWithRoles) GetMFADevices(ctx context.Context, req *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error) {
	return a.authServer.GetMFADevices(ctx, req)
}

// TODO(awly): decouple auth.ClientI from auth.ServerWithRoles, they exist on
// opposite sides of the connection.

// AddMFADevice exists to satisfy auth.ClientI but is not implemented here.
// Use auth.GRPCServer.AddMFADevice or client.Client.AddMFADevice instead.
func (a *ServerWithRoles) AddMFADevice(ctx context.Context) (proto.AuthService_AddMFADeviceClient, error) {
	return nil, trace.NotImplemented("bug: AddMFADevice must not be called on auth.ServerWithRoles")
}

// DeleteMFADevice exists to satisfy auth.ClientI but is not implemented here.
// Use auth.GRPCServer.DeleteMFADevice or client.Client.DeleteMFADevice instead.
func (a *ServerWithRoles) DeleteMFADevice(ctx context.Context) (proto.AuthService_DeleteMFADeviceClient, error) {
	return nil, trace.NotImplemented("bug: DeleteMFADevice must not be called on auth.ServerWithRoles")
}

// AddMFADeviceSync is implemented by AuthService.AddMFADeviceSync.
func (a *ServerWithRoles) AddMFADeviceSync(ctx context.Context, req *proto.AddMFADeviceSyncRequest) (*proto.AddMFADeviceSyncResponse, error) {
	// The token provides its own authorization and authentication.
	res, err := a.authServer.AddMFADeviceSync(ctx, req)
	return res, trace.Wrap(err)
}

// DeleteMFADeviceSync is implemented by AuthService.DeleteMFADeviceSync.
func (a *ServerWithRoles) DeleteMFADeviceSync(ctx context.Context, req *proto.DeleteMFADeviceSyncRequest) error {
	// The token provides its own authorization and authentication.
	return a.authServer.DeleteMFADeviceSync(ctx, req)
}

// GenerateUserSingleUseCerts exists to satisfy auth.ClientI but is not
// implemented here.
//
// Use auth.GRPCServer.GenerateUserSingleUseCerts or
// client.Client.GenerateUserSingleUseCerts instead.
func (a *ServerWithRoles) GenerateUserSingleUseCerts(ctx context.Context) (proto.AuthService_GenerateUserSingleUseCertsClient, error) {
	return nil, trace.NotImplemented("bug: GenerateUserSingleUseCerts must not be called on auth.ServerWithRoles")
}

func (a *ServerWithRoles) IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	if !hasLocalUserRole(a.context) && !hasRemoteUserRole(a.context) {
		return nil, trace.AccessDenied("only a user role can call IsMFARequired, got %T", a.context.Checker)
	}
	return a.authServer.isMFARequired(ctx, a.context.Checker, req)
}

// SearchEvents allows searching audit events with pagination support.
func (a *ServerWithRoles) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) (events []apievents.AuditEvent, lastKey string, err error) {
	if err := a.action(apidefaults.Namespace, types.KindEvent, types.VerbList); err != nil {
		return nil, "", trace.Wrap(err)
	}

	events, lastKey, err = a.alog.SearchEvents(fromUTC, toUTC, namespace, eventTypes, limit, order, startKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return events, lastKey, nil
}

// SearchSessionEvents allows searching session audit events with pagination support.
func (a *ServerWithRoles) SearchSessionEvents(fromUTC, toUTC time.Time, limit int, order types.EventOrder, startKey string, cond *types.WhereExpr, sessionID string) (events []apievents.AuditEvent, lastKey string, err error) {
	if cond != nil {
		return nil, "", trace.BadParameter("cond is an internal parameter, should not be set by client")
	}

	cond, err = a.actionForListWithCondition(apidefaults.Namespace, types.KindSession, services.SessionIdentifier)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// TODO(codingllama): Refactor cond out of SearchSessionEvents and simplify signature.
	events, lastKey, err = a.alog.SearchSessionEvents(fromUTC, toUTC, limit, order, startKey, cond, sessionID)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return events, lastKey, nil
}

// GetLock gets a lock by name.
func (a *ServerWithRoles) GetLock(ctx context.Context, name string) (types.Lock, error) {
	if err := a.action(apidefaults.Namespace, types.KindLock, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetLock(ctx, name)
}

// GetLocks gets all/in-force locks that match at least one of the targets when specified.
func (a *ServerWithRoles) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	if err := a.action(apidefaults.Namespace, types.KindLock, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetLocks(ctx, inForceOnly, targets...)
}

// UpsertLock upserts a lock.
func (a *ServerWithRoles) UpsertLock(ctx context.Context, lock types.Lock) error {
	if err := a.action(apidefaults.Namespace, types.KindLock, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertLock(ctx, lock)
}

// DeleteLock deletes a lock.
func (a *ServerWithRoles) DeleteLock(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindLock, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteLock(ctx, name)
}

// DeleteAllLocks not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAllLocks(context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// ReplaceRemoteLocks replaces the set of locks associated with a remote cluster.
func (a *ServerWithRoles) ReplaceRemoteLocks(ctx context.Context, clusterName string, locks []types.Lock) error {
	role, ok := a.context.Identity.(RemoteBuiltinRole)
	if !a.hasRemoteBuiltinRole(string(types.RoleRemoteProxy)) || !ok || role.ClusterName != clusterName {
		return trace.AccessDenied("this request can be only executed by a remote proxy of cluster %q", clusterName)
	}
	return a.authServer.ReplaceRemoteLocks(ctx, clusterName, locks)
}

// StreamSessionEvents streams all events from a given session recording. An error is returned on the first
// channel if one is encountered. Otherwise the event channel is closed when the stream ends.
// The event channel is not closed on error to prevent race conditions in downstream select statements.
func (a *ServerWithRoles) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	if err := a.actionForKindSession(apidefaults.Namespace, types.VerbList, sessionID); err != nil {
		c, e := make(chan apievents.AuditEvent), make(chan error, 1)
		e <- trace.Wrap(err)
		return c, e
	}

	return a.alog.StreamSessionEvents(ctx, sessionID, startIndex)
}

// CreateApp creates a new application resource.
func (a *ServerWithRoles) CreateApp(ctx context.Context, app types.Application) error {
	if err := a.action(apidefaults.Namespace, types.KindApp, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	// Don't allow users create apps they wouldn't have access to (e.g.
	// non-matching labels).
	if err := a.checkAccessToApp(app); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.CreateApp(ctx, app))
}

// UpdateApp updates existing application resource.
func (a *ServerWithRoles) UpdateApp(ctx context.Context, app types.Application) error {
	if err := a.action(apidefaults.Namespace, types.KindApp, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	// Don't allow users update apps they don't have access to (e.g.
	// non-matching labels). Make sure to check existing app too.
	existing, err := a.authServer.GetApp(ctx, app.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkAccessToApp(existing); err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkAccessToApp(app); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.UpdateApp(ctx, app))
}

// GetApp returns specified application resource.
func (a *ServerWithRoles) GetApp(ctx context.Context, name string) (types.Application, error) {
	if err := a.action(apidefaults.Namespace, types.KindApp, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	app, err := a.authServer.GetApp(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.checkAccessToApp(app); err != nil {
		return nil, trace.Wrap(err)
	}
	return app, nil
}

// GetApps returns all application resources.
func (a *ServerWithRoles) GetApps(ctx context.Context) (result []types.Application, err error) {
	if err := a.action(apidefaults.Namespace, types.KindApp, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	// Filter out apps user doesn't have access to.
	apps, err := a.authServer.GetApps(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, app := range apps {
		if err := a.checkAccessToApp(app); err == nil {
			result = append(result, app)
		}
	}
	return result, nil
}

// DeleteApp removes the specified application resource.
func (a *ServerWithRoles) DeleteApp(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindApp, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	// Make sure user has access to the application before deleting.
	app, err := a.authServer.GetApp(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkAccessToApp(app); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.DeleteApp(ctx, name))
}

// DeleteAllApps removes all application resources.
func (a *ServerWithRoles) DeleteAllApps(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindApp, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	// Make sure to only delete apps user has access to.
	apps, err := a.authServer.GetApps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, app := range apps {
		if err := a.checkAccessToApp(app); err == nil {
			if err := a.authServer.DeleteApp(ctx, app.GetName()); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

func (a *ServerWithRoles) checkAccessToDatabase(database types.Database) error {
	return a.context.Checker.CheckAccess(database,
		// MFA is not required for operations on database resources but
		// will be enforced at the connection time.
		services.AccessMFAParams{Verified: true})
}

// CreateDatabase creates a new database resource.
func (a *ServerWithRoles) CreateDatabase(ctx context.Context, database types.Database) error {
	if err := a.action(apidefaults.Namespace, types.KindDatabase, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	// Don't allow users create databases they wouldn't have access to (e.g.
	// non-matching labels).
	if err := a.checkAccessToDatabase(database); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.CreateDatabase(ctx, database))
}

// UpdateDatabase updates existing database resource.
func (a *ServerWithRoles) UpdateDatabase(ctx context.Context, database types.Database) error {
	if err := a.action(apidefaults.Namespace, types.KindDatabase, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	// Don't allow users update databases they don't have access to (e.g.
	// non-matching labels). Make sure to check existing database too.
	existing, err := a.authServer.GetDatabase(ctx, database.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkAccessToDatabase(existing); err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkAccessToDatabase(database); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.UpdateDatabase(ctx, database))
}

// GetDatabase returns specified database resource.
func (a *ServerWithRoles) GetDatabase(ctx context.Context, name string) (types.Database, error) {
	if err := a.action(apidefaults.Namespace, types.KindDatabase, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	database, err := a.authServer.GetDatabase(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.checkAccessToDatabase(database); err != nil {
		return nil, trace.Wrap(err)
	}
	return database, nil
}

// GetDatabases returns all database resources.
func (a *ServerWithRoles) GetDatabases(ctx context.Context) (result []types.Database, err error) {
	if err := a.action(apidefaults.Namespace, types.KindDatabase, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	// Filter out databases user doesn't have access to.
	databases, err := a.authServer.GetDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, database := range databases {
		if err := a.checkAccessToDatabase(database); err == nil {
			result = append(result, database)
		}
	}
	return result, nil
}

// DeleteDatabase removes the specified database resource.
func (a *ServerWithRoles) DeleteDatabase(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindDatabase, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	// Make sure user has access to the database before deleting.
	database, err := a.authServer.GetDatabase(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkAccessToDatabase(database); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.DeleteDatabase(ctx, name))
}

// DeleteAllDatabases removes all database resources.
func (a *ServerWithRoles) DeleteAllDatabases(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindDatabase, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	// Make sure to only delete databases user has access to.
	databases, err := a.authServer.GetDatabases(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, database := range databases {
		if err := a.checkAccessToDatabase(database); err == nil {
			if err := a.authServer.DeleteDatabase(ctx, database.GetName()); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// GetWindowsDesktopServices returns all registered windows desktop services.
func (a *ServerWithRoles) GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error) {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktopService, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	services, err := a.authServer.GetWindowsDesktopServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services, nil
}

// GetWindowsDesktopService returns a registered windows desktop service by name.
func (a *ServerWithRoles) GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error) {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktopService, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	service, err := a.authServer.GetWindowsDesktopService(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return service, nil
}

// UpsertWindowsDesktopService creates or updates a new windows desktop service.
func (a *ServerWithRoles) UpsertWindowsDesktopService(ctx context.Context, s types.WindowsDesktopService) (*types.KeepAlive, error) {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktopService, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.UpsertWindowsDesktopService(ctx, s)
}

// DeleteWindowsDesktopService removes the specified windows desktop service.
func (a *ServerWithRoles) DeleteWindowsDesktopService(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktopService, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteWindowsDesktopService(ctx, name)
}

// DeleteAllWindowsDesktopServices removes all registered windows desktop services.
func (a *ServerWithRoles) DeleteAllWindowsDesktopServices(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktopService, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllWindowsDesktopServices(ctx)
}

// GetWindowsDesktops returns all registered windows desktop hosts.
func (a *ServerWithRoles) GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktop, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	hosts, err := a.authServer.GetWindowsDesktops(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	filtered, err := a.filterWindowsDesktops(hosts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return filtered, nil
}

// CreateWindowsDesktop creates a new windows desktop host.
func (a *ServerWithRoles) CreateWindowsDesktop(ctx context.Context, s types.WindowsDesktop) error {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktop, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateWindowsDesktop(ctx, s)
}

// UpdateWindowsDesktop updates an existing windows desktop host.
func (a *ServerWithRoles) UpdateWindowsDesktop(ctx context.Context, s types.WindowsDesktop) error {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktop, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	existing, err := a.authServer.GetWindowsDesktops(ctx,
		types.WindowsDesktopFilter{HostID: s.GetHostID(), Name: s.GetName()})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(existing) == 0 {
		return trace.NotFound("no windows desktops with HostID %s and Name %s",
			s.GetHostID(), s.GetName())
	}

	if err := a.checkAccessToWindowsDesktop(existing[0]); err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkAccessToWindowsDesktop(s); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpdateWindowsDesktop(ctx, s)
}

// UpsertWindowsDesktop updates a windows desktop resource, creating it if it doesn't exist.
func (a *ServerWithRoles) UpsertWindowsDesktop(ctx context.Context, s types.WindowsDesktop) error {
	// Ensure caller has both Create and Update permissions.
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktop, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	if s.GetHostID() == "" {
		// dont try to insert desktops with empty hostIDs
		return nil
	}

	// If the desktop exists, check access,
	// if it doesn't, continue.
	existing, err := a.authServer.GetWindowsDesktops(ctx,
		types.WindowsDesktopFilter{HostID: s.GetHostID(), Name: s.GetName()})
	if err == nil && len(existing) != 0 {
		if err := a.checkAccessToWindowsDesktop(existing[0]); err != nil {
			return trace.Wrap(err)
		}
	} else if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if err := a.checkAccessToWindowsDesktop(s); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertWindowsDesktop(ctx, s)
}

// DeleteWindowsDesktop removes the specified Windows desktop host.
// Note: unlike GetWindowsDesktops, this will delete at-most one desktop.
// Passing an empty host ID will not trigger "delete all" behavior. To delete
// all desktops, use DeleteAllWindowsDesktops.
func (a *ServerWithRoles) DeleteWindowsDesktop(ctx context.Context, hostID, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktop, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	desktop, err := a.authServer.GetWindowsDesktops(ctx,
		types.WindowsDesktopFilter{HostID: hostID, Name: name})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(desktop) == 0 {
		return trace.NotFound("no windows desktops with HostID %s and Name %s",
			hostID, name)
	}
	if err := a.checkAccessToWindowsDesktop(desktop[0]); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteWindowsDesktop(ctx, hostID, name)
}

// DeleteAllWindowsDesktops removes all registered windows desktop hosts.
func (a *ServerWithRoles) DeleteAllWindowsDesktops(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktop, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	// Only delete the desktops the user has access to.
	desktops, err := a.authServer.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, desktop := range desktops {
		if err := a.checkAccessToWindowsDesktop(desktop); err == nil {
			if err := a.authServer.DeleteWindowsDesktop(ctx, desktop.GetHostID(), desktop.GetName()); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

func (a *ServerWithRoles) filterWindowsDesktops(desktops []types.WindowsDesktop) ([]types.WindowsDesktop, error) {
	// For certain built-in roles allow full access
	if a.hasBuiltinRole(types.RoleAdmin, types.RoleProxy, types.RoleWindowsDesktop) {
		return desktops, nil
	}

	filtered := make([]types.WindowsDesktop, 0, len(desktops))
	for _, desktop := range desktops {
		if err := a.checkAccessToWindowsDesktop(desktop); err == nil {
			filtered = append(filtered, desktop)
		}
	}

	return filtered, nil
}

func (a *ServerWithRoles) checkAccessToWindowsDesktop(w types.WindowsDesktop) error {
	return a.context.Checker.CheckAccess(w,
		// MFA is not required for operations on desktop resources
		services.AccessMFAParams{Verified: true},
		// Note: we don't use the Windows login matcher here, as we won't know what OS user
		// the user is trying to log in as until they initiate the connection.
	)
}

// GenerateWindowsDesktopCert generates a certificate for Windows RDP
// authentication.
func (a *ServerWithRoles) GenerateWindowsDesktopCert(ctx context.Context, req *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error) {
	// Only windows_desktop_service should be requesting Windows certificates.
	if !a.hasBuiltinRole(types.RoleWindowsDesktop) {
		return nil, trace.AccessDenied("access denied")
	}
	return a.authServer.GenerateWindowsDesktopCert(ctx, req)
}

// StartAccountRecovery is implemented by AuthService.StartAccountRecovery.
func (a *ServerWithRoles) StartAccountRecovery(ctx context.Context, req *proto.StartAccountRecoveryRequest) (types.UserToken, error) {
	return a.authServer.StartAccountRecovery(ctx, req)
}

// VerifyAccountRecovery is implemented by AuthService.VerifyAccountRecovery.
func (a *ServerWithRoles) VerifyAccountRecovery(ctx context.Context, req *proto.VerifyAccountRecoveryRequest) (types.UserToken, error) {
	// The token provides its own authorization and authentication.
	return a.authServer.VerifyAccountRecovery(ctx, req)
}

// CompleteAccountRecovery is implemented by AuthService.CompleteAccountRecovery.
func (a *ServerWithRoles) CompleteAccountRecovery(ctx context.Context, req *proto.CompleteAccountRecoveryRequest) error {
	// The token provides its own authorization and authentication.
	return a.authServer.CompleteAccountRecovery(ctx, req)
}

// CreateAccountRecoveryCodes is implemented by AuthService.CreateAccountRecoveryCodes.
func (a *ServerWithRoles) CreateAccountRecoveryCodes(ctx context.Context, req *proto.CreateAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
	return a.authServer.CreateAccountRecoveryCodes(ctx, req)
}

// GetAccountRecoveryToken is implemented by AuthService.GetAccountRecoveryToken.
func (a *ServerWithRoles) GetAccountRecoveryToken(ctx context.Context, req *proto.GetAccountRecoveryTokenRequest) (types.UserToken, error) {
	return a.authServer.GetAccountRecoveryToken(ctx, req)
}

// CreateAuthenticateChallenge is implemented by AuthService.CreateAuthenticateChallenge.
func (a *ServerWithRoles) CreateAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	// No permission check is required b/c this request verifies request by one of the following:
	//   - username + password, anyone who has user's password can generate a sign request
	//   - token provide its own auth
	//   - the user extracted from context can retrieve their own challenges
	return a.authServer.CreateAuthenticateChallenge(ctx, req)
}

// CreatePrivilegeToken is implemented by AuthService.CreatePrivilegeToken.
func (a *ServerWithRoles) CreatePrivilegeToken(ctx context.Context, req *proto.CreatePrivilegeTokenRequest) (*types.UserTokenV3, error) {
	return a.authServer.CreatePrivilegeToken(ctx, req)
}

// CreateRegisterChallenge is implemented by AuthService.CreateRegisterChallenge.
func (a *ServerWithRoles) CreateRegisterChallenge(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
	// The token provides its own authorization and authentication.
	return a.authServer.CreateRegisterChallenge(ctx, req)
}

// GetAccountRecoveryCodes is implemented by AuthService.GetAccountRecoveryCodes.
func (a *ServerWithRoles) GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
	// User in context can retrieve their own recovery codes.
	return a.authServer.GetAccountRecoveryCodes(ctx, req)
}

// GenerateCertAuthorityCRL generates an empty CRL for a CA.
func (a *ServerWithRoles) GenerateCertAuthorityCRL(ctx context.Context, caType types.CertAuthType) ([]byte, error) {
	// Only windows_desktop_service should be requesting CRLs
	if !a.hasBuiltinRole(types.RoleWindowsDesktop) {
		return nil, trace.AccessDenied("access denied")
	}
	crl, err := a.authServer.GenerateCertAuthorityCRL(ctx, caType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return crl, nil
}

// UpdatePresence is coupled to the service layer and must exist here but is never actually called
// since it's handled by the session presence task. This is never valid to call.
func (a *ServerWithRoles) UpdatePresence(ctx context.Context, sessionID, user string) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpdatePresence is coupled to the service layer and must exist here but is never actually called
// since it's handled by the session presence task. This is never valid to call.
func (a *ServerWithRoles) MaintainSessionPresence(ctx context.Context) (proto.AuthService_MaintainSessionPresenceClient, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// NewAdminAuthServer returns auth server authorized as admin,
// used for auth server cached access
func NewAdminAuthServer(authServer *Server, sessions session.Service, alog events.IAuditLog) (ClientI, error) {
	ctx, err := NewAdminContext()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ServerWithRoles{
		authServer: authServer,
		context:    *ctx,
		alog:       alog,
		sessions:   sessions,
	}, nil
}

func emitSSOLoginFailureEvent(ctx context.Context, emitter apievents.Emitter, method string, err error, testFlow bool) {
	code := events.UserSSOLoginFailureCode
	if testFlow {
		code = events.UserSSOTestFlowLoginFailureCode
	}

	emitErr := emitter.EmitAuditEvent(ctx, &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: code,
		},
		Method: method,
		Status: apievents.Status{
			Success:     false,
			Error:       trace.Unwrap(err).Error(),
			UserMessage: err.Error(),
		},
	})

	if emitErr != nil {
		log.WithError(err).Warnf("Failed to emit %v login failure event.", method)
	}
}

// verbsToReplaceResourceWithOrigin determines the verbs/actions required of a role
// to replace the resource currently stored in the backend.
func verbsToReplaceResourceWithOrigin(stored types.ResourceWithOrigin) []string {
	verbs := []string{types.VerbUpdate}
	if stored.Origin() == types.OriginConfigFile {
		verbs = append(verbs, types.VerbCreate)
	}
	return verbs
}
