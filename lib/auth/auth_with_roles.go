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

package auth

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	collectortracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	otlpcommonv1 "go.opentelemetry.io/proto/otlp/common/v1"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/clusterconfig/clusterconfigv1"
	"github.com/gravitational/teleport/lib/auth/okta"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// ServerWithRoles is a wrapper around auth service
// methods that focuses on authorizing every request
type ServerWithRoles struct {
	authServer *Server
	alog       events.AuditLogSessionStreamer
	// context holds authorization context
	context authz.Context
}

// CloseContext is closed when the auth server shuts down
func (a *ServerWithRoles) CloseContext() context.Context {
	return a.authServer.closeCtx
}

// actionForResource will determine if a user has access to the given resource. This call respects where clauses.
func (a *ServerWithRoles) actionForResource(resource types.Resource, kind string, verbs ...string) error {
	sctx := &services.Context{User: a.context.User}
	if resource != nil {
		sctx.Resource = resource
	}
	return trace.Wrap(a.actionWithContext(sctx, apidefaults.Namespace, kind, verbs...))
}

// actionWithContext will determine if a user has access given a services.Context. This call respects where clauses.
func (a *ServerWithRoles) actionWithContext(ctx *services.Context, namespace, resource string, verbs ...string) error {
	if len(verbs) == 0 {
		return trace.BadParameter("no verbs provided for authorization check on resource %q", resource)
	}
	var errs []error
	for _, verb := range verbs {
		errs = append(errs, a.context.Checker.CheckAccessToRule(ctx, namespace, resource, verb))
	}
	if err := trace.NewAggregate(errs...); err != nil {
		return err
	}
	return nil
}

type actionConfig struct {
	context authz.Context
}

type actionOption func(*actionConfig)

func (a *ServerWithRoles) withOptions(opts ...actionOption) actionConfig {
	cfg := actionConfig{context: a.context}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// action will determine if a user has access to the given resource kind. This does not respect where clauses.
func (c actionConfig) action(namespace, resource string, verbs ...string) error {
	if len(verbs) == 0 {
		return trace.BadParameter("no verbs provided for authorization check on resource %q", resource)
	}
	var errs []error
	for _, verb := range verbs {
		errs = append(errs, c.context.Checker.CheckAccessToRule(&services.Context{User: c.context.User}, namespace, resource, verb))
	}
	if err := trace.NewAggregate(errs...); err != nil {
		return err
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
	if authz.IsCurrentUser(a.context, username) {
		return nil
	}
	return a.context.Checker.CheckAccessToRule(&services.Context{User: a.context.User},
		apidefaults.Namespace, types.KindUser, types.VerbCreate)
}

// authConnectorAction is a special checker that grants access to auth
// connectors. It first checks if you have access to the specific connector.
// If not, it checks if the requester has the meta KindAuthConnector access
// (which grants access to all connectors).
func (a *ServerWithRoles) authConnectorAction(namespace string, resource string, verb string) error {
	if err := a.context.Checker.CheckAccessToRule(&services.Context{User: a.context.User}, namespace, resource, verb); err != nil {
		if err := a.context.Checker.CheckAccessToRule(&services.Context{User: a.context.User}, namespace, types.KindAuthConnector, verb); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// actionForListWithCondition extracts a restrictive filter condition to be
// added to a list query after a simple resource check fails.
func (a *ServerWithRoles) actionForListWithCondition(namespace, resource, identifier string) (*types.WhereExpr, error) {
	origErr := a.action(namespace, resource, types.VerbList)
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
	origErr := a.context.Checker.CheckAccessToRule(ruleCtx, namespace, kind, verb)
	if origErr == nil || !trace.IsAccessDenied(origErr) {
		return trace.Wrap(origErr)
	}
	if err := extendContext(ruleCtx); err != nil {
		log.WithError(err).Warning("Failed to extend context for second RBAC check.")
		// Return the original AccessDenied to avoid leaking information.
		return trace.Wrap(origErr)
	}
	return trace.Wrap(a.context.Checker.CheckAccessToRule(ruleCtx, namespace, kind, verb))
}

// actionForKindSession is a special checker that grants access to session
// recordings. It can allow access to a specific recording based on the
// `where` section of the user's access rule for kind `session`.
func (a *ServerWithRoles) actionForKindSession(namespace string, sid session.ID) error {
	extendContext := func(ctx *services.Context) error {
		sessionEnd, err := a.findSessionEndEvent(namespace, sid)
		ctx.Session = sessionEnd
		return trace.Wrap(err)
	}

	return trace.Wrap(a.actionWithExtendedContext(namespace, types.KindSession, types.VerbRead, extendContext))
}

// localServerAction returns an access denied error if the role is not one of the builtin server roles.
func (a *ServerWithRoles) localServerAction() error {
	role, ok := a.context.Identity.(authz.BuiltinRole)
	if !ok || !role.IsServer() {
		return trace.AccessDenied("this request can be only executed by a teleport built-in server")
	}
	return nil
}

// remoteServerAction returns an access denied error if the role is not one of the remote builtin server roles.
func (a *ServerWithRoles) remoteServerAction() error {
	role, ok := a.context.UnmappedIdentity.(authz.RemoteBuiltinRole)
	if !ok || !role.IsRemoteServer() {
		return trace.AccessDenied("this request can be only executed by a teleport remote server")
	}
	return nil
}

// isLocalOrRemoteServerAction returns true if the role is one of the builtin server roles (local or remote).
func (a *ServerWithRoles) isLocalOrRemoteServerAction() bool {
	errLocal := a.localServerAction()
	errRemote := a.remoteServerAction()
	return errLocal == nil || errRemote == nil
}

// hasBuiltinRole checks that the attached identity is a builtin role and
// whether any of the given roles match the role set.
func (a *ServerWithRoles) hasBuiltinRole(roles ...types.SystemRole) bool {
	for _, role := range roles {
		if authz.HasBuiltinRole(a.context, string(role)) {
			return true
		}
	}
	return false
}

// HasBuiltinRole checks if the identity is a builtin role with the matching
// name.
// Deprecated: use authz.HasBuiltinRole instead.
func HasBuiltinRole(authContext authz.Context, name string) bool {
	// TODO(jakule): This function can be removed once teleport.e is updated
	// to use authz.HasBuiltinRole.
	return authz.HasBuiltinRole(authContext, name)
}

// HasRemoteBuiltinRole checks if the identity is a remote builtin role with the
// matching name.
func HasRemoteBuiltinRole(authContext authz.Context, name string) bool {
	if _, ok := authContext.UnmappedIdentity.(authz.RemoteBuiltinRole); !ok {
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

// CreateSessionTracker creates a tracker resource for an active session.
func (a *ServerWithRoles) CreateSessionTracker(ctx context.Context, tracker types.SessionTracker) (types.SessionTracker, error) {
	if err := a.localServerAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	tracker, err := a.authServer.CreateSessionTracker(ctx, tracker)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tracker, nil
}

func (a *ServerWithRoles) filterSessionTracker(joinerRoles []types.Role, tracker types.SessionTracker, verb string) bool {
	// Apply RFD 45 RBAC rules to the session if it's SSH.
	// This is a bit of a hack. It converts to the old legacy format
	// which we don't have all data for, luckily the fields we don't have aren't made available
	// to the RBAC filter anyway.
	if tracker.GetSessionKind() == types.SSHSessionKind {
		ruleCtx := &services.Context{User: a.context.User}
		ruleCtx.SSHSession = &session.Session{
			Kind:           tracker.GetSessionKind(),
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
		if err := a.context.Checker.CheckAccessToRule(ruleCtx, apidefaults.Namespace, types.KindSSHSession, verb); err != nil {
			return false
		}
	}

	ruleCtx := &services.Context{User: a.context.User, SessionTracker: tracker}
	if a.context.Checker.CheckAccessToRule(ruleCtx, apidefaults.Namespace, types.KindSessionTracker, types.VerbList) == nil {
		return true
	}

	evaluator := NewSessionAccessEvaluator(tracker.GetHostPolicySets(), tracker.GetSessionKind(), tracker.GetHostUser())
	modes := evaluator.CanJoin(SessionAccessContext{Username: a.context.User.GetName(), Roles: joinerRoles})
	return len(modes) != 0
}

const (
	forwardedTag = "teleport.forwarded.for"
)

// Export forwards OTLP traces to the upstream collector configured in the tracing service. This allows for
// tsh, tctl, etc to be able to export traces without having to know how to connect to the upstream collector
// for the cluster.
//
// All spans received will have a `teleport.forwarded.for` attribute added to them with the value being one of
// two things depending on the role of the forwarder:
//  1. User forwarded: `teleport.forwarded.for: alice`
//  2. Instance forwarded: `teleport.forwarded.for: Proxy.clustername:Proxy,Node,Instance`
//
// This allows upstream consumers of the spans to be able to identify forwarded spans and act on them accordingly.
func (a *ServerWithRoles) Export(ctx context.Context, req *collectortracev1.ExportTraceServiceRequest) (*collectortracev1.ExportTraceServiceResponse, error) {
	var sb strings.Builder

	sb.WriteString(a.context.User.GetName())

	// if forwarded on behalf of a Teleport service add its system roles
	if role, ok := a.context.Identity.(authz.BuiltinRole); ok {
		sb.WriteRune(':')
		sb.WriteString(role.Role.String())
		if len(role.AdditionalSystemRoles) > 0 {
			sb.WriteRune(',')
			sb.WriteString(role.AdditionalSystemRoles.String())
		}
	}

	// the forwarded attribute to add
	value := &otlpcommonv1.KeyValue{
		Key: forwardedTag,
		Value: &otlpcommonv1.AnyValue{
			Value: &otlpcommonv1.AnyValue_StringValue{
				StringValue: sb.String(),
			},
		},
	}

	// returns the index at which the attribute with
	// the forwardedTag key exists, -1 if not found
	tagIndex := func(attrs []*otlpcommonv1.KeyValue) int {
		for i, attr := range attrs {
			if attr.Key == forwardedTag {
				return i
			}
		}

		return -1
	}

	for _, resourceSpans := range req.ResourceSpans {
		// if there is a resource, tag it with the
		// forwarded attribute instead of each of tagging
		// each span
		if resourceSpans.Resource != nil {
			if index := tagIndex(resourceSpans.Resource.Attributes); index != -1 {
				resourceSpans.Resource.Attributes[index] = value
			} else {
				resourceSpans.Resource.Attributes = append(resourceSpans.Resource.Attributes, value)
			}

			// override any span attributes with a forwarded tag,
			// but we don't need to add one if the span isn't already
			// tagged since we just tagged the resource
			for _, scopeSpans := range resourceSpans.ScopeSpans {
				for _, span := range scopeSpans.Spans {
					if index := tagIndex(span.Attributes); index != -1 {
						span.Attributes[index] = value
					}
				}
			}

			continue
		}

		// there was no resource, so we must now tag all the
		// individual spans with the forwarded tag
		for _, scopeSpans := range resourceSpans.ScopeSpans {
			for _, span := range scopeSpans.Spans {
				if index := tagIndex(span.Attributes); index != -1 {
					span.Attributes[index] = value
				} else {
					span.Attributes = append(span.Attributes, value)
				}
			}
		}
	}

	if err := a.authServer.traceClient.UploadTraces(ctx, req.ResourceSpans); err != nil {
		return &collectortracev1.ExportTraceServiceResponse{}, trace.Wrap(err)
	}

	return &collectortracev1.ExportTraceServiceResponse{}, nil
}

// GetSessionTracker returns the current state of a session tracker for an active session.
func (a *ServerWithRoles) GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	tracker, err := a.authServer.GetSessionTracker(ctx, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.localServerAction(); err == nil {
		return tracker, nil
	}

	user := a.context.User
	joinerRoles, err := services.FetchRoles(user.GetRoles(), a.authServer, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ok := a.filterSessionTracker(joinerRoles, tracker, types.VerbRead)
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

	if err := a.localServerAction(); err == nil {
		return sessions, nil
	}

	var filteredSessions []types.SessionTracker
	user := a.context.User
	joinerRoles, err := services.FetchRoles(user.GetRoles(), a.authServer, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, sess := range sessions {
		ok := a.filterSessionTracker(joinerRoles, sess, types.VerbList)
		if ok {
			filteredSessions = append(filteredSessions, sess)
		}
	}

	return filteredSessions, nil
}

// GetActiveSessionTrackersWithFilter returns a list of active sessions filtered by a filter.
func (a *ServerWithRoles) GetActiveSessionTrackersWithFilter(ctx context.Context, filter *types.SessionTrackerFilter) ([]types.SessionTracker, error) {
	sessions, err := a.authServer.GetActiveSessionTrackersWithFilter(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.localServerAction(); err == nil {
		return sessions, nil
	}

	var filteredSessions []types.SessionTracker
	user := a.context.User
	joinerRoles, err := services.FetchRoles(user.GetRoles(), a.authServer, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, sess := range sessions {
		ok := a.filterSessionTracker(joinerRoles, sess, types.VerbList)
		if ok {
			filteredSessions = append(filteredSessions, sess)
		}
	}

	return filteredSessions, nil
}

// RemoveSessionTracker removes a tracker resource for an active session.
func (a *ServerWithRoles) RemoveSessionTracker(ctx context.Context, sessionID string) error {
	if err := a.localServerAction(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.RemoveSessionTracker(ctx, sessionID)
}

// UpdateSessionTracker updates a tracker resource for an active session.
func (a *ServerWithRoles) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error {
	if err := a.localServerAction(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.UpdateSessionTracker(ctx, req)
}

// AuthenticateWebUser authenticates web user, creates and returns a web session
// in case authentication is successful
func (a *ServerWithRoles) AuthenticateWebUser(ctx context.Context, req authclient.AuthenticateUserRequest) (types.WebSession, error) {
	// authentication request has it's own authentication, however this limits the requests
	// types to proxies to make it harder to break
	if !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.AuthenticateWebUser(ctx, req)
}

// AuthenticateSSHUser authenticates SSH console user, creates and  returns a pair of signed TLS and SSH
// short lived certificates as a result
func (a *ServerWithRoles) AuthenticateSSHUser(ctx context.Context, req authclient.AuthenticateSSHRequest) (*authclient.SSHLoginResponse, error) {
	// authentication request has it's own authentication, however this limits the requests
	// types to proxies to make it harder to break
	if !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.AuthenticateSSHUser(ctx, req)
}

// GenerateOpenSSHCert signs a SSH certificate that can be used
// to connect to Agentless nodes.
func (a *ServerWithRoles) GenerateOpenSSHCert(ctx context.Context, req *proto.OpenSSHCertRequest) (*proto.OpenSSHCert, error) {
	// this limits the requests types to proxies to make it harder to break
	if !a.hasBuiltinRole(types.RoleProxy) && !a.hasRemoteBuiltinRole(string(types.RoleRemoteProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.GenerateOpenSSHCert(ctx, req)
}

// CompareAndSwapCertAuthority updates existing cert authority if the existing cert authority
// value matches the value stored in the backend.
func (a *ServerWithRoles) CompareAndSwapCertAuthority(new, existing types.CertAuthority) error {
	if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CompareAndSwapCertAuthority(new, existing)
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

func (a *ServerWithRoles) RegisterUsingToken(ctx context.Context, req *types.RegisterUsingTokenRequest) (*proto.Certs, error) {
	isProxy := a.hasBuiltinRole(types.RoleProxy)

	// We do not trust remote addr in the request unless it's coming from the Proxy.
	if !isProxy || req.RemoteAddr == "" {
		if err := setRemoteAddrFromContext(ctx, req); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Similarly, do not trust bot instance IDs or generation values in the
	// request unless from a component with the proxy role (e.g. the join
	// service). They will be derived from the client certificate otherwise.
	if !isProxy {
		if req.BotInstanceID != "" {
			log.WithFields(logrus.Fields{
				"bot_instance_id": req.BotInstanceID,
			}).Warnf("Untrusted client attempted to provide a bot instance ID, this will be ignored")

			req.BotInstanceID = ""
		}

		if req.BotGeneration > 0 {
			log.WithFields(logrus.Fields{
				"bot_generation": req.BotGeneration,
			}).Warnf("Untrusted client attempted to provide a bot generation, this will be ignored")

			req.BotGeneration = 0
		}
	}

	// If the identity has a BotInstanceID or BotGeneration included, copy it
	// onto the request - but only if one wasn't already passed along via the
	// proxy.
	ident := a.context.Identity.GetIdentity()
	req.BotInstanceID = cmp.Or(req.BotInstanceID, ident.BotInstanceID)
	req.BotGeneration = cmp.Or(req.BotGeneration, int32(ident.Generation))

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
func (a *ServerWithRoles) RegisterUsingIAMMethod(ctx context.Context, challengeResponse client.RegisterIAMChallengeResponseFunc) (*proto.Certs, error) {
	certs, err := a.authServer.RegisterUsingIAMMethod(ctx, challengeResponse)
	return certs, trace.Wrap(err)
}

// RegisterUsingAzureMethod registers the caller using the Azure join method and
// returns signed certs to join the cluster.
//
// See (*Server).RegisterUsingAzureMethod for further documentation.
//
// This wrapper does not do any extra authz checks, as the register method has
// its own authz mechanism.
func (a *ServerWithRoles) RegisterUsingAzureMethod(ctx context.Context, challengeResponse client.RegisterAzureChallengeResponseFunc) (*proto.Certs, error) {
	certs, err := a.authServer.RegisterUsingAzureMethod(ctx, challengeResponse)
	return certs, trace.Wrap(err)
}

// RegisterUsingTPMMethod registers the caller using the TPM join method and
// returns signed certs to join the cluster.
//
// See (*Server).RegisterUsingTPMMethod for further documentation.
//
// This wrapper does not do any extra authz checks, as the register method has
// its own authz mechanism.
func (a *ServerWithRoles) RegisterUsingTPMMethod(
	ctx context.Context,
	initReq *proto.RegisterUsingTPMMethodInitialRequest,
	solveChallenge client.RegisterTPMChallengeResponseFunc,
) (*proto.Certs, error) {
	certs, err := a.authServer.registerUsingTPMMethod(
		ctx, initReq, solveChallenge,
	)
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
	if a.context.User.GetName() != authclient.HostFQDN(req.HostID, clusterName) {
		return nil, trace.AccessDenied("username mismatch %q and %q", a.context.User.GetName(), authclient.HostFQDN(req.HostID, clusterName))
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
	role, ok := a.context.Identity.(authz.BuiltinRole)
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
	var assertions proto.SystemRoleAssertionSet
	var err error
	if req.SystemRoleAssertionID != "" {
		assertions, err = a.authServer.GetSystemRoleAssertions(ctx, req.HostID, req.SystemRoleAssertionID)
		if err != nil {
			// include this error in the logs, since it might be indicative of a bug if it occurs outside of the context
			// of a general backend outage.
			log.Warnf("Failed to load system role assertion set %q for instance %q: %v", req.SystemRoleAssertionID, req.HostID, err)
			return trace.AccessDenied("failed to load system role assertion set with ID %q", req.SystemRoleAssertionID)
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

// AssertSystemRole is used by agents to prove that they have a given system role when their credentials
// originate from multiple separate join tokens so that they can be issued an instance certificate that
// encompasses all of their capabilities. This method will be deprecated once we have a more comprehensive
// model for join token joining/replacement.
func (a *ServerWithRoles) AssertSystemRole(ctx context.Context, req proto.SystemRoleAssertion) error {
	role, ok := a.context.Identity.(authz.BuiltinRole)
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

	return a.authServer.AssertSystemRole(ctx, req)
}

// RegisterInventoryControlStream handles the upstream half of the control stream handshake, then passes the control stream to
// the auth server's main control logic. We also return the post-auth hello message back up to the grpcserver layer in order to
// use it for metrics purposes.
func (a *ServerWithRoles) RegisterInventoryControlStream(ics client.UpstreamInventoryControlStream) (proto.UpstreamInventoryHello, error) {
	// this value gets set further down
	var hello proto.UpstreamInventoryHello

	// Ensure that caller is a teleport server
	role, ok := a.context.Identity.(authz.BuiltinRole)
	if !ok || !role.IsServer() {
		return hello, trace.AccessDenied("inventory control streams can only be created by a teleport built-in server")
	}

	// wait for upstream hello
	select {
	case msg := <-ics.Recv():
		switch m := msg.(type) {
		case proto.UpstreamInventoryHello:
			hello = m
		default:
			return hello, trace.BadParameter("expected upstream hello, got: %T", m)
		}
	case <-ics.Done():
		return hello, trace.Wrap(ics.Error())
	case <-a.CloseContext().Done():
		return hello, trace.Errorf("auth server shutdown")
	}

	// verify that server is creating stream on behalf of itself.
	if hello.ServerID != role.GetServerID() {
		return hello, trace.AccessDenied("control streams do not support impersonation (%q -> %q)", role.GetServerID(), hello.ServerID)
	}

	// in order to reduce sensitivity to downgrades/misconfigurations, we simply filter out
	// services that are unrecognized or unauthorized, rather than rejecting hellos that claim them.
	var filteredServices []types.SystemRole
	for _, service := range hello.Services {
		if !a.hasBuiltinRole(service) {
			log.Warnf("Omitting service %q for control stream of instance %q (unknown or unauthorized).", service, role.GetServerID())
			continue
		}
		filteredServices = append(filteredServices, service)
	}

	hello.Services = filteredServices

	return hello, a.authServer.RegisterInventoryControlStream(ics, hello)
}

func (a *ServerWithRoles) GetInventoryStatus(ctx context.Context, req proto.InventoryStatusRequest) (proto.InventoryStatusSummary, error) {
	if err := a.action(apidefaults.Namespace, types.KindInstance, types.VerbList, types.VerbRead); err != nil {
		return proto.InventoryStatusSummary{}, trace.Wrap(err)
	}

	if req.Connected {
		if !a.hasBuiltinRole(types.RoleAdmin) {
			return proto.InventoryStatusSummary{}, trace.AccessDenied("requires local tctl, try using 'tctl inventory ls' instead")
		}
	}
	return a.authServer.GetInventoryStatus(ctx, req)
}

// GetInventoryConnectedServiceCounts returns the counts of each connected service seen in the inventory.
func (a *ServerWithRoles) GetInventoryConnectedServiceCounts() (proto.InventoryConnectedServiceCounts, error) {
	// TODO(fspmarshall): switch this to being scoped to instance:read once we have a sane remote version of
	// this method. for now we're leaving it as requiring local admin because the returned value is basically
	// nonsense if you aren't connected locally.
	if !a.hasBuiltinRole(types.RoleAdmin) {
		return proto.InventoryConnectedServiceCounts{}, trace.AccessDenied("requires builtin admin role")
	}
	return a.authServer.GetInventoryConnectedServiceCounts(), nil
}

func (a *ServerWithRoles) PingInventory(ctx context.Context, req proto.InventoryPingRequest) (proto.InventoryPingResponse, error) {
	// this is scoped to admin-only not because we don't have appropriate rbac, but because this method doesn't function
	// as expected if you aren't connected locally.
	if !a.hasBuiltinRole(types.RoleAdmin) {
		return proto.InventoryPingResponse{}, trace.AccessDenied("requires builtin admin role")
	}
	return a.authServer.PingInventory(ctx, req)
}

func (a *ServerWithRoles) GetInstances(ctx context.Context, filter types.InstanceFilter) stream.Stream[types.Instance] {
	if err := a.action(apidefaults.Namespace, types.KindInstance, types.VerbList, types.VerbRead); err != nil {
		return stream.Fail[types.Instance](trace.Wrap(err))
	}

	return a.authServer.GetInstances(ctx, filter)
}

// GetNodeStream returns a stream of nodes.
func (a *ServerWithRoles) GetNodeStream(ctx context.Context, namespace string) stream.Stream[types.Server] {
	if err := a.action(namespace, types.KindNode, types.VerbList, types.VerbRead); err != nil {
		return stream.Fail[types.Server](trace.Wrap(err))
	}

	return a.authServer.GetNodeStream(ctx, namespace)
}

func (a *ServerWithRoles) GetClusterAlerts(ctx context.Context, query types.GetClusterAlertsRequest) ([]types.ClusterAlert, error) {
	// unauthenticated clients can never check for alerts. we don't normally explicitly
	// check for this kind of thing, but since alerts use an unusual access-control
	// pattern, explicitly rejecting the nop role makes things easier.
	if a.hasBuiltinRole(types.RoleNop) {
		return nil, trace.AccessDenied("alerts not available to unauthenticated clients")
	}

	alerts, err := a.authServer.GetClusterAlerts(ctx, query)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var acks []types.AlertAcknowledgement
	if !query.WithAcknowledged {
		// load acks so that we can filter out acknowledged alerts
		acks, err = a.authServer.GetAlertAcks(ctx)
		if err != nil {
			// we don't fail here since users are allowed to see acknowledged alerts, acks
			// are intended only as a tool for reducing noise.
			log.Warnf("Failed to load alert acks: %v", err)
		}
	}

	// by default we only show alerts whose labels specify that a given user should see them, but users
	// with permissions to view all resources of kind 'cluster_alert' can opt into viewing all alerts
	// regardless of labels for management/debug purposes.
	var resourceLevelPermit bool
	if query.WithUntargeted && a.action(apidefaults.Namespace, types.KindClusterAlert, types.VerbRead, types.VerbList) == nil {
		resourceLevelPermit = true
	}

	// filter alerts by acks and teleport.internal 'permit' labels to determine whether the alert
	// was intended to be visible to the calling user.
	filtered := alerts[:0]
Outer:
	for _, alert := range alerts {
		// skip acknowledged alerts
		for _, ack := range acks {
			if ack.AlertID == alert.Metadata.Name {
				continue Outer
			}
		}

		// remaining checks in this loop are evaluating per-alert access, so short-circuit
		// if we are going off of resource-level permissions for this query.
		if resourceLevelPermit {
			filtered = append(filtered, alert)
			continue Outer
		}

		if alert.Metadata.Labels[types.AlertPermitAll] == "yes" {
			// alert may be shown to all authenticated users
			filtered = append(filtered, alert)
			continue Outer
		}

		// the verb-permit label permits users to view an alert if they hold
		// one of the specified <resource>:<verb> pairs (e.g. `node:list|token:create`
		// would be satisfied by either a user that can list nodes *or* create tokens).
	Verbs:
		for _, s := range strings.Split(alert.Metadata.Labels[types.AlertVerbPermit], "|") {
			rv := strings.Split(s, ":")
			if len(rv) != 2 {
				continue Verbs
			}

			if a.action(apidefaults.Namespace, rv[0], rv[1]) == nil {
				// user holds at least one of the resource:verb pairs specified by
				// the verb-permit label.
				filtered = append(filtered, alert)
				continue Outer
			}
		}
	}
	alerts = filtered

	if !query.WithSuperseded {
		// aggregate supersede directives and filter. we do this as a separate filter
		// step since we only obey supersede relationships within the set of
		// visible alerts (i.e. an alert that isn't visible cannot supersede an alert
		// that is visible).

		sups := make(map[string]types.AlertSeverity)

		for _, alert := range alerts {
			for _, id := range strings.Split(alert.Metadata.Labels[types.AlertSupersedes], ",") {
				if sups[id] < alert.Spec.Severity {
					sups[id] = alert.Spec.Severity
				}
			}
		}

		filtered = alerts[:0]
		for _, alert := range alerts {
			if sups[alert.Metadata.Name] > alert.Spec.Severity {
				continue
			}
			filtered = append(filtered, alert)
		}
		alerts = filtered
	}

	return alerts, nil
}

func (a *ServerWithRoles) UpsertClusterAlert(ctx context.Context, alert types.ClusterAlert) error {
	if err := a.action(apidefaults.Namespace, types.KindClusterAlert, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.UpsertClusterAlert(ctx, alert)
}

func (a *ServerWithRoles) CreateAlertAck(ctx context.Context, ack types.AlertAcknowledgement) error {
	// we treat alert acks as an extension of the cluster alert resource rather than its own resource
	if err := a.action(apidefaults.Namespace, types.KindClusterAlert, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.CreateAlertAck(ctx, ack)
}

func (a *ServerWithRoles) GetAlertAcks(ctx context.Context) ([]types.AlertAcknowledgement, error) {
	// we treat alert acks as an extension of the cluster alert resource rather than its own resource.
	if err := a.action(apidefaults.Namespace, types.KindClusterAlert, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetAlertAcks(ctx)
}

func (a *ServerWithRoles) ClearAlertAcks(ctx context.Context, req proto.ClearAlertAcksRequest) error {
	// we treat alert acks as an extension of the cluster alert resource rather than its own resource
	if err := a.action(apidefaults.Namespace, types.KindClusterAlert, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.ClearAlertAcks(ctx, req)
}

func (a *ServerWithRoles) UpsertNode(ctx context.Context, s types.Server) (*types.KeepAlive, error) {
	if err := a.action(s.GetNamespace(), types.KindNode, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.UpsertNode(ctx, s)
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
		if !a.hasBuiltinRole(types.RoleApp) && !a.hasBuiltinRole(types.RoleOkta) {
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
		if handle.HostID == "" {
			return trace.BadParameter("hostID is required for kubernetes keep alive")
		} else if serverName != handle.HostID {
			return trace.AccessDenied("access denied")
		}
		// Legacy kube proxy can heartbeat kube servers from the proxy itself so
		// we need to check if the host has the Kube or Proxy role.
		if !a.hasBuiltinRole(types.RoleKube, types.RoleProxy) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(apidefaults.Namespace, types.KindKubeServer, types.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	case constants.KeepAliveDatabaseService:
		if serverName != handle.Name {
			return trace.AccessDenied("access denied")
		}
		if !a.hasBuiltinRole(types.RoleDatabase) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(apidefaults.Namespace, types.KindDatabaseService, types.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unknown keep alive type %q", handle.Type)
	}

	return a.authServer.KeepAliveServer(ctx, handle)
}

// NewStream returns a new event stream (equivalent to NewWatcher, but with slightly different
// performance characteristics).
func (a *ServerWithRoles) NewStream(ctx context.Context, watch types.Watch) (stream.Stream[types.Event], error) {
	if err := a.authorizeWatchRequest(&watch); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.NewStream(ctx, watch)
}

// NewWatcher returns a new event watcher
func (a *ServerWithRoles) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	if err := a.authorizeWatchRequest(&watch); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.NewWatcher(ctx, watch)
}

// authorizeWatchRequest performs permission checks and filtering on incoming watch requests.
func (a *ServerWithRoles) authorizeWatchRequest(watch *types.Watch) error {
	if len(watch.Kinds) == 0 {
		return trace.AccessDenied("can't setup global watch")
	}

	validKinds := make([]types.WatchKind, 0, len(watch.Kinds))
	for _, kind := range watch.Kinds {
		err := a.hasWatchPermissionForKind(kind)
		if err != nil {
			if watch.AllowPartialSuccess {
				continue
			}
			return trace.Wrap(err)
		}

		validKinds = append(validKinds, kind)
	}

	if len(validKinds) == 0 {
		return trace.BadParameter("none of the requested kinds can be watched")
	}

	watch.Kinds = validKinds
	switch {
	case a.hasBuiltinRole(types.RoleProxy):
		watch.QueueSize = defaults.ProxyQueueSize
	case a.hasBuiltinRole(types.RoleNode):
		watch.QueueSize = defaults.NodeQueueSize
	}

	return nil
}

// hasWatchPermissionForKind checks the permissions for data of each kind.
// For watching, most kinds of data just need a Read permission, but some
// have more complicated logic.
func (a *ServerWithRoles) hasWatchPermissionForKind(kind types.WatchKind) error {
	verb := types.VerbRead
	switch kind.Kind {
	case types.KindCertAuthority:
		if !kind.LoadSecrets {
			verb = types.VerbReadNoSecrets
		}
	case types.KindAccessRequest:
		var filter types.AccessRequestFilter
		if err := filter.FromMap(kind.Filter); err != nil {
			return trace.Wrap(err)
		}

		// Users can watch their own access requests.
		if filter.User != "" && a.currentUserAction(filter.User) == nil {
			return nil
		}
	case types.KindWebSession:
		if !kind.LoadSecrets {
			verb = types.VerbReadNoSecrets
		}

		var filter types.WebSessionFilter
		if err := filter.FromMap(kind.Filter); err != nil {
			return trace.Wrap(err)
		}

		// TODO (Joerger): DELETE IN 17.0.0
		// Set LoadSecrets to true for requests from old proxies.
		if a.hasBuiltinRole(types.RoleProxy) {
			kind.LoadSecrets = true
		}

		// Allow reading Snowflake sessions to DB service.
		if kind.SubKind == types.KindSnowflakeSession && a.hasBuiltinRole(types.RoleDatabase) {
			return nil
		}

		// Users can watch their own web sessions without secrets.
		if filter.User != "" && !kind.LoadSecrets && a.currentUserAction(filter.User) == nil {
			return nil
		}
	case types.KindHeadlessAuthentication:
		var filter types.HeadlessAuthenticationFilter
		if err := filter.FromMap(kind.Filter); err != nil {
			return trace.Wrap(err)
		}

		// Users can only watch their own headless authentications, meaning we don't fallback to
		// the generalized verb-kind-action check below.
		if !authz.IsLocalUser(a.context) {
			return trace.AccessDenied("non-local user roles cannot watch headless authentications")
		} else if filter.Username == "" {
			return trace.AccessDenied("user cannot watch headless authentications without a filter for their username")
		} else if filter.Username != a.context.User.GetName() {
			return trace.AccessDenied("user %q cannot watch headless authentications of %q", a.context.User.GetName(), filter.Username)
		}

		return nil
	}
	return trace.Wrap(a.action(apidefaults.Namespace, kind.Kind, verb))
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
	node, err := a.authServer.GetNode(ctx, namespace, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.checkAccessToNode(node); err != nil {
		if trace.IsAccessDenied(err) {
			return nil, trace.NotFound("not found")
		}

		return nil, trace.Wrap(err)
	}

	return node, nil
}

// resourceAccess is used for different rbac results when filtering items
type resourceAccess struct {
	// accessChecker is used to check a user's access to resources. This can
	// be extended to include SearchAsRoles
	accessChecker resourceAccessChecker
	// baseAuthChecker is set when a user's auth context is extended during a
	// searchAsRoles request
	baseAuthChecker resourceAccessChecker
	// kindAccessMap is used to check errors for list/read verbs per kind
	kindAccessMap map[string]error
	// requestableMap is used to track if a resource matches a filter but is only
	// available after an access request. This map is of Resource.GetName()
	requestableMap map[string]struct{}
}

func (c *resourceAccess) checkAccess(resource types.ResourceWithLabels, filter services.MatchResourceFilter) (bool, error) {
	resourceKind := resource.GetKind()

	if canAccessErr := c.kindAccessMap[resourceKind]; canAccessErr != nil {
		// skip access denied error. It is expected that resources won't be available
		// to some users and we want to keep iterating until we've reached the request limit
		// of resources they have access to
		if trace.IsAccessDenied(canAccessErr) {
			return false, nil
		}
		return false, trace.Wrap(canAccessErr)
	}

	// Filter first and only check RBAC if there is a match to improve perf.
	match, err := services.MatchResourceByFilters(resource, filter, nil)
	if err != nil {
		log.WithFields(logrus.Fields{
			"resource_name": resource.GetName(),
			"resource_kind": resourceKind,
			"error":         err,
		}).
			Warn("Unable to determine access to resource, matching with filter failed")
		return false, nil
	}

	if !match {
		return false, nil
	}

	if resourceKind == types.KindSAMLIdPServiceProvider {
		return true, nil
	}

	// check access normally if base checker doesnt exist
	if c.baseAuthChecker == nil {
		if err := c.accessChecker.CanAccess(resource); err != nil {
			if trace.IsAccessDenied(err) {
				return false, nil
			}
			return false, trace.Wrap(err)
		}
		return true, nil
	}

	// baseAuthChecker exists if the current auth context has been extended for a includeRequestable request.
	// if true, we should check with the base auth checker first and if that returns false, check the extended context
	// so we know if a resource is being matched because they have access to it currently, or only to be added
	// to an access request
	if err := c.baseAuthChecker.CanAccess(resource); err != nil {
		if !trace.IsAccessDenied(err) {
			return false, trace.Wrap(err)
		}

		// user does not have access with their base context
		// check if they would have access with the extended context
		if err := c.accessChecker.CanAccess(resource); err != nil {
			if trace.IsAccessDenied(err) {
				return false, nil
			}
			return false, trace.Wrap(err)
		}
		c.requestableMap[resource.GetName()] = struct{}{}
	}

	return true, nil
}

// ListUnifiedResources returns a paginated list of unified resources filtered by user access.
func (a *ServerWithRoles) ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error) {
	filter := services.MatchResourceFilter{
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
		Kinds:          req.Kinds,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filter.PredicateExpression = expression
	}

	resourceAccess := &resourceAccess{
		// Populate kindAccessMap with any access errors the user has for each possible
		// resource kind. This allows the access check to occur a single time per resource
		// kind instead of once per matching resource.
		kindAccessMap: make(map[string]error, len(services.UnifiedResourceKinds)),
		// requestableMap is populated with resources that are being returned but can only
		// be accessed to the user via an access request
		requestableMap: make(map[string]struct{}),
	}

	for _, kind := range services.UnifiedResourceKinds {
		actionVerbs := []string{types.VerbList, types.VerbRead}
		if kind == types.KindNode {
			// We are checking list only for Nodes to keep backwards compatibility.
			// The read verb got added to GetNodes initially in:
			//   https://github.com/gravitational/teleport/pull/1209
			// but got removed shortly afterwards in:
			//   https://github.com/gravitational/teleport/pull/1224
			actionVerbs = []string{types.VerbList}
		}

		resourceAccess.kindAccessMap[kind] = a.action(apidefaults.Namespace, kind, actionVerbs...)
	}

	// Before doing any listing, verify that the user is allowed to list
	// at least one of the requested kinds. If no access is permitted, then
	// return an access denied error.
	requested := req.Kinds
	if len(req.Kinds) == 0 {
		requested = services.UnifiedResourceKinds
	}
	var rbacErrors int
	for _, kind := range requested {
		if err, ok := resourceAccess.kindAccessMap[kind]; ok && err != nil {
			rbacErrors++
		}
	}

	if rbacErrors == len(requested) {
		return nil, trace.AccessDenied("User does not have access to any of the requested kinds: %v", requested)
	}

	// Apply any requested additional search_as_roles and/or preview_as_roles
	// for the duration of the search.
	if req.UseSearchAsRoles || req.UsePreviewAsRoles || req.IncludeRequestable {
		extendedContext, err := a.authContextForSearch(ctx, &proto.ListResourcesRequest{
			UseSearchAsRoles:    req.UseSearchAsRoles || req.IncludeRequestable,
			UsePreviewAsRoles:   req.UsePreviewAsRoles,
			ResourceType:        types.KindUnifiedResource,
			Namespace:           apidefaults.Namespace,
			Labels:              req.Labels,
			PredicateExpression: req.PredicateExpression,
			SearchKeywords:      req.SearchKeywords,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		baseContext := a.context
		// If IncludeRequestable is true, we will create a baseChecker to
		// use during RBAC to determine if a resource would be available only after extending
		if req.IncludeRequestable {
			baseChecker, err := a.newResourceAccessChecker(types.KindUnifiedResource)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			resourceAccess.baseAuthChecker = baseChecker
		}

		a.context = *extendedContext
		defer func() {
			a.context = baseContext
		}()
	}

	checker, err := a.newResourceAccessChecker(types.KindUnifiedResource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceAccess.accessChecker = checker

	var (
		unifiedResources types.ResourcesWithLabels
		nextKey          string
	)
	if req.PinnedOnly {
		prefs, err := a.authServer.GetUserPreferences(ctx, a.context.User.GetName())
		if err != nil {
			return nil, trace.Wrap(err, "getting user preferences")
		}
		if len(prefs.ClusterPreferences.PinnedResources.ResourceIds) == 0 {
			return &proto.ListUnifiedResourcesResponse{}, nil
		}
		unifiedResources, err = a.authServer.UnifiedResourceCache.GetUnifiedResourcesByIDs(ctx, prefs.ClusterPreferences.PinnedResources.GetResourceIds(), func(resource types.ResourceWithLabels) (bool, error) {
			match, err := resourceAccess.checkAccess(resource, filter)
			return match, trace.Wrap(err)
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// we need to sort pinned resources manually because they are fetched in the order they were pinned
		if req.SortBy.Field != "" {
			if err := unifiedResources.SortByCustom(req.SortBy); err != nil {
				return nil, trace.Wrap(err, "sorting unified resources")
			}
		}
	} else {
		unifiedResources, nextKey, err = a.authServer.UnifiedResourceCache.IterateUnifiedResources(ctx, func(resource types.ResourceWithLabels) (bool, error) {
			match, err := resourceAccess.checkAccess(resource, filter)
			return match, trace.Wrap(err)
		}, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	paginatedResources, err := services.MakePaginatedResources(ctx, types.KindUnifiedResource, unifiedResources, resourceAccess.requestableMap)
	if err != nil {
		return nil, trace.Wrap(err, "making paginated unified resources")
	}

	if req.IncludeLogins {
		for _, r := range paginatedResources {
			if n := r.GetNode(); n != nil {
				logins, err := checker.GetAllowedLoginsForResource(n)
				if err != nil {
					log.WithError(err).WithField("resource", n.GetName()).Warn("Unable to determine logins for node")
					continue
				}
				r.Logins = logins
			} else if d := r.GetWindowsDesktop(); d != nil {
				logins, err := checker.GetAllowedLoginsForResource(d)
				if err != nil {
					log.WithError(err).WithField("resource", d.GetName()).Warn("Unable to determine logins for desktop")
					continue
				}
				r.Logins = logins
			}
		}
	}

	return &proto.ListUnifiedResourcesResponse{
		NextKey:   nextKey,
		Resources: paginatedResources,
	}, nil
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

	// Filter nodes to return the ones for the connected identity.
	filteredNodes := make([]types.Server, 0)
	startFilter := time.Now()
	for _, node := range nodes {
		if err := a.checkAccessToNode(node); err != nil {
			if trace.IsAccessDenied(err) {
				continue
			}

			return nil, trace.Wrap(err)
		}

		filteredNodes = append(filteredNodes, node)
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

// authContextForSearch returns an extended authz.Context which should be used
// when searching for resources that a user may be able to request access to,
// but does not already have access to.
// Extra roles are determined from the user's search_as_roles and
// preview_as_roles if [req] requested that each be used.
func (a *ServerWithRoles) authContextForSearch(ctx context.Context, req *proto.ListResourcesRequest) (*authz.Context, error) {
	var extraRoles []string
	if req.UseSearchAsRoles {
		extraRoles = append(extraRoles, a.context.Checker.GetAllowedSearchAsRoles()...)
	}
	if req.UsePreviewAsRoles {
		extraRoles = append(extraRoles, a.context.Checker.GetAllowedPreviewAsRoles()...)
	}
	if len(extraRoles) == 0 {
		// Return the current auth context unmodified.
		return &a.context, nil
	}

	clusterName, err := a.authServer.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a new auth context with the additional roles
	extendedContext, err := a.context.WithExtraRoles(a.authServer, clusterName.GetClusterName(), extraRoles)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return extendedContext, nil
}

var disableUnqualifiedLookups = os.Getenv("TELEPORT_UNSTABLE_DISABLE_UNQUALIFIED_LOOKUPS") == "yes"

// GetSSHTargets gets all servers that would match an equivalent ssh dial request. Note that this method
// returns all resources directly accessible to the user *and* all resources available via 'SearchAsRoles',
// which is what we want when handling things like ambiguous host errors and resource-based access requests,
// but may result in confusing behavior if it is used outside of those contexts.
func (a *ServerWithRoles) GetSSHTargets(ctx context.Context, req *proto.GetSSHTargetsRequest) (*proto.GetSSHTargetsResponse, error) {
	// try to detect case-insensitive routing setting, but default to false if we can't load
	// networking config (equivalent to proxy routing behavior).
	var caseInsensitiveRouting bool
	if cfg, err := a.authServer.GetReadOnlyClusterNetworkingConfig(ctx); err == nil {
		caseInsensitiveRouting = cfg.GetCaseInsensitiveRouting()
	}

	matcher, err := apiutils.NewSSHRouteMatcherFromConfig(apiutils.SSHRouteMatcherConfig{
		Host:                      req.Host,
		Port:                      req.Port,
		CaseInsensitive:           caseInsensitiveRouting,
		DisableUnqualifiedLookups: disableUnqualifiedLookups,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lreq := &proto.ListUnifiedResourcesRequest{
		Kinds:            []string{types.KindNode},
		SortBy:           types.SortBy{Field: types.ResourceMetadataName},
		UseSearchAsRoles: true,
	}
	var servers []*types.ServerV2
	for {
		// note that we're calling ServerWithRoles.ListUnifiedResources here rather than some internal method. This method
		// delegates all RBAC filtering to ListResources, and then performs additional filtering on top of that.
		lrsp, err := a.ListUnifiedResources(ctx, lreq)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, rsc := range lrsp.Resources {
			srv := rsc.GetNode()
			if srv == nil {
				log.Warnf("Unexpected resource type %T, expected *types.ServerV2 (skipping)", rsc)
				continue
			}

			if !matcher.RouteToServer(srv) {
				continue
			}

			servers = append(servers, srv)
		}

		if lrsp.NextKey == "" || len(lrsp.Resources) == 0 {
			break
		}

		lreq.StartKey = lrsp.NextKey
	}

	return &proto.GetSSHTargetsResponse{
		Servers: servers,
	}, nil
}

// ResolveSSHTarget gets a server that would match an equivalent ssh dial request.
func (a *ServerWithRoles) ResolveSSHTarget(ctx context.Context, req *proto.ResolveSSHTargetRequest) (*proto.ResolveSSHTargetResponse, error) {
	// try to detect case-insensitive routing setting, but default to false if we can't load
	// networking config (equivalent to proxy routing behavior).
	var routeToMostRecent bool
	if cfg, err := a.authServer.GetReadOnlyClusterNetworkingConfig(ctx); err == nil {
		routeToMostRecent = cfg.GetRoutingStrategy() == types.RoutingStrategy_MOST_RECENT
	}

	var servers []*types.ServerV2
	switch {
	case req.Host != "":
		if len(req.Labels) > 0 || req.PredicateExpression != "" || len(req.SearchKeywords) > 0 {
			log.Warn("ssh target resolution request contained both host and a resource matcher - ignoring resource matcher")
		}

		resp, err := a.GetSSHTargets(ctx, &proto.GetSSHTargetsRequest{
			Host: req.Host,
			Port: req.Port,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		servers = resp.Servers
	case len(req.Labels) > 0 || req.PredicateExpression != "" || len(req.SearchKeywords) > 0:
		lreq := &proto.ListUnifiedResourcesRequest{
			Kinds:               []string{types.KindNode},
			SortBy:              types.SortBy{Field: types.ResourceMetadataName},
			Labels:              req.Labels,
			PredicateExpression: req.PredicateExpression,
			SearchKeywords:      req.SearchKeywords,
		}
		for {
			// note that we're calling ServerWithRoles.ListUnifiedResources here rather than some internal method. This method
			// delegates all RBAC filtering to ListResources, and then performs additional filtering on top of that.
			lrsp, err := a.ListUnifiedResources(ctx, lreq)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			for _, rsc := range lrsp.Resources {
				srv := rsc.GetNode()
				if srv == nil {
					log.Warnf("Unexpected resource type %T, expected *types.ServerV2 (skipping)", rsc)
					continue
				}

				servers = append(servers, srv)
			}

			// If the routing strategy doesn't permit ambiguous matches, then abort
			// early if more than one server has been found already
			if !routeToMostRecent && len(servers) > 1 {
				break
			}

			if lrsp.NextKey == "" || len(lrsp.Resources) == 0 {
				break
			}

			lreq.StartKey = lrsp.NextKey

		}
	default:
		return nil, trace.BadParameter("request did not contain any host information or resource matcher")
	}

	switch len(servers) {
	case 1:
		return &proto.ResolveSSHTargetResponse{Server: servers[0]}, nil
	case 0:
		return nil, trace.NotFound("no matching hosts")
	default:
		if !routeToMostRecent {
			return nil, trace.Wrap(teleport.ErrNodeIsAmbiguous)
		}

		// Return the most recent version of the resource.
		server := slices.MaxFunc(servers, func(a, b *types.ServerV2) int {
			return a.Expiry().Compare(b.Expiry())
		})
		return &proto.ResolveSSHTargetResponse{Server: server}, nil
	}
}

// ListResources returns a paginated list of resources filtered by user access.
func (a *ServerWithRoles) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	// Check if auth server has a license for this resource type but only return an
	// error if the requester is not a builtin or remote server.
	// Builtin and remote server roles are allowed to list resources to avoid crashes
	// even if the license is missing.
	// Users with other roles will get an error if the license is missing so they
	// can request a license with the correct features.
	if err := enforceLicense(req.ResourceType); err != nil && !a.isLocalOrRemoteServerAction() {
		return nil, trace.Wrap(err)
	}

	// Apply any requested additional search_as_roles and/or preview_as_roles
	// for the duration of the search.
	if req.UseSearchAsRoles || req.UsePreviewAsRoles {
		extendedContext, err := a.authContextForSearch(ctx, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		baseContext := a.context
		a.context = *extendedContext
		defer func() {
			a.context = baseContext
		}()
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

	case types.KindDatabaseServer,
		types.KindDatabaseService,
		types.KindAppServer,
		types.KindKubeServer,
		types.KindWindowsDesktop,
		types.KindWindowsDesktopService,
		types.KindUserGroup,
		types.KindSAMLIdPServiceProvider:

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
		ResourceKind:   req.ResourceType,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filter.PredicateExpression = expression
	}

	req.Labels = nil
	req.SearchKeywords = nil
	req.PredicateExpression = ""

	// Increase the limit to one more than was requested so
	// that an additional page load is not needed to determine
	// the next key.
	req.Limit++

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
			if req.IncludeLogins {
				logins, err := resourceChecker.GetAllowedLoginsForResource(resource)
				if err != nil {
					log.WithError(err).WithField("resource", resource.GetName()).Warn("Unable to determine logins for resource")
				}

				resource = &types.EnrichedResource{
					ResourceWithLabels: resource,
					Logins:             logins,
				}
			}
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
	GetAllowedLoginsForResource(resource services.AccessCheckable) ([]string, error)
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
	state := services.AccessState{MFAVerified: true}
	switch rr := resource.(type) {
	case types.AppServer:
		return r.CheckAccess(rr.GetApp(), state)
	case types.KubeServer:
		return r.CheckAccess(rr.GetCluster(), state)
	case types.DatabaseServer:
		return r.CheckAccess(rr.GetDatabase(), state)
	case types.DatabaseService:
		return r.CheckAccess(rr, state)
	case types.Database:
		return r.CheckAccess(rr, state)
	case types.Server:
		return r.CheckAccess(rr, state)
	case types.WindowsDesktop:
		return r.CheckAccess(rr, state)
	case types.WindowsDesktopService:
		return r.CheckAccess(rr, state)
	case types.UserGroup:
		// Because usergroup only has ResourceWithLabels, it looks like this will match
		// everything. To get around this, we'll match on it last and then double check
		// that the kind is equal to usergroup. If it's not, we'll fall through and return
		// the bad parameter as expected.
		if rr.GetKind() == types.KindUserGroup {
			return r.CheckAccess(rr, state)
		}
	case types.SAMLIdPServiceProvider:
		return r.CheckAccess(rr, state)
	}

	return trace.BadParameter("could not check access to resource type %T", r)
}

// newResourceAccessChecker creates a resourceAccessChecker for the provided resource type
func (a *ServerWithRoles) newResourceAccessChecker(resource string) (resourceAccessChecker, error) {
	switch resource {
	case types.KindAppServer,
		types.KindDatabaseServer,
		types.KindDatabaseService,
		types.KindWindowsDesktop,
		types.KindWindowsDesktopService,
		types.KindNode,
		types.KindKubeServer,
		types.KindUserGroup,
		types.KindUnifiedResource,
		types.KindSAMLIdPServiceProvider:
		return &resourceChecker{AccessChecker: a.context.Checker}, nil
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

	// TODO(sshah) DELETE IN 17.0
	case types.KindAppOrSAMLIdPServiceProvider:
		appsAndServiceProviders, err := a.GetAppServersAndSAMLIdPServiceProviders(ctx, req.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		appsOrSPs := types.AppServersOrSAMLIdPServiceProviders(appsAndServiceProviders)

		if err := appsOrSPs.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}

		resources = appsOrSPs.AsResources()

	case types.KindSAMLIdPServiceProvider:
		// Only add SAMLIdPServiceProviders to the list if the caller has an enterprise license.
		if modules.GetModules().BuildType() == modules.BuildEnterprise {
			// Only attempt to list SAMLIdPServiceProviders if the caller has the permission to.
			if err := a.action(req.Namespace, types.KindSAMLIdPServiceProvider, types.VerbList); err == nil {
				var serviceProviders []types.SAMLIdPServiceProvider
				var startKey string
				for {
					sps, nextKey, err := a.authServer.ListSAMLIdPServiceProviders(ctx, int(req.Limit), startKey)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					serviceProviders = append(serviceProviders, sps...)
					if nextKey == "" {
						break
					}
					startKey = nextKey
				}
				sps := types.SAMLIdPServiceProviders(serviceProviders)
				if err := sps.SortByCustom(req.SortBy); err != nil {
					return nil, trace.Wrap(err)
				}
				resources = sps.AsResources()
			}
		}

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
		kubeServers, err := a.GetKubernetesServers(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Extract kube clusters into its own list.
		var clusters []types.KubeCluster
		for _, svc := range kubeServers {
			clusters = append(clusters, svc.GetCluster())
		}

		sortedClusters := types.KubeClusters(clusters)
		if err := sortedClusters.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = sortedClusters.AsResources()
	case types.KindKubeServer:
		kubeServers, err := a.GetKubernetesServers(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sortedServers := types.KubeServers(kubeServers)
		if err := sortedServers.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = sortedServers.AsResources()
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
	case types.KindUserGroup:
		var allUserGroups types.UserGroups
		userGroups, nextKey, err := a.ListUserGroups(ctx, int(req.Limit), "")
		for {
			if err != nil {
				return nil, trace.Wrap(err)
			}

			for _, ug := range userGroups {
				allUserGroups = append(allUserGroups, ug)
			}

			if nextKey == "" {
				break
			}

			userGroups, nextKey, err = a.ListUserGroups(ctx, int(req.Limit), nextKey)
		}

		if err := allUserGroups.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = allUserGroups.AsResources()

	default:
		return nil, trace.NotImplemented("resource type %q is not supported for listResourcesWithSort", req.ResourceType)
	}

	params := local.FakePaginateParams{
		ResourceType:   req.ResourceType,
		Limit:          req.Limit,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
		StartKey:       req.StartKey,
		EnrichResourceFn: func(r types.ResourceWithLabels) (types.ResourceWithLabels, error) {
			if req.IncludeLogins && (r.GetKind() == types.KindNode || r.GetKind() == types.KindWindowsDesktop) {
				resourceChecker, err := a.newResourceAccessChecker(req.ResourceType)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				logins, err := resourceChecker.GetAllowedLoginsForResource(r)
				if err != nil {
					log.WithError(err).WithField("resource", r.GetName()).Warn("Unable to determine logins for resource")
				}

				return &types.EnrichedResource{ResourceWithLabels: r, Logins: logins}, nil
			}

			return r, nil
		},
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		params.PredicateExpression = expression
	}

	// Apply request filters and get pagination info.
	resp, err := local.FakePaginate(resources, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func (a *ServerWithRoles) UpsertAuthServer(ctx context.Context, s types.Server) error {
	if err := a.action(apidefaults.Namespace, types.KindAuthServer, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertAuthServer(ctx, s)
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

func (a *ServerWithRoles) UpsertProxy(ctx context.Context, s types.Server) error {
	if err := a.action(apidefaults.Namespace, types.KindProxy, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertProxy(ctx, s)
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
func (a *ServerWithRoles) DeleteProxy(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindProxy, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteProxy(ctx, name)
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

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteToken(ctx, token)
}

func (a *ServerWithRoles) GetTokens(ctx context.Context) ([]types.ProvisionToken, error) {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetTokens(ctx)
}

func (a *ServerWithRoles) GetToken(ctx context.Context, token string) (types.ProvisionToken, error) {
	// The Proxy has permission to look up tokens by name in order to validate
	// attempts to use the node join script.
	if isProxy := a.hasBuiltinRole(types.RoleProxy); !isProxy {
		if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetToken(ctx, token)
}

func enforceEnterpriseJoinMethodCreation(token types.ProvisionToken) error {
	if modules.GetModules().BuildType() == modules.BuildEnterprise {
		return nil
	}

	v, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("unexpected token type %T", token)
	}

	switch v.Spec.JoinMethod {
	case types.JoinMethodGitHub:
		if v.Spec.GitHub != nil && v.Spec.GitHub.EnterpriseServerHost != "" {
			return trace.Wrap(
				ErrRequiresEnterprise,
				"github enterprise server joining",
			)
		}
	case types.JoinMethodSpacelift:
		return trace.Wrap(
			ErrRequiresEnterprise,
			"spacelift joining",
		)
	case types.JoinMethodTPM:
		return trace.Wrap(
			ErrRequiresEnterprise,
			"tpm joining",
		)
	}

	return nil
}

// emitTokenEvent is called by Create/Upsert Token in order to emit any relevant
// events.
func emitTokenEvent(ctx context.Context, e apievents.Emitter, token types.ProvisionToken,
) {
	userMetadata := authz.ClientUserMetadata(ctx)
	if err := e.EmitAuditEvent(ctx, &apievents.ProvisionTokenCreate{
		Metadata: apievents.Metadata{
			Type: events.ProvisionTokenCreateEvent,
			Code: events.ProvisionTokenCreateCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      token.GetSafeName(),
			Expires:   token.Expiry(),
			UpdatedBy: userMetadata.GetUser(),
		},
		UserMetadata: userMetadata,
		Roles:        token.GetRoles(),
		JoinMethod:   token.GetJoinMethod(),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit join token create event.")
	}
}

func (a *ServerWithRoles) UpsertToken(ctx context.Context, token types.ProvisionToken) error {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}

	if err := enforceEnterpriseJoinMethodCreation(token); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.UpsertToken(ctx, token); err != nil {
		return trace.Wrap(err)
	}

	emitTokenEvent(ctx, a.authServer.emitter, token)
	return nil
}

func (a *ServerWithRoles) CreateToken(ctx context.Context, token types.ProvisionToken) error {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}

	if err := enforceEnterpriseJoinMethodCreation(token); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.CreateToken(ctx, token); err != nil {
		return trace.Wrap(err)
	}

	emitTokenEvent(ctx, a.authServer.emitter, token)
	return nil
}

// ChangePassword updates users password based on the old password.
func (a *ServerWithRoles) ChangePassword(
	ctx context.Context,
	req *proto.ChangePasswordRequest,
) error {
	if err := a.currentUserAction(req.User); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.ChangePassword(ctx, req)
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
func (a *ServerWithRoles) ExtendWebSession(ctx context.Context, req authclient.WebSessionReq) (types.WebSession, error) {
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
	if err := r.c.action(apidefaults.Namespace, types.KindWebSession, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
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

// notImplementedMessage is the message to return for endpoints that are not
// implemented. This is due to how service interfaces are used with Teleport.
const notImplementedMessage = "not implemented: can only be called by auth locally"

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
	// delegate all access-control logic to ListAccessRequests to reduce duplicate maintenance.
	req := proto.ListAccessRequestsRequest{
		Filter: &filter,
	}
	var requests []types.AccessRequest
	for {
		rsp, err := a.ListAccessRequests(ctx, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, r := range rsp.AccessRequests {
			requests = append(requests, r)
		}

		req.StartKey = rsp.NextKey
		if req.StartKey == "" {
			break
		}
	}

	return requests, nil
}

// ListAccessRequests is an access request getter with pagination and sorting options.
func (a *ServerWithRoles) ListAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error) {
	// ensure filter is non-nil
	if req.Filter == nil {
		req.Filter = &types.AccessRequestFilter{}
	}
	// set the requesting user to be used in the filter match. This is only meant to be set here
	// and will be overwritten if set elsewhere
	req.Filter.Requester = a.context.User.GetName()

	if err := a.action(apidefaults.Namespace, types.KindAccessRequest, types.VerbList, types.VerbRead); err != nil {
		// Users are allowed to read + list their own access requests and
		// requests they are allowed to review, unless access was *explicitly*
		// denied. This means deny rules block the action but allow rules are
		// not required.
		if services.IsAccessExplicitlyDenied(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		// nil err means the user has explicit read + list permissions and can
		// get all requests.
		return a.authServer.ListAccessRequests(ctx, req)
	}

	// users can always view their own access requests unless the read or list
	// verbs are explicitly denied
	if req.Filter.User != "" && a.currentUserAction(req.Filter.User) == nil {
		return a.authServer.ListAccessRequests(ctx, req)
	}

	// user does not have read/list permissions and is not specifically requesting only
	// their own requests.  we therefore subselect the filter results to show only those requests
	// that the user *is* allowed to see (specifically, their own requests + requests that they
	// are allowed to review).
	identity := a.context.Identity.GetIdentity()
	checker, err := services.NewReviewPermissionChecker(
		ctx,
		a.authServer,
		a.context.User.GetName(),
		&identity,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// unless the user has allow directives for reviewing, they will never be able to
	// see any requests other than their own.
	if !checker.HasAllowDirectives() {
		if req.Filter.User != "" {
			// filter specifies a user, but it wasn't caught by the preceding exception,
			// so just return nothing.
			return &proto.ListAccessRequestsResponse{}, nil
		}
		req.Filter.User = a.context.User.GetName()
		return a.authServer.ListAccessRequests(ctx, req)
	}

	// aggregate all requests that the caller owns and/or is able to review. Note that we perform all filtering via the
	// passed-in matcher since the pagination key format varies by sort index and is an internal implementation detail
	// of the access request cache.
	rsp, err := a.authServer.ListMatchingAccessRequests(ctx, req, func(accessRequest *types.AccessRequestV3) (matches bool) {
		if accessRequest.GetUser() == a.context.User.GetName() {
			return true
		}

		canReview, err := checker.CanReviewRequest(accessRequest)
		if err != nil {
			log.Warnf("Failed to evaluate review permissions for user %q against request %q: %v", a.context.User.GetName(), accessRequest.GetName(), err)
			return false
		}

		return canReview
	})

	return rsp, trace.Wrap(err)
}

func (a *ServerWithRoles) CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindAccessRequest, types.VerbCreate); err != nil {
		// An exception is made to allow users to create *pending* access requests
		// for themselves unless the create verb was explicitly denied.
		if services.IsAccessExplicitlyDenied(err) || !req.GetState().IsPending() || a.currentUserAction(req.GetUser()) != nil {
			return nil, trace.Wrap(err)
		}
	}

	if !req.GetState().IsPending() {
		if authz.IsCurrentUser(a.context, req.GetUser()) {
			return nil, trace.AccessDenied("cannot create access request for self in non-pending state")
		}

		if len(a.context.Identity.GetIdentity().ActiveRequests) != 0 {
			return nil, trace.AccessDenied("cannot create access requests in non-pending state while using an access request")
		}

		log.Warnf("Use of resource-level access request 'create' permission by user %q to create non-pending access request for user %q. Creation of non-pending requests will be deprecated in future version of teleport. Consider migrating to a workflow with a separate approval step.", a.context.User.GetName(), req.GetUser())
	}

	if !authz.IsCurrentUser(a.context, req.GetUser()) {
		// If this request was authorized by allow rules and not ownership, require MFA.
		if err := a.context.AuthorizeAdminAction(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// ensure request ID is set server-side
	req.SetName(uuid.Must(uuid.NewV7()).String())

	resp, err := a.authServer.CreateAccessRequestV2(ctx, req, a.context.Identity.GetIdentity())
	return resp, trace.Wrap(err)
}

func (a *ServerWithRoles) SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) error {
	if err := a.action(apidefaults.Namespace, types.KindAccessRequest, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	if len(a.context.Identity.GetIdentity().ActiveRequests) != 0 {
		return trace.AccessDenied("cannot directly update the state of an access request while using an access request")
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	if params.State.IsPromoted() {
		return trace.BadParameter("state promoted can be only set when promoting to access list")
	}

	// load the request to verify additional access-control limits
	reqs, err := a.GetAccessRequests(ctx, types.AccessRequestFilter{
		ID: params.RequestID,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if len(reqs) < 1 {
		return trace.NotFound("cannot set state of access request %q (not found)", params.RequestID)
	}

	if authz.IsCurrentUser(a.context, reqs[0].GetUser()) {
		return trace.AccessDenied("directly updating the state of your own access requests is not permitted")
	}

	return a.authServer.SetAccessRequestState(ctx, params)
}

// AuthorizeAccessReviewRequest checks if the current user is allowed to submit the given access review request.
func AuthorizeAccessReviewRequest(context authz.Context, params types.AccessReviewSubmission) error {
	// review author must match calling user, except in the case of the builtin admin role. we make this
	// exception in order to allow for convenient testing with local tctl connections.
	if !authz.HasBuiltinRole(context, string(types.RoleAdmin)) {
		if params.Review.Author != context.User.GetName() {
			return trace.AccessDenied("user %q cannot submit reviews on behalf of %q", context.User.GetName(), params.Review.Author)
		}

		// MaybeCanReviewRequests returns false positives, but it will tell us
		// if the user definitely can't review requests, which saves a lot of work.
		if !context.Checker.MaybeCanReviewRequests() {
			return trace.AccessDenied("user %q cannot submit reviews", context.User.GetName())
		}
	}

	return nil
}

func (a *ServerWithRoles) SubmitAccessReview(ctx context.Context, submission types.AccessReviewSubmission) (types.AccessRequest, error) {
	// Prevent users from submitting access reviews with the "promoted" state.
	// Promotion is only allowed by SubmitAccessReviewAllowPromotion API in the Enterprise module.
	if submission.Review.ProposedState.IsPromoted() {
		return nil, trace.BadParameter("state promoted can be only set when promoting to access list")
	}

	// review author defaults to username of caller.
	if submission.Review.Author == "" {
		submission.Review.Author = a.context.User.GetName()
	}

	// Check if the current user is allowed to submit the given access review request.
	if err := AuthorizeAccessReviewRequest(a.context, submission); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	// note that we haven't actually enforced any access-control other than requiring
	// the author field to match the calling user.  fine-grained permissions are evaluated
	// under optimistic locking at the level of the backend service.  the correctness of the
	// author field is all that needs to be enforced at this level.

	identity := a.context.Identity.GetIdentity()
	return a.authServer.submitAccessReview(ctx, submission, &identity)
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
	case types.KindAccessRequest, types.KindAccessList:
		// for backwards compatibility, we allow list/read against kinds to also grant list/read for
		// access request related plugin data.
		if a.action(apidefaults.Namespace, filter.Kind, types.VerbList) != nil {
			if err := a.action(apidefaults.Namespace, types.KindAccessPluginData, types.VerbList); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		if a.action(apidefaults.Namespace, filter.Kind, types.VerbRead) != nil {
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
	case types.KindAccessRequest, types.KindAccessList:
		// for backwards compatibility, we allow update against access requests to also grant update for
		// access request related plugin data.
		if a.action(apidefaults.Namespace, params.Kind, types.VerbUpdate) != nil {
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
	return a.authServer.Ping(ctx)
}

func (a *ServerWithRoles) DeleteAccessRequest(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindAccessRequest, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteAccessRequest(ctx, name)
}

// GetUsers returns all existing users
// TODO(tross): DELETE IN 17.0.0
// Deprecated: use [usersv1.Service.ListUsers] instead.
func (a *ServerWithRoles) GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error) {
	if withSecrets {
		// TODO(fspmarshall): replace admin requirement with VerbReadWithSecrets once we've
		// migrated to that model.
		if !a.hasBuiltinRole(types.RoleAdmin) {
			err := trace.AccessDenied("user %q requested access to all users with secrets", a.context.User.GetName())
			log.Warning(err)
			if err := a.authServer.emitter.EmitAuditEvent(ctx, &apievents.UserLogin{
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

	users, err := a.authServer.GetUsers(ctx, withSecrets)
	return users, trace.Wrap(err)
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

// desiredAccessInfo inspects the current request to determine which access
// information (roles, traits, and allowed resource IDs) the requesting user
// wants to be present on the resulting certificate. This does not attempt to
// determine if the user is allowed to assume the returned roles. Will set
// `req.AccessRequests` and potentially shorten `req.Expires` based on the
// access request expirations.
func (a *ServerWithRoles) desiredAccessInfo(ctx context.Context, req *proto.UserCertsRequest, user types.User) (*services.AccessInfo, error) {
	if req.Username != a.context.User.GetName() {
		if isRoleImpersonation(*req) {
			err := trace.AccessDenied("User %v tried to issue a cert for %v and added role requests. This is not supported.", a.context.User.GetName(), req.Username)
			log.WithError(err).Warn()
			return nil, err
		}
		if len(req.AccessRequests) > 0 {
			err := trace.AccessDenied("User %v tried to issue a cert for %v and added access requests. This is not supported.", a.context.User.GetName(), req.Username)
			log.WithError(err).Warn()
			return nil, err
		}
		return a.desiredAccessInfoForImpersonation(user)
	}
	if isRoleImpersonation(*req) {
		if len(req.AccessRequests) > 0 {
			err := trace.AccessDenied("User %v tried to issue a cert with both role and access requests. This is not supported.", a.context.User.GetName())
			log.WithError(err).Warn()
			return nil, err
		}
		return a.desiredAccessInfoForRoleRequest(req, user.GetTraits())
	}
	return a.desiredAccessInfoForUser(ctx, req, user)
}

// desiredAccessInfoForImpersonation returns the desired AccessInfo for an
// impersonation request.
func (a *ServerWithRoles) desiredAccessInfoForImpersonation(user types.User) (*services.AccessInfo, error) {
	return &services.AccessInfo{
		Roles:  user.GetRoles(),
		Traits: user.GetTraits(),
	}, nil
}

// desiredAccessInfoForRoleRequest returns the desired roles for a role request.
func (a *ServerWithRoles) desiredAccessInfoForRoleRequest(req *proto.UserCertsRequest, traits wrappers.Traits) (*services.AccessInfo, error) {
	// If UseRoleRequests is set, make sure we don't return unusable certs: an
	// identity without roles can't be parsed.
	if len(req.RoleRequests) == 0 {
		return nil, trace.BadParameter("at least one role request is required")
	}

	// If role requests are provided, attempt to satisfy them instead of
	// pulling them directly from the logged in identity. Role requests
	// are intended to reduce allowed permissions so we'll accept them
	// as-is for now (and ensure the user is allowed to assume them
	// later).
	//
	// Traits are copied across from the impersonating user so that role
	// variables within the impersonated role behave as expected.
	return &services.AccessInfo{
		Roles:  req.RoleRequests,
		Traits: traits,
	}, nil
}

// desiredAccessInfoForUser returns the desired AccessInfo
// cert request which may contain access requests.
func (a *ServerWithRoles) desiredAccessInfoForUser(ctx context.Context, req *proto.UserCertsRequest, user types.User) (*services.AccessInfo, error) {
	currentIdentity := a.context.Identity.GetIdentity()

	// Start with the base AccessInfo for current logged-in identity, before
	// considering new or dropped access requests. This will include roles from
	// currently assumed role access requests, and allowed resources from
	// currently assumed resource access requests.
	accessInfo, err := services.AccessInfoFromLocalTLSIdentity(currentIdentity, a.authServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if dropAnyRequests := len(req.DropAccessRequests) > 0; dropAnyRequests {
		// Reset to the base roles and traits stored in the backend user,
		// currently active requests (not being dropped) and new access requests
		// will be filled in below.
		accessInfo = services.AccessInfoFromUser(user)

		// Check for ["*"] as special case to drop all requests.
		if len(req.DropAccessRequests) == 1 && req.DropAccessRequests[0] == "*" {
			// Removing all access requests from cert, return early with base roles
			// for the user. Make sure to clear req.AccessRequests, or these will be
			// encoded in the cert.
			req.AccessRequests = nil
			return accessInfo, nil
		}
	}

	// Build a list of access request IDs which we need to fetch and apply to
	// the new cert request.
	var finalRequestIDs []string
	for _, requestList := range [][]string{currentIdentity.ActiveRequests, req.AccessRequests} {
		for _, reqID := range requestList {
			if !slices.Contains(req.DropAccessRequests, reqID) {
				finalRequestIDs = append(finalRequestIDs, reqID)
			}
		}
	}
	finalRequestIDs = apiutils.Deduplicate(finalRequestIDs)

	// Replace req.AccessRequests with final filtered values, these will be
	// encoded into the cert.
	req.AccessRequests = finalRequestIDs

	// Reset the resource restrictions, we are going to iterate all access
	// requests below, if there are any resource requests this will be set.
	accessInfo.AllowedResourceIDs = nil

	for _, reqID := range finalRequestIDs {
		// Fetch and validate the access request for this user.
		accessRequest, err := a.authServer.getValidatedAccessRequest(ctx, currentIdentity, req.Username, reqID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Cannot generate a cert that would outlive the access request.
		if accessRequest.GetAccessExpiry().Before(req.Expires) {
			req.Expires = accessRequest.GetAccessExpiry()
		}

		// Merge requested roles from all access requests.
		accessInfo.Roles = append(accessInfo.Roles, accessRequest.GetRoles()...)

		// Make sure only 1 access request includes resource restrictions. There
		// is not a logical way to merge resource access requests, e.g. if a
		// user requested "node1" with role "user" and "node2" with role "admin".
		if requestedResourceIDs := accessRequest.GetRequestedResourceIDs(); len(requestedResourceIDs) > 0 {
			if len(accessInfo.AllowedResourceIDs) > 0 {
				return nil, trace.BadParameter("cannot generate certificate with multiple resource access requests")
			}
			accessInfo.AllowedResourceIDs = requestedResourceIDs
		}
	}
	accessInfo.Roles = apiutils.Deduplicate(accessInfo.Roles)

	return accessInfo, nil
}

// GenerateUserCerts generates users certificates
func (a *ServerWithRoles) GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
	identity := a.context.Identity.GetIdentity()
	return a.generateUserCerts(
		ctx, req,
		certRequestDeviceExtensions(identity.DeviceExtensions),
	)
}

func isRoleImpersonation(req proto.UserCertsRequest) bool {
	// For now, either req.UseRoleRequests or len(req.RoleRequests) > 0
	// indicates that role impersonation is being used.
	// In Teleport 14.0.0, make using len(req.RoleRequests) > 0 without
	// req.UseRoleRequests an error. This will simplify logic throughout
	// by having a clear indicator of whether role impersonation is in use
	// and make this helper redundant.
	return req.UseRoleRequests || len(req.RoleRequests) > 0
}

// getBotName returns the name of the bot embedded in the user metadata, if any.
// For non-bot users, returns "".
func getBotName(user types.User) string {
	name, ok := user.GetLabel(types.BotLabel)
	if ok {
		return name
	}

	return ""
}

func (a *ServerWithRoles) generateUserCerts(ctx context.Context, req proto.UserCertsRequest, opts ...certRequestOption) (*proto.Certs, error) {
	// Device trust: authorize device before issuing certificates.
	readOnlyAuthPref, err := a.authServer.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.verifyUserDeviceForCertIssuance(req.Usage, readOnlyAuthPref.GetDeviceTrust()); err != nil {
		return nil, trace.Wrap(err)
	}

	var verifiedMFADeviceID string
	if req.MFAResponse != nil {
		requiredExt := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION}
		mfaData, err := a.authServer.ValidateMFAAuthResponse(ctx, req.GetMFAResponse(), req.Username, requiredExt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		verifiedMFADeviceID = mfaData.Device.Id
	}

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
		if isRoleImpersonation(req) {
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
	user, err := a.authServer.GetUser(ctx, req.Username, false)
	if err != nil {
		log.WithError(err).Debugf("Could not impersonate user %v. The user could not be fetched from local store.", req.Username)
		return nil, trace.AccessDenied("access denied")
	}

	// Do not allow SSO users to be impersonated.
	if req.Username != a.context.User.GetName() && user.GetUserType() == types.UserTypeSSO {
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
			return nil, trace.Wrap(client.ErrClientCredentialsHaveExpired)
		}

		if identity.Renewable || isRoleImpersonation(req) {
			// Bot self-renewal or role impersonation can request certs with an
			// expiry up to the global maximum allowed value.
			if maxTime := a.authServer.GetClock().Now().Add(defaults.MaxRenewableCertTTL); req.Expires.After(maxTime) {
				req.Expires = maxTime
			}
		} else if isLocalProxyCertReq(&req) {
			// If requested certificate is for headless Kubernetes access of local proxy it is limited by max session ttl
			// or mfa_verification_interval or req.Expires.

			// Calculate the expiration time.
			roleSet, err := services.FetchRoles(user.GetRoles(), a, user.GetTraits())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// [roleSet.AdjustMFAVerificationInterval] will reduce the adjusted sessionTTL if any of the roles requires
			// MFA tap and `mfa_verification_interval` is set and lower than [roleSet.AdjustSessionTTL].
			sessionTTL := roleSet.AdjustMFAVerificationInterval(
				roleSet.AdjustSessionTTL(readOnlyAuthPref.GetDefaultSessionTTL().Duration()),
				readOnlyAuthPref.GetRequireMFAType().IsSessionMFARequired())
			adjustedSessionExpires := a.authServer.GetClock().Now().UTC().Add(sessionTTL)
			if req.Expires.After(adjustedSessionExpires) {
				req.Expires = adjustedSessionExpires
			}
		} else if req.Expires.After(sessionExpires) {
			// Standard user impersonation has an expiry limited to the expiry
			// of the current session. This prevents a user renewing their
			// own certificates indefinitely to avoid re-authenticating.
			req.Expires = sessionExpires
		}
	}

	// If the user is not a user cert renewal (impersonation, etc.), this is an
	// admin action and requires MFA.
	if req.Username != a.context.User.GetName() {
		// Admin action MFA is not used to create mfa verified certs.
		if err := a.context.AuthorizeAdminAction(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// we're going to extend the roles list based on the access requests, so we
	// ensure that all the current requests are added to the new certificate
	// (and are checked again)
	req.AccessRequests = append(req.AccessRequests, a.context.Identity.GetIdentity().ActiveRequests...)
	if req.Username != a.context.User.GetName() && len(req.AccessRequests) > 0 {
		return nil, trace.AccessDenied("user %q requested cert for %q and included access requests, this is not supported.", a.context.User.GetName(), req.Username)
	}

	accessInfo, err := a.desiredAccessInfo(ctx, &req, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parsedRoles, err := services.FetchRoleList(accessInfo.Roles, a.authServer, accessInfo.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// add implicit roles to the set and build a checker
	roleSet := services.NewRoleSet(parsedRoles...)
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker := services.NewAccessCheckerWithRoleSet(accessInfo, clusterName.GetClusterName(), roleSet)

	switch {
	case a.hasBuiltinRole(types.RoleAdmin):
		// builtin admins can impersonate anyone
		// this is required for local tctl commands to work
	case req.Username == a.context.User.GetName():
		// users can impersonate themselves, but role impersonation requests
		// must be checked.
		if isRoleImpersonation(req) {
			// Note: CheckImpersonateRoles() checks against the _stored_
			// impersonate roles for the user rather than the set available
			// to the current identity. If not explicitly denied (as above),
			// this could allow a role-impersonated certificate to request new
			// certificates with alternate RoleRequests.
			err = a.context.Checker.CheckImpersonateRoles(a.context.User, parsedRoles)
			if err != nil {
				log.Warning(err)
				err := trace.AccessDenied("user %q has requested role impersonation for %q", a.context.User.GetName(), accessInfo.Roles)
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
			err := trace.AccessDenied("user %q has requested to generate certs for %q.", a.context.User.GetName(), accessInfo.Roles)
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

	appSessionID := req.RouteToApp.SessionID
	if req.RouteToApp.Name != "" {
		// Create a new app session using the same cert request. The user certs
		// generated below will be linked to this session by the session ID.
		if req.RouteToApp.SessionID == "" {
			ws, err := a.authServer.CreateAppSessionFromReq(ctx, NewAppSessionRequest{
				NewWebSessionRequest: NewWebSessionRequest{
					User:           req.Username,
					LoginIP:        a.context.Identity.GetIdentity().LoginIP,
					SessionTTL:     req.Expires.Sub(a.authServer.GetClock().Now()),
					Traits:         accessInfo.Traits,
					Roles:          accessInfo.Roles,
					AccessRequests: req.AccessRequests,
					// App sessions created through generateUserCerts are securely contained
					// to the Proxy and Auth roles, and thus should pass hardware key requirements
					// through the "web_session" attestation. User's will be required to provide
					// MFA instead.
					AttestWebSession: true,
				},
				PublicAddr:        req.RouteToApp.PublicAddr,
				ClusterName:       req.RouteToApp.ClusterName,
				AWSRoleARN:        req.RouteToApp.AWSRoleARN,
				AzureIdentity:     req.RouteToApp.AzureIdentity,
				GCPServiceAccount: req.RouteToApp.GCPServiceAccount,
				MFAVerified:       verifiedMFADeviceID,
				DeviceExtensions:  DeviceExtensions(a.context.Identity.GetIdentity().DeviceExtensions),
				AppName:           req.RouteToApp.Name,
				AppURI:            req.RouteToApp.URI,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			appSessionID = ws.GetName()
		} else {
			// TODO (Joerger): DELETE IN 17.0.0
			// Old clients will pass a session ID of an existing App session instead of a
			// single GenerateUserCerts call. This is allowed, but ensure new clients cannot
			// generate certs for MFA verified app sessions without an additional MFA check.
			ws, err := a.GetAppSession(ctx, types.GetAppSessionRequest{
				SessionID: req.RouteToApp.SessionID,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			// If the app session is MFA verified and this request is not MFA verified, deny the request.
			x509Cert, err := tlsca.ParseCertificatePEM(ws.GetTLSCert())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			wsIdentity, err := tlsca.FromSubject(x509Cert.Subject, x509Cert.NotAfter)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if wsIdentity.IsMFAVerified() && verifiedMFADeviceID == "" {
				return nil, trace.BadParameter("mfa verification required to sign app certs for mfa-verified session")
			}
		}
	}

	// Generate certificate, note that the roles TTL will be ignored because
	// the request is coming from "tctl auth sign" itself.
	certReq := certRequest{
		mfaVerified:       verifiedMFADeviceID,
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
		dbRoles:           req.RouteToDatabase.Roles,
		appSessionID:      appSessionID,
		appName:           req.RouteToApp.Name,
		appPublicAddr:     req.RouteToApp.PublicAddr,
		appURI:            req.RouteToApp.URI,
		appClusterName:    req.RouteToApp.ClusterName,
		awsRoleARN:        req.RouteToApp.AWSRoleARN,
		azureIdentity:     req.RouteToApp.AzureIdentity,
		gcpServiceAccount: req.RouteToApp.GCPServiceAccount,
		checker:           checker,
		// Copy IP from current identity to the generated certificate, if present,
		// to avoid generateUserCerts() being used to drop IP pinning in the new certificates.
		loginIP:                a.context.Identity.GetIdentity().LoginIP,
		traits:                 accessInfo.Traits,
		activeRequests:         req.AccessRequests,
		connectionDiagnosticID: req.ConnectionDiagnosticID,
		attestationStatement:   keys.AttestationStatementFromProto(req.AttestationStatement),
		botName:                getBotName(user),

		// Always pass through a bot instance ID if available. Legacy bots
		// joining without an instance ID may have one generated when
		// `updateBotInstance()` is called below, and this (empty) value will be
		// overridden.
		botInstanceID: a.context.Identity.GetIdentity().BotInstanceID,
		// Propagate any join attributes from the current identity to the new
		// identity.
		joinAttributes: a.context.Identity.GetIdentity().JoinAttributes,
	}

	if user.GetName() != a.context.User.GetName() {
		certReq.impersonator = a.context.User.GetName()
	} else if isRoleImpersonation(req) {
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
		!isRoleImpersonation(req) &&
		!certReq.disallowReissue {
		certReq.renewable = true
	}

	// If the cert is renewable, process any bot instance updates (generation
	// counter, auth records, etc). `updateBotInstance()` may modify certain
	// `certReq` attributes (generation, botInstanceID).
	if certReq.renewable {
		currentIdentityGeneration := a.context.Identity.GetIdentity().Generation

		// If we're handling a renewal for a bot, we want to return the
		// Host CAs as well as the User CAs.
		if certReq.botName != "" {
			certReq.includeHostCA = true
		}

		// Update the bot instance based on this authentication. This may create
		// a new bot instance record if the identity is missing an instance ID.
		if err := a.authServer.updateBotInstance(
			ctx, &certReq, user.GetName(), certReq.botName,
			certReq.botInstanceID, nil, int32(currentIdentityGeneration),
		); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	certs, err := a.authServer.generateUserCert(ctx, certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certs, nil
}

// GetAccessRequestAllowedPromotions returns a list of roles that the user can
// promote to, based on the given access requests.
func (a *ServerWithRoles) GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	promotions, err := a.authServer.GetAccessRequestAllowedPromotions(ctx, req)
	return promotions, trace.Wrap(err)
}

// verifyUserDeviceForCertIssuance verifies if the user device is a trusted
// device, in accordance to the certificate usage and the cluster's DeviceTrust
// settings. It's meant to be called before issuing new user certificates.
// Each Node (or access server) verifies the device independently, so this check
// is not paramount to the access system itself, but it stops bad attempts from
// progressing further and provides better feedback than other protocol-specific
// failures.
func (a *ServerWithRoles) verifyUserDeviceForCertIssuance(usage proto.UserCertsRequest_CertUsage, dt *types.DeviceTrust) error {
	// Ignore App or WindowsDeskop requests, they do not support device trust.
	if usage == proto.UserCertsRequest_App || usage == proto.UserCertsRequest_WindowsDesktop {
		return nil
	}

	identity := a.context.Identity.GetIdentity()
	return trace.Wrap(dtauthz.VerifyTLSUser(dt, identity))
}

func (a *ServerWithRoles) CreateResetPasswordToken(ctx context.Context, req authclient.CreateUserTokenRequest) (types.UserToken, error) {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if a.hasBuiltinRole(types.RoleOkta) {
		return nil, trace.AccessDenied("access denied")
	}

	// Allow reused MFA responses to allow creating a reset token after creating a user.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.CreateResetPasswordToken(ctx, req)
}

func (a *ServerWithRoles) GetResetPasswordToken(ctx context.Context, tokenID string) (types.UserToken, error) {
	// tokens are their own authz mechanism, no need to double check
	return a.authServer.getResetPasswordToken(ctx, tokenID)
}

// ChangeUserAuthentication is implemented by AuthService.ChangeUserAuthentication.
func (a *ServerWithRoles) ChangeUserAuthentication(ctx context.Context, req *proto.ChangeUserAuthenticationRequest) (*proto.ChangeUserAuthenticationResponse, error) {
	// Token is it's own authentication, no need to double check.
	resp, err := a.authServer.ChangeUserAuthentication(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We use the presence of a WebAuthn response, along with the absence of a
	// password, as a proxy to determine that a passwordless registration took
	// place, as it is not possible to infer that just from the WebAuthn response.
	isPasswordless := req.NewMFARegisterResponse != nil && len(req.NewPassword) == 0
	if isPasswordless && modules.GetModules().Features().Cloud {
		if err := a.trySettingConnectorNameToPasswordless(ctx); err != nil {
			log.WithError(err).Error("Failed to set passwordless as connector name.")
		}
	}

	return resp, nil
}

// trySettingConnectorNameToPasswordless sets cluster_auth_preference connectorName to `passwordless` when the first cloud user chooses passwordless as the authentication method.
// This simplifies UX for cloud users, as they will not need to select a passwordless connector when logging in.
func (a *ServerWithRoles) trySettingConnectorNameToPasswordless(ctx context.Context) error {
	users, err := a.authServer.GetUsers(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}

	// Only set the connector name on the first user registration.
	if !hasOneNonPresetUser(users) {
		return nil
	}

	authPreference, err := a.authServer.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Don't overwrite an existing connector name.
	if connector := authPreference.GetConnectorName(); connector != "" && connector != constants.LocalConnector {
		return nil
	}

	authPreference.SetConnectorName(constants.PasswordlessConnector)
	_, err = a.authServer.UpdateAuthPreference(ctx, authPreference)
	return trace.Wrap(err)
}

// hasOneNonPresetUser returns true only if there is exactly one non-preset user in the provided list of users.
// This method always compare with the original preset users, and take into account that some preset users may
// have been removed.
func hasOneNonPresetUser(users []types.User) bool {
	presets := getPresetUsers()

	// Exit early if the number of users is greater than the number of presets + 1.
	if len(users) > len(presets)+1 {
		return false
	}

	// We can't simply compare the number of existing users to presets because presets may have been deleted.
	// Hence, we need to check each user to determine if they are a preset user.
	presetMap := make(map[string]struct{}, len(presets))
	for _, preset := range presets {
		presetMap[preset.GetName()] = struct{}{}
	}

	qtyNonPreset := 0
	for _, user := range users {
		if _, isPreset := presetMap[user.GetName()]; !isPreset {
			qtyNonPreset++
		}
		if qtyNonPreset > 1 {
			return false
		}
	}

	return qtyNonPreset == 1
}

// UpdateUser updates an existing user in a backend.
// Captures the auth user who modified the user record.
// TODO(tross): DELETE IN 17.0.0
// Deprecated: use [usersv1.Service.UpdateUser] instead.
func (a *ServerWithRoles) UpdateUser(ctx context.Context, user types.User) (types.User, error) {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := okta.CheckOrigin(&a.context, user); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkOktaUserAccess(ctx, &a.context, a.authServer, user.GetName(), types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := a.authServer.UpdateUser(ctx, user)
	return updated, trace.Wrap(err)
}

// UpsertUser create or updates an existing user.
// TODO(tross): DELETE IN 17.0.0
// Deprecated: use [usersv1.Service.UpdateUser] instead.
func (a *ServerWithRoles) UpsertUser(ctx context.Context, u types.User) (types.User, error) {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := okta.CheckOrigin(&a.context, u); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkOktaUserAccess(ctx, &a.context, a.authServer, u.GetName(), types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	createdBy := u.GetCreatedBy()
	if createdBy.IsEmpty() {
		u.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{Name: a.context.User.GetName()},
		})
	}
	user, err := a.authServer.UpsertUser(ctx, u)
	return user, trace.Wrap(err)
}

// CompareAndSwapUser updates an existing user in a backend, but fails if the
// backend's value does not match the expected value.
// Captures the auth user who modified the user record.
func (a *ServerWithRoles) CompareAndSwapUser(ctx context.Context, new, existing types.User) error {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	if err := okta.CheckOrigin(&a.context, new); err != nil {
		return trace.Wrap(err)
	}

	// Checking the `existing` origin should be enough to assert that okta has
	// write access to the user, because if the backend record says something
	// different then the `CompareAndSwap()` will fail anyway, and this way we
	// save ourselves a backend user lookup.

	if err := okta.CheckAccess(&a.context, existing, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.CompareAndSwapUser(ctx, new, existing)
}

// UpsertOIDCConnector creates or updates an OIDC connector.
func (a *ServerWithRoles) UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if !modules.GetModules().Features().GetEntitlement(entitlements.OIDC).Enabled {
		// TODO(zmb3): ideally we would wrap ErrRequiresEnterprise here, but
		// we can't currently propagate wrapped errors across the gRPC boundary,
		// and we want tctl to display a clean user-facing message in this case
		return nil, trace.AccessDenied("OIDC is only available in Teleport Enterprise")
	}

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := a.authServer.UpsertOIDCConnector(ctx, connector)
	return upserted, trace.Wrap(err)
}

// UpdateOIDCConnector updates an existing OIDC connector.
func (a *ServerWithRoles) UpdateOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if !modules.GetModules().Features().GetEntitlement(entitlements.OIDC).Enabled {
		// TODO(zmb3): ideally we would wrap ErrRequiresEnterprise here, but
		// we can't currently propagate wrapped errors across the gRPC boundary,
		// and we want tctl to display a clean user-facing message in this case
		return nil, trace.AccessDenied("OIDC is only available in Teleport Enterprise")
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := a.authServer.UpdateOIDCConnector(ctx, connector)
	return updated, trace.Wrap(err)
}

// CreateOIDCConnector creates a new OIDC connector.
func (a *ServerWithRoles) CreateOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if !modules.GetModules().Features().GetEntitlement(entitlements.OIDC).Enabled {
		// TODO(zmb3): ideally we would wrap ErrRequiresEnterprise here, but
		// we can't currently propagate wrapped errors across the gRPC boundary,
		// and we want tctl to display a clean user-facing message in this case
		return nil, trace.AccessDenied("OIDC is only available in Teleport Enterprise")
	}

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	creted, err := a.authServer.CreateOIDCConnector(ctx, connector)
	return creted, trace.Wrap(err)
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
	return a.authServer.GetOIDCConnector(ctx, id, withSecrets)
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
	return a.authServer.GetOIDCConnectors(ctx, withSecrets)
}

func (a *ServerWithRoles) CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.OIDC).Enabled {
		// TODO(zmb3): ideally we would wrap ErrRequiresEnterprise here, but
		// we can't currently propagate wrapped errors across the gRPC boundary,
		// and we want tctl to display a clean user-facing message in this case
		return nil, trace.AccessDenied("OIDC is only available in Teleport Enterprise")
	}

	if err := a.action(apidefaults.Namespace, types.KindOIDCRequest, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	// require additional permissions for executing SSO test flow.
	if req.SSOTestFlow {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbCreate); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Only the Proxy service can create web sessions via OIDC connector.
	if req.CreateWebSession && !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}

	oidcReq, err := a.authServer.CreateOIDCAuthRequest(ctx, req)
	if err != nil {
		if trace.IsNotFound(err) {
			// This flow is triggered via an unauthenticated endpoint, so it's not unusual to see
			// attempts to hit this API with an invalid connector ID. These are not legitimate SSO
			// attempts, so avoid cluttering the audit log with them.
			log.WithField("connector", req.ConnectorID).Infoln("rejecting invalid OIDC auth request")
		} else {
			emitSSOLoginFailureEvent(a.CloseContext(), a.authServer.emitter, events.LoginMethodOIDC, err, req.SSOTestFlow)
		}
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

func (a *ServerWithRoles) ValidateOIDCAuthCallback(ctx context.Context, q url.Values) (*authclient.OIDCAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	resp, err := a.authServer.ValidateOIDCAuthCallback(ctx, q)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Only the Proxy service can create web sessions via OIDC connector.
	if resp.Session != nil && !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}

	return resp, nil
}

func (a *ServerWithRoles) DeleteOIDCConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteOIDCConnector(ctx, connectorID)
}

// UpsertSAMLConnector creates or updates a SAML connector.
func (a *ServerWithRoles) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.SAML).Enabled {
		return nil, trace.Wrap(ErrSAMLRequiresEnterprise)
	}

	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := a.authServer.UpsertSAMLConnector(ctx, connector)
	return upserted, trace.Wrap(err)
}

// CreateSAMLConnector creates a new SAML connector.
func (a *ServerWithRoles) CreateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.SAML).Enabled {
		return nil, trace.Wrap(ErrSAMLRequiresEnterprise)
	}

	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := a.authServer.CreateSAMLConnector(ctx, connector)
	return created, trace.Wrap(err)
}

// UpdateSAMLConnector updates an existing SAML connector
func (a *ServerWithRoles) UpdateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.SAML).Enabled {
		return nil, trace.Wrap(ErrSAMLRequiresEnterprise)
	}

	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := a.authServer.UpdateSAMLConnector(ctx, connector)
	return updated, trace.Wrap(err)
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

	return a.authServer.GetSAMLConnector(ctx, id, withSecrets)
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
	return a.authServer.GetSAMLConnectors(ctx, withSecrets)
}

func (a *ServerWithRoles) CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.SAML).Enabled {
		return nil, trace.Wrap(ErrSAMLRequiresEnterprise)
	}

	if err := a.action(apidefaults.Namespace, types.KindSAMLRequest, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	// require additional permissions for executing SSO test flow.
	if req.SSOTestFlow {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbCreate); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Only the Proxy service can create web sessions via SAML connector.
	if req.CreateWebSession && !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}

	samlReq, err := a.authServer.CreateSAMLAuthRequest(ctx, req)
	if err != nil {
		if trace.IsNotFound(err) {
			// This flow is triggered via an unauthenticated endpoint, so it's not unusual to see
			// attempts to hit this API with an invalid connector ID. These are not legitimate SSO
			// attempts, so avoid cluttering the audit log with them.
			log.WithField("connector", req.ConnectorID).Infoln("rejecting invalid SAML auth request")
		} else {
			emitSSOLoginFailureEvent(a.CloseContext(), a.authServer.emitter, events.LoginMethodSAML, err, req.SSOTestFlow)
		}
		return nil, trace.Wrap(err)
	}

	return samlReq, nil
}

// ValidateSAMLResponse validates SAML auth response.
func (a *ServerWithRoles) ValidateSAMLResponse(ctx context.Context, samlResponse, connectorID, clientIP string) (*authclient.SAMLAuthResponse, error) {
	isProxy := a.hasBuiltinRole(types.RoleProxy)
	if !isProxy {
		clientIP = "" // We only trust IP information coming from the Proxy.
	}

	// auth callback is it's own authz, no need to check extra permissions
	resp, err := a.authServer.ValidateSAMLResponse(ctx, samlResponse, connectorID, clientIP)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Only the Proxy service can create web sessions via SAML connector.
	if resp.Session != nil && !isProxy {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}

	return resp, nil
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

	if err := a.context.AuthorizeAdminAction(); err != nil {
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
func (a *ServerWithRoles) UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.checkGithubConnector(connector); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := a.authServer.upsertGithubConnector(ctx, connector)
	return upserted, trace.Wrap(err)
}

// CreateGithubConnector creates a new Github connector.
func (a *ServerWithRoles) CreateGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.checkGithubConnector(connector); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := a.authServer.createGithubConnector(ctx, connector)
	return created, trace.Wrap(err)
}

// UpdateGithubConnector updates an existing Github connector.
func (a *ServerWithRoles) UpdateGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error) {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.checkGithubConnector(connector); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := a.authServer.updateGithubConnector(ctx, connector)
	return updated, trace.Wrap(err)
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
	return a.authServer.GetGithubConnector(ctx, id, withSecrets)
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
	return a.authServer.GetGithubConnectors(ctx, withSecrets)
}

// DeleteGithubConnector deletes a Github connector by name.
func (a *ServerWithRoles) DeleteGithubConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.deleteGithubConnector(ctx, connectorID)
}

func (a *ServerWithRoles) CreateGithubAuthRequest(ctx context.Context, req types.GithubAuthRequest) (*types.GithubAuthRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindGithubRequest, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	// require additional permissions for executing SSO test flow.
	if req.SSOTestFlow {
		if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbCreate); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Only the Proxy service can create web sessions via Github connector.
	if req.CreateWebSession && !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}

	githubReq, err := a.authServer.CreateGithubAuthRequest(ctx, req)
	if err != nil {
		if trace.IsNotFound(err) {
			// This flow is triggered via an unauthenticated endpoint, so it's not unusual to see
			// attempts to hit this API with an invalid connector ID. These are not legitimate SSO
			// attempts, so avoid cluttering the audit log with them.
			log.Infof("rejecting invalid GitHub auth request for connector %v", req.ConnectorID)
		} else {
			emitSSOLoginFailureEvent(a.authServer.closeCtx, a.authServer.emitter, events.LoginMethodGithub, err, req.SSOTestFlow)
		}
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

func (a *ServerWithRoles) ValidateGithubAuthCallback(ctx context.Context, q url.Values) (*authclient.GithubAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	resp, err := a.authServer.ValidateGithubAuthCallback(ctx, q)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Only the Proxy service can create web sessions via Github connector.
	if resp.Session != nil && !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}

	return resp, nil
}

// EmitAuditEvent emits a single audit event
func (a *ServerWithRoles) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	if err := a.action(apidefaults.Namespace, types.KindEvent, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	role, ok := a.context.Identity.(authz.BuiltinRole)
	if !ok || !role.IsServer() {
		return trace.AccessDenied("this request can be only executed by a teleport built-in server")
	}
	err := events.ValidateServerMetadata(event, role.GetServerID(), a.hasBuiltinRole(types.RoleProxy))
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
	role, ok := a.context.Identity.(authz.BuiltinRole)
	if !ok || !role.IsServer() {
		return nil, trace.AccessDenied("this request can be only executed by a Teleport server")
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
	role, ok := a.context.Identity.(authz.BuiltinRole)
	if !ok || !role.IsServer() {
		return nil, trace.AccessDenied("this request can be only executed by a Teleport server")
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

func (s *streamWithRoles) RecordEvent(ctx context.Context, pe apievents.PreparedSessionEvent) error {
	event := pe.GetAuditEvent()
	err := events.ValidateServerMetadata(event, s.serverID, s.a.hasBuiltinRole(types.RoleProxy))
	if err != nil {
		// TODO: this should be a proper audit event
		// notifying about access violation
		log.Warningf("Rejecting audit event %v from %v: %v. A node is attempting to "+
			"submit events for an identity other than the one on its x509 certificate.",
			event.GetID(), s.serverID, err)
		// this message is sparse on purpose to avoid conveying extra data to an attacker
		return trace.AccessDenied("failed to validate event metadata")
	}
	return s.stream.RecordEvent(ctx, pe)
}

func (a *ServerWithRoles) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	if err := a.actionForKindSession(namespace, sid); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionChunk(namespace, sid, offsetBytes, maxBytes)
}

func (a *ServerWithRoles) GetSessionEvents(namespace string, sid session.ID, afterN int) ([]events.EventFields, error) {
	if err := a.actionForKindSession(namespace, sid); err != nil {
		return nil, trace.Wrap(err)
	}

	// emit a session recording view event for the audit log
	if err := a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &apievents.SessionRecordingAccess{
		Metadata: apievents.Metadata{
			Type: events.SessionRecordingAccessEvent,
			Code: events.SessionRecordingAccessCode,
		},
		SessionID:    sid.String(),
		UserMetadata: a.context.Identity.GetIdentity().GetUserMetadata(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionEvents(namespace, sid, afterN)
}

func (a *ServerWithRoles) findSessionEndEvent(namespace string, sid session.ID) (apievents.AuditEvent, error) {
	sessionEvents, _, err := a.alog.SearchSessionEvents(context.TODO(), events.SearchSessionEventsRequest{
		From:  time.Time{},
		To:    a.authServer.clock.Now().UTC(),
		Limit: defaults.EventsIterationLimit,
		Order: types.EventOrderAscending,
		Cond: &types.WhereExpr{Equals: types.WhereExpr2{
			L: &types.WhereExpr{Field: events.SessionEventID},
			R: &types.WhereExpr{Literal: sid.String()},
		}},
		SessionID: sid.String(),
	})
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
	// delegate all access-control logic to ListRoles, which will eventually
	// supplant GetRoles everywhere.
	var roles []types.Role
	var req proto.ListRolesRequest
	for {
		rsp, err := a.ListRoles(ctx, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, r := range rsp.Roles {
			roles = append(roles, r)
		}
		req.StartKey = rsp.NextKey
		if req.StartKey == "" {
			break
		}
	}

	return roles, nil
}

// ListRoles is a paginated role getter.
func (a *ServerWithRoles) ListRoles(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error) {
	authErr := a.action(apidefaults.Namespace, types.KindRole, types.VerbList, types.VerbRead)
	if authErr == nil {
		rsp, err := a.authServer.ListRoles(ctx, req)
		return rsp, trace.Wrap(err)
	}

	firstPage := req.StartKey == ""

	// most users are at least allowed to view *some* roles, so fallback to per-role access checks.
	roles, nextKey, err := a.authServer.IterateRoles(ctx, req, func(role *types.RoleV6) (bool, error) {
		return a.actionForResource(role, types.KindRole, types.VerbList, types.VerbRead) == nil, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If we found no roles, let's return the previous auth error.
	if firstPage && len(roles) == 0 {
		return nil, trace.Wrap(authErr)
	}

	return &proto.ListRolesResponse{
		Roles:   roles,
		NextKey: nextKey,
	}, nil
}

func (a *ServerWithRoles) validateRole(ctx context.Context, role types.Role) error {
	if downgradeReason := role.GetMetadata().Labels[types.TeleportDowngradedLabel]; downgradeReason != "" {
		return trace.BadParameter("refusing to upsert role because %s label is set with reason %q",
			types.TeleportDowngradedLabel, downgradeReason)
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

	// check that the given RequireMFAType is supported in this build.
	if role.GetPrivateKeyPolicy().IsHardwareKeyPolicy() {
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return trace.AccessDenied("Hardware Key support is only available with an enterprise license")
		}
	}

	return nil
}

// CreateRole creates a new role.
func (a *ServerWithRoles) CreateRole(ctx context.Context, role types.Role) (types.Role, error) {
	if err := a.actionForResource(role, types.KindRole, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.validateRole(ctx, role); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := a.authServer.CreateRole(ctx, role)
	return created, trace.Wrap(err)
}

// UpdateRole updates an existing role.
func (a *ServerWithRoles) UpdateRole(ctx context.Context, role types.Role) (types.Role, error) {
	oldRole, getErr := a.authServer.GetRole(ctx, role.GetName())
	oldRoleIsNotFound := trace.IsNotFound(getErr)
	if getErr != nil && !oldRoleIsNotFound {
		return nil, trace.Wrap(getErr)
	}

	// If oldRole is nil, this whole call will fail. However, we'll continue so that the rest of
	// the call can return an appropriate error.
	if oldRole != nil {
		if err := a.actionForResource(oldRole, types.KindRole, types.VerbUpdate); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := a.actionForResource(role, types.KindRole, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.validateRole(ctx, role); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := a.authServer.UpdateRole(ctx, role)
	return updated, trace.Wrap(err)
}

// UpsertRole creates or updates role.
func (a *ServerWithRoles) UpsertRole(ctx context.Context, role types.Role) (types.Role, error) {
	oldRole, err := a.authServer.GetRole(ctx, role.GetName())
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	verbs := []string{types.VerbCreate}
	if oldRole != nil {
		verbs = []string{types.VerbUpdate}
		if err := a.actionForResource(oldRole, types.KindRole, verbs...); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := a.actionForResource(role, types.KindRole, verbs...); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.validateRole(ctx, role); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := a.authServer.UpsertRole(ctx, role)
	return upserted, trace.Wrap(err)
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
	case modules.GetModules().BuildType() != modules.BuildEnterprise && len(allowReq.SearchAsRoles) != 0:
		return trace.AccessDenied(
			"role field allow.search_as_roles is only available in enterprise subscriptions")
	default:
		return nil
	}
}

// GetRole returns role by name
func (a *ServerWithRoles) GetRole(ctx context.Context, name string) (types.Role, error) {
	// Current-user exception: we always allow users to read roles
	// that they hold.  This requirement is checked first to avoid
	// misleading denial messages in the logs.
	if slices.Contains(a.context.User.GetRoles(), name) {
		return a.authServer.GetRole(ctx, name)
	}

	authErr := a.action(apidefaults.Namespace, types.KindRole, types.VerbRead)
	if authErr == nil {
		return a.authServer.GetRole(ctx, name)
	}

	// See if the user has access to this individual role.
	role, err := a.authServer.GetRole(ctx, name)
	if err != nil {
		return nil, trace.Wrap(authErr)
	}

	if err := a.actionForResource(role, types.KindRole, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return role, nil
}

// DeleteRole deletes role by name
func (a *ServerWithRoles) DeleteRole(ctx context.Context, name string) error {
	// See if the user has access to this individual role. If this get fails,
	// the delete will fail, but we'll leave it to the later call to send the appropriate
	// error.
	role, err := a.authServer.GetRole(ctx, name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if err := a.actionForResource(role, types.KindRole, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	// DELETE IN (7.0)
	// It's OK to delete this code alongside migrateOSS code in auth.
	// It prevents 6.0 from migrating resources multiple times
	// and the role is used for `tctl users add` code too.
	if modules.GetModules().IsOSSBuild() && name == teleport.AdminRoleName {
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

// GetAuthPreference gets cluster auth preference.
func (a *ServerWithRoles) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	if err := a.action(apidefaults.Namespace, types.KindClusterAuthPreference, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := a.authServer.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cfg.Clone(), nil
}

func (a *ServerWithRoles) GetUIConfig(ctx context.Context) (types.UIConfig, error) {
	if err := a.action(apidefaults.Namespace, types.KindUIConfig, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := a.authServer.GetUIConfig(ctx)
	return cfg, trace.Wrap(err)
}

func (a *ServerWithRoles) SetUIConfig(ctx context.Context, uic types.UIConfig) error {
	if err := a.action(apidefaults.Namespace, types.KindUIConfig, types.VerbUpdate, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.SetUIConfig(ctx, uic))
}

func (a *ServerWithRoles) DeleteUIConfig(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindUIConfig, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.DeleteUIConfig(ctx))
}

// GetInstaller retrieves an installer script resource
func (a *ServerWithRoles) GetInstaller(ctx context.Context, name string) (types.Installer, error) {
	if err := a.action(apidefaults.Namespace, types.KindInstaller, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetInstaller(ctx, name)
}

// GetInstallers gets all the installer resources.
func (a *ServerWithRoles) GetInstallers(ctx context.Context) ([]types.Installer, error) {
	if err := a.action(apidefaults.Namespace, types.KindInstaller, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetInstallers(ctx)
}

// SetInstaller sets an Installer script resource
func (a *ServerWithRoles) SetInstaller(ctx context.Context, inst types.Installer) error {
	if err := a.action(apidefaults.Namespace, types.KindInstaller, types.VerbUpdate, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.SetInstaller(ctx, inst))
}

// DeleteInstaller removes an installer script resource
func (a *ServerWithRoles) DeleteInstaller(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindInstaller, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.DeleteInstaller(ctx, name))
}

// DeleteAllInstallers removes all installer script resources
func (a *ServerWithRoles) DeleteAllInstallers(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindInstaller, types.VerbDelete, types.VerbList); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.DeleteAllInstallers(ctx))
}

// SetAuthPreference sets cluster auth preference.
// Deprecated: Use Update/UpsertAuthPreference where appropriate.
func (a *ServerWithRoles) SetAuthPreference(ctx context.Context, newAuthPref types.AuthPreference) error {
	storedAuthPref, err := a.authServer.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.action(apidefaults.Namespace, types.KindClusterAuthPreference, verbsToReplaceResourceWithOrigin(storedAuthPref)...); err != nil {
		return trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}

	// check that the given RequireMFAType is supported in this build.
	if newAuthPref.GetPrivateKeyPolicy().IsHardwareKeyPolicy() {
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return trace.AccessDenied("Hardware Key support is only available with an enterprise license")
		}
	}

	if err := dtconfig.ValidateConfigAgainstModules(newAuthPref.GetDeviceTrust()); err != nil {
		return trace.Wrap(err)
	}

	_, err = a.authServer.UpsertAuthPreference(ctx, newAuthPref)

	var msg string
	if err != nil {
		msg = err.Error()
	}

	oldSecondFactor := storedAuthPref.GetSecondFactor()
	newSecondFactor := newAuthPref.GetSecondFactor()

	if auditErr := a.authServer.emitter.EmitAuditEvent(ctx, &apievents.AuthPreferenceUpdate{
		Metadata: apievents.Metadata{
			Type: events.AuthPreferenceUpdateEvent,
			Code: events.AuthPreferenceUpdateCode,
		},
		UserMetadata:       a.context.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success:     err == nil,
			Error:       msg,
			UserMessage: msg,
		},
		AdminActionsMFA: clusterconfigv1.GetAdminActionsMFAStatus(oldSecondFactor, newSecondFactor),
	}); auditErr != nil {
		log.WithError(auditErr).Warn("Failed to emit auth preference update event event.")
	}

	return trace.Wrap(err)
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

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	defaultAuthPref := types.DefaultAuthPreference()
	_, err = a.authServer.UpsertAuthPreference(ctx, defaultAuthPref)

	var msg string
	if err != nil {
		msg = err.Error()
	}

	oldSecondFactor := storedAuthPref.GetSecondFactor()
	newSecondFactor := defaultAuthPref.GetSecondFactor()

	if auditErr := a.authServer.emitter.EmitAuditEvent(ctx, &apievents.AuthPreferenceUpdate{
		Metadata: apievents.Metadata{
			Type: events.AuthPreferenceUpdateEvent,
			Code: events.AuthPreferenceUpdateCode,
		},
		UserMetadata:       a.context.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success:     err == nil,
			Error:       msg,
			UserMessage: msg,
		},
		AdminActionsMFA: clusterconfigv1.GetAdminActionsMFAStatus(oldSecondFactor, newSecondFactor),
	}); auditErr != nil {
		log.WithError(auditErr).Warn("Failed to emit auth preference update event event.")
	}

	return trace.Wrap(err)
}

// GetClusterAuditConfig gets cluster audit configuration.
func (a *ServerWithRoles) GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error) {
	if err := a.action(apidefaults.Namespace, types.KindClusterAuditConfig, types.VerbRead); err != nil {
		if err2 := a.action(apidefaults.Namespace, types.KindClusterConfig, types.VerbRead); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetClusterAuditConfig(ctx)
}

// GetClusterNetworkingConfig gets cluster networking configuration.
func (a *ServerWithRoles) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	if err := a.action(apidefaults.Namespace, types.KindClusterNetworkingConfig, types.VerbRead); err != nil {
		if err2 := a.action(apidefaults.Namespace, types.KindClusterConfig, types.VerbRead); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}
	cfg, err := a.authServer.GetReadOnlyClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cfg.Clone(), nil
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

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}

	tst, err := newNetConfig.GetTunnelStrategyType()
	if err != nil {
		return trace.Wrap(err)
	}
	if tst == types.ProxyPeering &&
		modules.GetModules().BuildType() != modules.BuildEnterprise {
		return trace.AccessDenied("proxy peering is an enterprise-only feature")
	}

	oldNetConf, err := a.authServer.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := clusterconfigv1.ValidateCloudNetworkConfigUpdate(a.context, newNetConfig, oldNetConf); err != nil {
		return trace.Wrap(err)
	}

	_, err = a.authServer.UpsertClusterNetworkingConfig(ctx, newNetConfig)
	var msg string
	if err != nil {
		msg = err.Error()
	}
	if auditErr := a.authServer.emitter.EmitAuditEvent(ctx, &apievents.ClusterNetworkingConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.ClusterNetworkingConfigUpdateEvent,
			Code: events.ClusterNetworkingConfigUpdateCode,
		},
		UserMetadata:       a.context.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success:     err == nil,
			Error:       msg,
			UserMessage: msg,
		},
	}); auditErr != nil {
		log.WithError(auditErr).Warn("Failed to emit cluster networking config update event event.")
	}
	return trace.Wrap(err)
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
	oldNetConf, err := a.authServer.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := clusterconfigv1.ValidateCloudNetworkConfigUpdate(a.context, types.DefaultClusterNetworkingConfig(), oldNetConf); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	_, err = a.authServer.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())

	var msg string
	if err != nil {
		msg = err.Error()
	}
	if auditErr := a.authServer.emitter.EmitAuditEvent(ctx, &apievents.ClusterNetworkingConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.ClusterNetworkingConfigUpdateEvent,
			Code: events.ClusterNetworkingConfigUpdateCode,
		},
		UserMetadata:       a.context.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success:     err == nil,
			Error:       msg,
			UserMessage: msg,
		},
	}); auditErr != nil {
		log.WithError(auditErr).Warn("Failed to emit cluster networking config update event event.")
	}

	return trace.Wrap(err)
}

// GetSessionRecordingConfig gets session recording configuration.
func (a *ServerWithRoles) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	if err := a.action(apidefaults.Namespace, types.KindSessionRecordingConfig, types.VerbRead); err != nil {
		if err2 := a.action(apidefaults.Namespace, types.KindClusterConfig, types.VerbRead); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetSessionRecordingConfig(ctx)
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

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}

	_, err = a.authServer.UpsertSessionRecordingConfig(ctx, newRecConfig)

	var msg string
	if err != nil {
		msg = err.Error()
	}
	if auditErr := a.authServer.emitter.EmitAuditEvent(ctx, &apievents.SessionRecordingConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.SessionRecordingConfigUpdateEvent,
			Code: events.SessionRecordingConfigUpdateCode,
		},
		UserMetadata:       a.context.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success:     err == nil,
			Error:       msg,
			UserMessage: msg,
		},
	}); auditErr != nil {
		log.WithError(auditErr).Warn("Failed to emit session recording config update event event.")
	}

	return trace.Wrap(err)
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

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	_, err = a.authServer.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())

	var msg string
	if err != nil {
		msg = err.Error()
	}
	if auditErr := a.authServer.emitter.EmitAuditEvent(ctx, &apievents.SessionRecordingConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.SessionRecordingConfigUpdateEvent,
			Code: events.SessionRecordingConfigUpdateCode,
		},
		UserMetadata:       a.context.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success:     err == nil,
			Error:       msg,
			UserMessage: msg,
		},
	}); auditErr != nil {
		log.WithError(auditErr).Warn("Failed to emit session recording config update event event.")
	}

	return trace.Wrap(err)
}

// GetServerInfos returns a stream of ServerInfos.
func (a *ServerWithRoles) GetServerInfos(ctx context.Context) stream.Stream[types.ServerInfo] {
	if err := a.action(apidefaults.Namespace, types.KindServerInfo, types.VerbList, types.VerbRead); err != nil {
		return stream.Fail[types.ServerInfo](trace.Wrap(err))
	}

	return a.authServer.GetServerInfos(ctx)
}

// GetServerInfo returns a ServerInfo by name.
func (a *ServerWithRoles) GetServerInfo(ctx context.Context, name string) (types.ServerInfo, error) {
	if err := a.action(apidefaults.Namespace, types.KindServerInfo, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	info, err := a.authServer.GetServerInfo(ctx, name)
	return info, trace.Wrap(err)
}

// UpsertServerInfo upserts a ServerInfo.
func (a *ServerWithRoles) UpsertServerInfo(ctx context.Context, si types.ServerInfo) error {
	if err := a.action(apidefaults.Namespace, types.KindServerInfo, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.authServer.UpsertServerInfo(ctx, si))
}

// DeleteServerInfo deletes a ServerInfo by name.
func (a *ServerWithRoles) DeleteServerInfo(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindServerInfo, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.authServer.DeleteServerInfo(ctx, name))
}

// DeleteAllServerInfos deletes all ServerInfos.
func (a *ServerWithRoles) DeleteAllServerInfos(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindServerInfo, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.authServer.DeleteAllServerInfos(ctx))
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
	// Don't allow a Cloud tenant to be a leaf cluster.
	if modules.GetModules().Features().Cloud {
		return nil, trace.NotImplemented("cloud tenants cannot be leaf clusters")
	}

	if err := a.action(apidefaults.Namespace, types.KindTrustedCluster, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.UpsertTrustedCluster(ctx, tc)
}

func (a *ServerWithRoles) ValidateTrustedCluster(ctx context.Context, validateRequest *authclient.ValidateTrustedClusterRequest) (*authclient.ValidateTrustedClusterResponse, error) {
	// Don't allow a leaf cluster to be added to a Cloud tenant.
	if modules.GetModules().Features().Cloud {
		return nil, trace.NotImplemented("leaf clusters cannot be added to cloud tenants")
	}

	// the token provides it's own authorization and authentication
	return a.authServer.validateTrustedCluster(ctx, validateRequest)
}

// DeleteTrustedCluster deletes a trusted cluster by name.
func (a *ServerWithRoles) DeleteTrustedCluster(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindTrustedCluster, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
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

// Deprecated: use [presencev1.PresenceService.GetRemoteCluster]
// TODO(noah): DELETE IN 17.0.0
func (a *ServerWithRoles) GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error) {
	if err := a.action(apidefaults.Namespace, types.KindRemoteCluster, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := a.authServer.GetRemoteCluster(ctx, clusterName)
	if err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}
	if err := a.context.Checker.CheckAccessToRemoteCluster(cluster); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}
	return cluster, nil
}

// Deprecated: use [presencev1.PresenceService.ListRemoteClusters]
// TODO(noah): DELETE IN 17.0.0
func (a *ServerWithRoles) GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	if err := a.action(apidefaults.Namespace, types.KindRemoteCluster, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	remoteClusters, err := a.authServer.GetRemoteClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.filterRemoteClustersForUser(remoteClusters)
}

// filterRemoteClustersForUser filters remote clusters based on what the current user is authorized to access
// TODO(noah): DELETE IN 17.0.0
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

// Deprecated: use [presencev1.PresenceService.DeleteRemoteCluster]
// TODO(noah): DELETE IN 17.0.0
func (a *ServerWithRoles) DeleteRemoteCluster(ctx context.Context, clusterName string) error {
	if err := a.action(apidefaults.Namespace, types.KindRemoteCluster, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteRemoteCluster(ctx, clusterName)
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
func (a *ServerWithRoles) ProcessKubeCSR(req authclient.KubeCSR) (*authclient.KubeCSRResponse, error) {
	// limits the requests types to proxies to make it harder to break
	if !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyClusterName := a.context.Identity.GetIdentity().TeleportCluster
	identityClusterName, err := extractOriginalClusterNameFromCSR(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if proxyClusterName != "" &&
		proxyClusterName != clusterName.GetClusterName() &&
		proxyClusterName != identityClusterName {
		log.WithFields(
			logrus.Fields{
				"proxy_cluster_name":    proxyClusterName,
				"identity_cluster_name": identityClusterName,
			},
		).Warn("KubeCSR request denied because the proxy and identity clusters didn't match")
		return nil, trace.AccessDenied("can not sign certs for users via a different cluster proxy")
	}
	return a.authServer.ProcessKubeCSR(req)
}

func extractOriginalClusterNameFromCSR(req authclient.KubeCSR) (string, error) {
	csr, err := tlsca.ParseCertificateRequestPEM(req.CSR)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Extract identity from the CSR. Pass zero time for id.Expiry, it won't be
	// used here.
	id, err := tlsca.FromSubject(csr.Subject, time.Time{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return id.TeleportCluster, nil
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

// UpsertDatabaseService creates or updates a new DatabaseService resource.
func (a *ServerWithRoles) UpsertDatabaseService(ctx context.Context, service types.DatabaseService) (*types.KeepAlive, error) {
	if err := a.action(service.GetNamespace(), types.KindDatabaseService, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.UpsertDatabaseService(ctx, service)
}

// DeleteAllDatabaseServices removes all DatabaseService resources.
func (a *ServerWithRoles) DeleteAllDatabaseServices(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindDatabaseService, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllDatabaseServices(ctx)
}

// DeleteDatabaseService removes a specific DatabaseService resource.
func (a *ServerWithRoles) DeleteDatabaseService(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindDatabaseService, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteDatabaseService(ctx, name)
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

// GenerateDatabaseCert generates a client certificate used by a database
// service to authenticate with the database instance, or a server certificate
// for configuring a self-hosted database, depending on the requester_name.
//
// This certificate can be requested by:
//
//   - Cluster administrator using "tctl auth sign --format=db" command locally
//     on the auth server to produce a certificate for configuring a self-hosted
//     database.
//   - Remote user using "tctl auth sign --format=db" command or
//     /webapi/sites/:site/sign/db with a remote proxy (e.g. Teleport Cloud),
//     as long as they can impersonate system role Db.
//   - Database service when initiating connection to a database instance to
//     produce a client certificate.
func (a *ServerWithRoles) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	err := a.checkAccessToGenerateDatabaseCert(types.KindDatabaseCertificate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateDatabaseCert(ctx, req)
}

// checkAccessToGenerateDatabaseCert is a helper for checking db cert gen authz.
// Requester must have at least one of:
// - create: database_certificate or database_client_certificate.
// - built-in Admin or DB role.
// - allowed to impersonate the built-in DB role.
func (a *ServerWithRoles) checkAccessToGenerateDatabaseCert(resourceKind string) error {
	const verb = types.VerbCreate
	// Check if the User can `create` Database Certificates
	err := a.action(apidefaults.Namespace, resourceKind, verb)
	if err != nil {
		if !trace.IsAccessDenied(err) {
			return trace.Wrap(err)
		}

		// Err is access denied, trying the old way

		// Check if this is a local cluster admin, or a database service, or a
		// user that is allowed to impersonate database service.
		if !a.hasBuiltinRole(types.RoleDatabase, types.RoleAdmin) {
			if err := a.canImpersonateBuiltinRole(types.RoleDatabase); err != nil {
				log.WithError(err).Warnf("User %v tried to generate database certificate but does not have '%s' permission for '%s' kind, nor is allowed to impersonate %q system role",
					a.context.User.GetName(), verb, resourceKind, types.RoleDatabase)
				return trace.AccessDenied("access denied. User must have '%s' permission for '%s' kind to generate the certificate ",
					verb, resourceKind)
			}
		}
	}
	return nil
}

// GenerateSnowflakeJWT generates JWT in the Snowflake required format.
func (a *ServerWithRoles) GenerateSnowflakeJWT(ctx context.Context, req *proto.SnowflakeJWTRequest) (*proto.SnowflakeJWTResponse, error) {
	// Check if this is a local cluster admin, or a database service, or a
	// user that is allowed to impersonate database service.
	if !a.hasBuiltinRole(types.RoleDatabase, types.RoleAdmin) {
		if err := a.canImpersonateBuiltinRole(types.RoleDatabase); err != nil {
			log.WithError(err).Warnf("User %v tried to generate database certificate but is not allowed to impersonate %q system role.",
				a.context.User.GetName(), types.RoleDatabase)
			return nil, trace.AccessDenied(`access denied. The user must be able to impersonate the builtin role and user "Db" in order to generate database certificates, for more info see https://goteleport.com/docs/reference/agent-services/database-access-reference/cli/#tctl-auth-sign.`)
		}
	}
	return a.authServer.GenerateSnowflakeJWT(ctx, req)
}

// canImpersonateBuiltinRole checks if the current user can impersonate the
// provided system role.
func (a *ServerWithRoles) canImpersonateBuiltinRole(role types.SystemRole) error {
	roleCtx, err := authz.NewBuiltinRoleContext(role)
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
		services.AccessState{MFAVerified: true})
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

// GetAppServersAndSAMLIdPServiceProviders returns a list containing all registered AppServers and SAMLIdPServiceProviders.
//
//nolint:staticcheck // SA1019. TODO(sshah) DELETE IN 17.0
func (a *ServerWithRoles) GetAppServersAndSAMLIdPServiceProviders(ctx context.Context, namespace string) ([]types.AppServerOrSAMLIdPServiceProvider, error) {
	appservers, err := a.GetApplicationServers(ctx, namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var appsAndSPs []types.AppServerOrSAMLIdPServiceProvider
	// Convert the AppServers to AppServerOrSAMLIdPServiceProviders.
	for _, appserver := range appservers {
		appServerV3 := appserver.(*types.AppServerV3)
		appAndSP := &types.AppServerOrSAMLIdPServiceProviderV1{
			Resource: &types.AppServerOrSAMLIdPServiceProviderV1_AppServer{
				AppServer: appServerV3,
			},
		}
		appsAndSPs = append(appsAndSPs, appAndSP)
	}

	// Only add SAMLIdPServiceProviders to the list if the caller has an enterprise license since this is an enteprise-only feature.
	if modules.GetModules().BuildType() == modules.BuildEnterprise {
		// Only attempt to list SAMLIdPServiceProviders if the caller has the permission to.
		if err := a.action(namespace, types.KindSAMLIdPServiceProvider, types.VerbList); err == nil {
			serviceProviders, _, err := a.authServer.ListSAMLIdPServiceProviders(ctx, 0, "")
			if err != nil {
				return nil, trace.Wrap(err)
			}
			for _, sp := range serviceProviders {
				spV1 := sp.(*types.SAMLIdPServiceProviderV1)
				appAndSP := &types.AppServerOrSAMLIdPServiceProviderV1{
					Resource: &types.AppServerOrSAMLIdPServiceProviderV1_SAMLIdPServiceProvider{
						SAMLIdPServiceProvider: spV1,
					},
				}
				appsAndSPs = append(appsAndSPs, appAndSP)
			}
		}
	}

	return appsAndSPs, nil
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

// GetAppSession gets an application web session.
func (a *ServerWithRoles) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	session, err := a.authServer.GetAppSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authErr := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbRead)
	if authErr == nil {
		return session, nil
	}

	// Users can fetch their own app sessions without secrets.
	if err := a.currentUserAction(session.GetUser()); err == nil {
		// TODO (Joerger): DELETE IN 17.0.0
		// App Session secrets should not be returned to the user. We only do this
		// here for backwards compatibility with `tsh proxy azure`, which uses the
		// app session key to sign JWT tokens with Azure claims. This check means
		// that `tsh proxy azure` will fail for old clients when used with Per-session
		// MFA or Hardware Key support, which is planned for release in v16.0.0.
		identity := a.context.Identity.GetIdentity()
		if !identity.IsMFAVerified() && identity.PrivateKeyPolicy != keys.PrivateKeyPolicyWebSession {
			return session, nil
		}

		return session.WithoutSecrets(), nil
	}

	return nil, trace.Wrap(authErr)
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
	// Check if this a database service.
	if !a.hasBuiltinRole(types.RoleDatabase) {
		// Users can only fetch their own web sessions.
		if err := a.currentUserAction(session.GetUser()); err != nil {
			if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	return session, nil
}

// GetSAMLIdPSession gets a SAML IdP session.
// TODO(Joerger): DELETE IN v18.0.0
func (a *ServerWithRoles) GetSAMLIdPSession(ctx context.Context, req types.GetSAMLIdPSessionRequest) (types.WebSession, error) {
	if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := a.authServer.GetSAMLIdPSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if session.GetSubKind() != types.KindSAMLIdPSession {
		return nil, trace.AccessDenied("GetSAMLIdPSession only allows reading sessions with SubKind SAMLIdpSession")
	}
	return session, nil
}

// ListAppSessions gets a paginated list of application web sessions.
func (a *ServerWithRoles) ListAppSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error) {
	if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbList, types.VerbRead); err != nil {
		return nil, "", trace.Wrap(err)
	}

	sessions, nextKey, err := a.authServer.ListAppSessions(ctx, pageSize, pageToken, user)
	return sessions, nextKey, trace.Wrap(err)
}

// GetSnowflakeSessions gets all Snowflake web sessions.
func (a *ServerWithRoles) GetSnowflakeSessions(ctx context.Context) ([]types.WebSession, error) {
	// Check if this a database service.
	if !a.hasBuiltinRole(types.RoleDatabase) {
		if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbList, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	sessions, err := a.authServer.GetSnowflakeSessions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sessions, nil
}

// ListSAMLIdPSessions gets a paginated list of SAML IdP sessions.
// TODO(Joerger): DELETE IN v18.0.0
func (a *ServerWithRoles) ListSAMLIdPSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error) {
	if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbList, types.VerbRead); err != nil {
		return nil, "", trace.Wrap(err)
	}

	sessions, nextKey, err := a.authServer.ListSAMLIdPSessions(ctx, pageSize, pageToken, user)
	return sessions, nextKey, trace.Wrap(err)
}

// CreateAppSession creates an application web session. Application web
// sessions represent a browser session the client holds.
func (a *ServerWithRoles) CreateAppSession(ctx context.Context, req *proto.CreateAppSessionRequest) (types.WebSession, error) {
	if err := a.currentUserAction(req.Username); err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := a.authServer.CreateAppSession(ctx, req, a.context.Identity.GetIdentity(), a.context.Checker)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session.WithoutSecrets(), nil
}

// CreateSnowflakeSession creates a Snowflake web session.
func (a *ServerWithRoles) CreateSnowflakeSession(ctx context.Context, req types.CreateSnowflakeSessionRequest) (types.WebSession, error) {
	// Check if this a database service.
	if !a.hasBuiltinRole(types.RoleDatabase) {
		if err := a.currentUserAction(req.Username); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	snowflakeSession, err := a.authServer.CreateSnowflakeSession(ctx, req, a.context.Identity.GetIdentity(), a.context.Checker)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return snowflakeSession, nil
}

// CreateSAMLIdPSession creates a SAML IdP session.
// TODO(Joerger): DELETE IN v18.0.0
func (a *ServerWithRoles) CreateSAMLIdPSession(ctx context.Context, req types.CreateSAMLIdPSessionRequest) (types.WebSession, error) {
	// Check if this a proxy service.
	if !a.hasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}

	samlSession, err := a.authServer.CreateSAMLIdPSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return samlSession, nil
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
	if !a.hasBuiltinRole(types.RoleDatabase) {
		if err := a.canDeleteWebSession(snowflakeSession.GetUser()); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := a.authServer.DeleteSnowflakeSession(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteSAMLIdPSession removes a SAML IdP session.
// TODO(Joerger): DELETE IN v18.0.0
func (a *ServerWithRoles) DeleteSAMLIdPSession(ctx context.Context, req types.DeleteSAMLIdPSessionRequest) error {
	samlSession, err := a.authServer.GetSAMLIdPSession(ctx, types.GetSAMLIdPSessionRequest(req))
	if err != nil {
		return trace.Wrap(err)
	}
	// Check if user can delete this web session.
	if err := a.canDeleteWebSession(samlSession.GetUser()); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authServer.DeleteSAMLIdPSession(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllSnowflakeSessions removes all Snowflake web sessions.
func (a *ServerWithRoles) DeleteAllSnowflakeSessions(ctx context.Context) error {
	if !a.hasBuiltinRole(types.RoleDatabase) {
		if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbList, types.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
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

// DeleteAllSAMLIdPSessions removes all SAML IdP sessions.
// TODO(Joerger): DELETE IN v18.0.0
func (a *ServerWithRoles) DeleteAllSAMLIdPSessions(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.DeleteAllSAMLIdPSessions(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteUserSAMLIdPSessions deletes all of a user's SAML IdP sessions.
// TODO(Joerger): DELETE IN v18.0.0
func (a *ServerWithRoles) DeleteUserSAMLIdPSessions(ctx context.Context, username string) error {
	// First, check if the current user can delete the request user sessions.
	if err := a.canDeleteWebSession(username); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.DeleteUserSAMLIdPSessions(ctx, username); err != nil {
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

	session, err := a.authServer.generateAppToken(ctx, req.Username, req.Roles, req.Traits, req.URI, req.Expires)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return session, nil
}

func (a *ServerWithRoles) Close() error {
	return a.authServer.Close()
}

func (a *ServerWithRoles) checkAccessToKubeCluster(cluster types.KubeCluster) error {
	return a.context.Checker.CheckAccess(
		cluster,
		// MFA is not required for operations on kube clusters resources but
		// will be enforced at the connection time.
		services.AccessState{MFAVerified: true})
}

// GetKubernetesServers returns all registered kubernetes servers.
func (a *ServerWithRoles) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	if err := a.action(apidefaults.Namespace, types.KindKubeServer, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := a.authServer.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Filter out kube servers the caller doesn't have access to.
	var filtered []types.KubeServer
	for _, server := range servers {
		err := a.checkAccessToKubeCluster(server.GetCluster())
		if err != nil && !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		} else if err == nil {
			filtered = append(filtered, server)
		}
	}

	return filtered, nil
}

// UpsertKubernetesServer creates or updates a Server representing a teleport
// kubernetes server.
func (a *ServerWithRoles) UpsertKubernetesServer(ctx context.Context, s types.KubeServer) (*types.KeepAlive, error) {
	if err := a.action(apidefaults.Namespace, types.KindKubeServer, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.UpsertKubernetesServer(ctx, s)
}

// DeleteKubernetesServer deletes specified kubernetes server.
func (a *ServerWithRoles) DeleteKubernetesServer(ctx context.Context, hostID, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindKubeServer, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteKubernetesServer(ctx, hostID, name)
}

// DeleteAllKubernetesServers deletes all registered kubernetes servers.
func (a *ServerWithRoles) DeleteAllKubernetesServers(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindKubeServer, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllKubernetesServers(ctx)
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

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.SetNetworkRestrictions(ctx, nr)
}

// DeleteNetworkRestrictions deletes the network restrictions.
func (a *ServerWithRoles) DeleteNetworkRestrictions(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindNetworkRestrictions, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteNetworkRestrictions(ctx)
}

// GetMFADevices returns a list of MFA devices.
func (a *ServerWithRoles) GetMFADevices(ctx context.Context, req *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error) {
	return a.authServer.GetMFADevices(ctx, req)
}

// AddMFADeviceSync is implemented by AuthService.AddMFADeviceSync.
func (a *ServerWithRoles) AddMFADeviceSync(ctx context.Context, req *proto.AddMFADeviceSyncRequest) (*proto.AddMFADeviceSyncResponse, error) {
	switch {
	case req.TokenID != "":
	default: // ContextUser
		if !authz.IsLocalOrRemoteUser(a.context) {
			return nil, trace.BadParameter("only end users are allowed to register devices using ContextUser")
		}
	}

	// The following serve as means of authentication for this RPC:
	//   - privilege token (or equivalent)
	//   - authenticated user using non-Proxy identity
	resp, err := a.authServer.AddMFADeviceSync(ctx, req)
	return resp, trace.Wrap(err)
}

// DeleteMFADeviceSync is implemented by AuthService.DeleteMFADeviceSync.
func (a *ServerWithRoles) DeleteMFADeviceSync(ctx context.Context, req *proto.DeleteMFADeviceSyncRequest) error {
	switch {
	case req.TokenID != "":
		// OK. Token holds the user.
	case req.ExistingMFAResponse != nil:
		// Sanity check: caller must be an end user.
		if !authz.IsLocalOrRemoteUser(a.context) {
			return trace.BadParameter("only end users are allowed to delete devices using an MFA authentication challenge")
		}
	default:
		// Let Server.DeleteMFADeviceSync handle the failure.
	}

	return a.authServer.DeleteMFADeviceSync(ctx, req)
}

// IsMFARequired queries whether MFA is required for the specified target.
func (a *ServerWithRoles) IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	// Check if MFA is required for admin actions. We don't currently have
	// a reason to check the name of the admin action in question.
	if _, ok := req.Target.(*proto.IsMFARequiredRequest_AdminAction); ok {
		if a.context.AdminActionAuthState == authz.AdminActionAuthUnauthorized {
			return &proto.IsMFARequiredResponse{
				Required:    true,
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			}, nil
		} else {
			return &proto.IsMFARequiredResponse{
				Required:    false,
				MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
			}, nil
		}
	}

	// Other than for admin action targets, IsMFARequired should only be called by users.
	if !authz.IsLocalOrRemoteUser(a.context) {
		return nil, trace.AccessDenied("only a user role can call IsMFARequired, got %T", a.context.Checker)
	}

	// Certain hardware-key based private key policies are treated as MFA verification,
	// except for app sessions which can only be attested with the key policy "web_session".
	if a.context.Identity.GetIdentity().PrivateKeyPolicy.MFAVerified() {
		if _, isAppReq := req.Target.(*proto.IsMFARequiredRequest_App); !isAppReq {
			return &proto.IsMFARequiredResponse{
				Required:    false,
				MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
			}, nil
		}
	}

	return a.authServer.isMFARequired(ctx, a.context.Checker, req)
}

// SearchEvents allows searching audit events with pagination support.
func (a *ServerWithRoles) SearchEvents(ctx context.Context, req events.SearchEventsRequest) (outEvents []apievents.AuditEvent, lastKey string, err error) {
	if err := a.action(apidefaults.Namespace, types.KindEvent, types.VerbList); err != nil {
		return nil, "", trace.Wrap(err)
	}

	outEvents, lastKey, err = a.alog.SearchEvents(ctx, req)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return outEvents, lastKey, nil
}

// ExportUnstructuredEvents exports events from a given event chunk returned by GetEventExportChunks. This API prioritizes
// performance over ordering and filtering, and is intended for bulk export of events.
func (a *ServerWithRoles) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured] {
	if err := a.action(apidefaults.Namespace, types.KindEvent, types.VerbList); err != nil {
		return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.Wrap(err))
	}

	return a.alog.ExportUnstructuredEvents(ctx, req)
}

// GetEventExportChunks returns a stream of event chunks that can be exported via ExportUnstructuredEvents. The returned
// list isn't ordered and polling for new chunks requires re-consuming the entire stream from the beginning.
func (a *ServerWithRoles) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	if err := a.action(apidefaults.Namespace, types.KindEvent, types.VerbList); err != nil {
		return stream.Fail[*auditlogpb.EventExportChunk](trace.Wrap(err))
	}

	return a.alog.GetEventExportChunks(ctx, req)
}

// SearchSessionEvents allows searching session audit events with pagination support.
func (a *ServerWithRoles) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) (outEvents []apievents.AuditEvent, lastKey string, err error) {
	if req.Cond != nil {
		return nil, "", trace.BadParameter("cond is an internal parameter, should not be set by client")
	}

	cond, err := a.actionForListWithCondition(apidefaults.Namespace, types.KindSession, services.SessionIdentifier)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// TODO(codingllama): Refactor cond out of SearchSessionEvents and simplify signature.
	req.Cond = cond
	outEvents, lastKey, err = a.alog.SearchSessionEvents(ctx, req)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return outEvents, lastKey, nil
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

	if err := okta.CheckOrigin(&a.context, lock); err != nil {
		return trace.Wrap(err)
	}

	if err := checkOktaLockTarget(ctx, &a.context, a.authServer, lock); err != nil {
		return trace.Wrap(err)
	}

	if err := checkOktaLockAccess(ctx, &a.context, a.authServer, lock.GetName(), types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	if lock.CreatedBy() == "" {
		hasAdmin := a.hasBuiltinRole(types.RoleAdmin)
		createdBy := string(types.RoleAdmin)
		if !hasAdmin {
			createdBy = a.context.User.GetName()
		}
		lock.SetCreatedBy(createdBy)
	}

	if lock.CreatedAt().IsZero() {
		lock.SetCreatedAt(a.authServer.clock.Now().UTC())
	}

	return a.authServer.UpsertLock(ctx, lock)
}

// DeleteLock deletes a lock.
func (a *ServerWithRoles) DeleteLock(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindLock, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := checkOktaLockAccess(ctx, &a.context, a.authServer, name, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteLock(ctx, name)
}

// ReplaceRemoteLocks replaces the set of locks associated with a remote cluster.
func (a *ServerWithRoles) ReplaceRemoteLocks(ctx context.Context, clusterName string, locks []types.Lock) error {
	role, ok := a.context.Identity.(authz.RemoteBuiltinRole)
	if !a.hasRemoteBuiltinRole(string(types.RoleRemoteProxy)) || !ok || role.ClusterName != clusterName {
		return trace.AccessDenied("this request can be only executed by a remote proxy of cluster %q", clusterName)
	}
	return a.authServer.ReplaceRemoteLocks(ctx, clusterName, locks)
}

// StreamSessionEvents streams all events from a given session recording. An error is returned on the first
// channel if one is encountered. Otherwise the event channel is closed when the stream ends.
// The event channel is not closed on error to prevent race conditions in downstream select statements.
func (a *ServerWithRoles) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	err := a.localServerAction()
	isTeleportServer := err == nil

	// StreamSessionEvents can be called internally, and when that
	// happens we don't want to emit an event or check for permissions.
	if isTeleportServer {
		return a.alog.StreamSessionEvents(ctx, sessionID, startIndex)
	}

	if err := a.actionForKindSession(apidefaults.Namespace, sessionID); err != nil {
		c, e := make(chan apievents.AuditEvent), make(chan error, 1)
		e <- trace.Wrap(err)
		return c, e
	}

	// We can only determine the session type after the streaming started. For
	// this reason, we delay the emit audit event until the first event or if
	// the streaming returns an error.
	cb := func(evt apievents.AuditEvent, _ error) {
		if err := a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &apievents.SessionRecordingAccess{
			Metadata: apievents.Metadata{
				Type: events.SessionRecordingAccessEvent,
				Code: events.SessionRecordingAccessCode,
			},
			SessionID:    sessionID.String(),
			UserMetadata: a.context.Identity.GetIdentity().GetUserMetadata(),
			SessionType:  string(sessionTypeFromStartEvent(evt)),
			Format:       metadata.SessionRecordingFormatFromContext(ctx),
		}); err != nil {
			log.WithError(err).Errorf("Failed to emit stream session event audit event")
		}
	}

	return a.alog.StreamSessionEvents(events.ContextWithSessionStartCallback(ctx, cb), sessionID, startIndex)
}

// sessionTypeFromStartEvent determines the session type given the session start
// event.
func sessionTypeFromStartEvent(sessionStart apievents.AuditEvent) types.SessionKind {
	switch e := sessionStart.(type) {
	case *apievents.SessionStart:
		if e.KubernetesCluster != "" {
			return types.KubernetesSessionKind
		}
		return types.SSHSessionKind
	case *apievents.DatabaseSessionStart:
		return types.DatabaseSessionKind
	case *apievents.AppSessionStart:
		return types.AppSessionKind
	case *apievents.WindowsDesktopSessionStart:
		return types.WindowsDesktopSessionKind
	default:
		return types.UnknownSessionKind
	}
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

// CreateKubernetesCluster creates a new kubernetes cluster resource.
func (a *ServerWithRoles) CreateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	if err := a.action(apidefaults.Namespace, types.KindKubernetesCluster, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	// Don't allow users create clusters they wouldn't have access to (e.g.
	// non-matching labels).
	if err := a.checkAccessToKubeCluster(cluster); err != nil {
		return trace.Wrap(err)
	}
	// Don't allow discovery service to create clusters with dynamic labels.
	if a.hasBuiltinRole(types.RoleDiscovery) && len(cluster.GetDynamicLabels()) > 0 {
		return trace.AccessDenied("discovered kubernetes cluster must not have dynamic labels")
	}
	return trace.Wrap(a.authServer.CreateKubernetesCluster(ctx, cluster))
}

// UpdateKubernetesCluster updates existing kubernetes cluster resource.
func (a *ServerWithRoles) UpdateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	if err := a.action(apidefaults.Namespace, types.KindKubernetesCluster, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	// Don't allow users update clusters they don't have access to (e.g.
	// non-matching labels). Make sure to check existing cluster too.
	existing, err := a.authServer.GetKubernetesCluster(ctx, cluster.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkAccessToKubeCluster(existing); err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkAccessToKubeCluster(cluster); err != nil {
		return trace.Wrap(err)
	}
	// Don't allow discovery service to create clusters with dynamic labels.
	if a.hasBuiltinRole(types.RoleDiscovery) && len(cluster.GetDynamicLabels()) > 0 {
		return trace.AccessDenied("discovered kubernetes cluster must not have dynamic labels")
	}
	return trace.Wrap(a.authServer.UpdateKubernetesCluster(ctx, cluster))
}

// GetKubernetesCluster returns specified kubernetes cluster resource.
func (a *ServerWithRoles) GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error) {
	if err := a.action(apidefaults.Namespace, types.KindKubernetesCluster, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	kubeCluster, err := a.authServer.GetKubernetesCluster(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.checkAccessToKubeCluster(kubeCluster); err != nil {
		return nil, trace.Wrap(err)
	}
	return kubeCluster, nil
}

// GetKubernetesClusters returns all kubernetes cluster resources.
func (a *ServerWithRoles) GetKubernetesClusters(ctx context.Context) (result []types.KubeCluster, err error) {
	if err := a.action(apidefaults.Namespace, types.KindKubernetesCluster, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	// Filter out kube clusters user doesn't have access to.
	clusters, err := a.authServer.GetKubernetesClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, cluster := range clusters {
		if err := a.checkAccessToKubeCluster(cluster); err == nil {
			result = append(result, cluster)
		}
	}
	return result, nil
}

// DeleteKubernetesCluster removes the specified kubernetes cluster resource.
func (a *ServerWithRoles) DeleteKubernetesCluster(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindKubernetesCluster, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	// Make sure user has access to the kubernetes cluster before deleting.
	cluster, err := a.authServer.GetKubernetesCluster(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkAccessToKubeCluster(cluster); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.authServer.DeleteKubernetesCluster(ctx, name))
}

// DeleteAllKubernetesClusters removes all kubernetes cluster resources.
func (a *ServerWithRoles) DeleteAllKubernetesClusters(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindKubernetesCluster, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	// Make sure to only delete kubernetes cluster user has access to.
	clusters, err := a.authServer.GetKubernetesClusters(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, cluster := range clusters {
		if err := a.checkAccessToKubeCluster(cluster); err == nil {
			if err := a.authServer.DeleteKubernetesCluster(ctx, cluster.GetName()); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

func (a *ServerWithRoles) checkAccessToNode(node types.Server) error {
	// For certain built-in roles, continue to allow full access and return
	// the full set of nodes to not break existing clusters during migration.
	//
	// In addition, allow proxy (and remote proxy) to access all nodes for its
	// smart resolution address resolution. Once the smart resolution logic is
	// moved to the auth server, this logic can be removed.
	builtinRole := authz.HasBuiltinRole(a.context, string(types.RoleAdmin)) ||
		authz.HasBuiltinRole(a.context, string(types.RoleProxy)) ||
		HasRemoteBuiltinRole(a.context, string(types.RoleRemoteProxy))

	if builtinRole {
		return nil
	}

	return a.context.Checker.CheckAccess(node,
		// MFA is not required for operations on node resources but
		// will be enforced at the connection time.
		services.AccessState{MFAVerified: true})
}

func (a *ServerWithRoles) checkAccessToDatabase(database types.Database) error {
	return a.context.Checker.CheckAccess(database,
		// MFA is not required for operations on database resources but
		// will be enforced at the connection time.
		services.AccessState{MFAVerified: true})
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
	// Don't allow discovery service to create databases with dynamic labels.
	if a.hasBuiltinRole(types.RoleDiscovery) && len(database.GetDynamicLabels()) > 0 {
		return trace.AccessDenied("discovered database must not have dynamic labels")
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
	// Don't allow discovery service to create databases with dynamic labels.
	if a.hasBuiltinRole(types.RoleDiscovery) && len(database.GetDynamicLabels()) > 0 {
		return trace.AccessDenied("discovered database must not have dynamic labels")
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
		services.AccessState{MFAVerified: true},
		// Note: we don't use the Windows login matcher here, as we won't know what OS user
		// the user is trying to log in as until they initiate the connection.
	)
}

// GenerateWindowsDesktopCert generates a certificate for Windows RDP or SQL Server
// authentication.
func (a *ServerWithRoles) GenerateWindowsDesktopCert(ctx context.Context, req *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error) {
	// Only windows_desktop_service should be requesting Windows certificates.
	// (We also allow RoleAdmin for tctl auth sign)
	if !a.hasBuiltinRole(types.RoleWindowsDesktop, types.RoleAdmin) {
		return nil, trace.AccessDenied("access denied")
	}
	return a.authServer.GenerateWindowsDesktopCert(ctx, req)
}

func (a *ServerWithRoles) GetDesktopBootstrapScript(ctx context.Context) (*proto.DesktopBootstrapScriptResponse, error) {
	// No sensitive information is returned in the bootstrap script.
	return a.authServer.GetDesktopBootstrapScript(ctx)
}

// GetConnectionDiagnostic returns the connection diagnostic with the matching name
func (a *ServerWithRoles) GetConnectionDiagnostic(ctx context.Context, name string) (types.ConnectionDiagnostic, error) {
	if err := a.action(apidefaults.Namespace, types.KindConnectionDiagnostic, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	connectionsDiagnostic, err := a.authServer.GetConnectionDiagnostic(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connectionsDiagnostic, nil
}

// CreateConnectionDiagnostic creates a new connection diagnostic.
func (a *ServerWithRoles) CreateConnectionDiagnostic(ctx context.Context, connectionDiagnostic types.ConnectionDiagnostic) error {
	if err := a.action(apidefaults.Namespace, types.KindConnectionDiagnostic, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.CreateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UpdateConnectionDiagnostic updates a connection diagnostic.
func (a *ServerWithRoles) UpdateConnectionDiagnostic(ctx context.Context, connectionDiagnostic types.ConnectionDiagnostic) error {
	if err := a.action(apidefaults.Namespace, types.KindConnectionDiagnostic, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.UpdateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// AppendDiagnosticTrace adds a new trace for the given ConnectionDiagnostic.
func (a *ServerWithRoles) AppendDiagnosticTrace(ctx context.Context, name string, t *types.ConnectionDiagnosticTrace) (types.ConnectionDiagnostic, error) {
	if err := a.action(apidefaults.Namespace, types.KindConnectionDiagnostic, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.AppendDiagnosticTrace(ctx, name, t)
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
	isLocalOrRemoteUser := authz.IsLocalOrRemoteUser(a.context)

	// Run preliminary user checks first.
	switch req.GetRequest().(type) {
	case *proto.CreateAuthenticateChallengeRequest_UserCredentials:
	case *proto.CreateAuthenticateChallengeRequest_RecoveryStartTokenID:
	case *proto.CreateAuthenticateChallengeRequest_Passwordless:
	default: // nil or *proto.CreateAuthenticateChallengeRequest_ContextUser:
		if !isLocalOrRemoteUser {
			return nil, trace.BadParameter("only end users are allowed to issue authentication challenges using ContextUser")
		}
	}

	// Have we been asked to check if MFA is necessary? Resolve that first.
	//
	// We run the check in this layer, instead of under Server.IsMFARequired,
	// because the ServerWithRoles.IsMFARequired variant adds logic of its own.
	var mfaRequired proto.MFARequired
	if req.MFARequiredCheck != nil {
		// Return a nicer error message.
		if !isLocalOrRemoteUser {
			return nil, trace.BadParameter("only end users are allowed to supply MFARequiredCheck")
		}

		mfaRequiredResp, err := a.IsMFARequired(ctx, req.MFARequiredCheck)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Exit early if we are certain that MFA is not necessary.
		if !mfaRequiredResp.Required {
			return &proto.MFAAuthenticateChallenge{
				// No challenges provided.
				MFARequired: mfaRequiredResp.MFARequired,
			}, nil
		}
		mfaRequired = mfaRequiredResp.MFARequired
	}

	// The following serve as means of authentication for this RPC:
	//   - username + password, anyone who has user's password can generate a sign request
	//   - token provide its own auth
	//   - the user extracted from context can create their own challenges
	authnChal, err := a.authServer.CreateAuthenticateChallenge(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set MFA requirement queried above, if any.
	authnChal.MFARequired = mfaRequired
	return authnChal, nil
}

// CreatePrivilegeToken is implemented by AuthService.CreatePrivilegeToken.
func (a *ServerWithRoles) CreatePrivilegeToken(ctx context.Context, req *proto.CreatePrivilegeTokenRequest) (*types.UserTokenV3, error) {
	// Device trust: authorize device before issuing a privileged token without an MFA response.
	//
	// This is an exceptional case for that that results in a "privilege_exception" token, which can
	// used to register a user's first MFA device thorugh the WebUI. Since a register challenge can
	// be created on behalf of the user using this token (e.g. by the Proxy Service), we must enforce
	// the device trust requirement seen in [CreatePrivilegeToken] here instead.
	if mfaResp := req.GetExistingMFAResponse(); mfaResp.GetTOTP() == nil && mfaResp.GetWebauthn() == nil {
		if err := a.enforceGlobalModeTrustedDevice(ctx); err != nil {
			return nil, trace.Wrap(err, "device trust is required for users to create a privileged token without an MFA check")
		}
	}

	return a.authServer.CreatePrivilegeToken(ctx, req)
}

// CreateRegisterChallenge is implemented by AuthService.CreateRegisterChallenge.
func (a *ServerWithRoles) CreateRegisterChallenge(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
	if req.TokenID == "" {
		if !authz.IsLocalOrRemoteUser(a.context) {
			return nil, trace.BadParameter("only end users are allowed issue registration challenges without a privilege token")
		}

		// Device trust: authorize device before issuing a register challenge without an MFA response or privilege token.
		// This is an exceptional case for users registering their first MFA challenge through `tsh`.
		if mfaResp := req.GetExistingMFAResponse(); mfaResp.GetTOTP() == nil && mfaResp.GetWebauthn() == nil {
			if err := a.enforceGlobalModeTrustedDevice(ctx); err != nil {
				return nil, trace.Wrap(err, "device trust is required for users to register their first MFA device")
			}
		}
	}

	// The following serve as means of authentication for this RPC:
	//   - privilege token (or equivalent)
	//   - authenticated user using non-Proxy identity
	return a.authServer.CreateRegisterChallenge(ctx, req)
}

// enforceGlobalModeTrustedDevice is used to enforce global device trust requirements
// for key endpoints.
func (a *ServerWithRoles) enforceGlobalModeTrustedDevice(ctx context.Context) error {
	readOnlyAuthPref, err := a.authServer.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = dtauthz.VerifyTLSUser(readOnlyAuthPref.GetDeviceTrust(), a.context.Identity.GetIdentity())
	return trace.Wrap(err)
}

// GetAccountRecoveryCodes is implemented by AuthService.GetAccountRecoveryCodes.
func (a *ServerWithRoles) GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
	// User in context can retrieve their own recovery codes.
	return a.authServer.GetAccountRecoveryCodes(ctx, req)
}

// GenerateCertAuthorityCRL generates an empty CRL for a CA.
//
// This CRL can be requested by:
//
//   - Windows desktop service when updating the certificate authority contents
//     on LDAP.
//   - Cluster administrator using "tctl auth crl --type=db_client" command locally
//     on the auth server to produce revocation list used to be configured on
//     external services such as Windows certificate store.
//   - Remote user using "tctl auth crl --type=db_client" command with a remote
//     proxy (e.g. Teleport Cloud), as long as they have permission to read
//     certificate authorities.
func (a *ServerWithRoles) GenerateCertAuthorityCRL(ctx context.Context, caType types.CertAuthType) ([]byte, error) {
	// Assume this is a user request, check if the user has permission to read CAs.
	err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbReadNoSecrets)
	if err != nil {
		// An error means the user doesn't have permission to read CAs, or this
		// is an admin on the auth server or the windows desktop service. We
		// expect to see an access denied error in any of those cases.
		if !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}

		// If this is an admin on the auth server (types.RoleAdmin) or the
		// windows desktop service (types.RoleWindowsDesktop), allow the
		// request. Otherwise, return the access denied error.
		if !a.hasBuiltinRole(types.RoleAdmin, types.RoleWindowsDesktop) {
			return nil, trace.AccessDenied("access denied")
		}
	}

	crl, err := a.authServer.GenerateCertAuthorityCRL(ctx, caType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return crl, nil
}

// SubmitUsageEvent submits an external usage event.
func (a *ServerWithRoles) SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error {
	if err := a.action(apidefaults.Namespace, types.KindUsageEvent, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.SubmitUsageEvent(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetLicense returns the license used to start the auth server
func (a *ServerWithRoles) GetLicense(ctx context.Context) (string, error) {
	if err := a.action(apidefaults.Namespace, types.KindLicense, types.VerbRead); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.GetLicense(ctx)
}

// ListReleases return Teleport Enterprise releases
func (a *ServerWithRoles) ListReleases(ctx context.Context) ([]*types.Release, error) {
	// on Cloud, any user is allowed to list releases
	if !modules.GetModules().Features().Cloud {
		if err := a.action(apidefaults.Namespace, types.KindDownload, types.VerbList); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return a.authServer.releaseService.ListReleases(ctx)
}

// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
func (a *ServerWithRoles) ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, nextToken string) ([]types.SAMLIdPServiceProvider, string, error) {
	if err := a.action(apidefaults.Namespace, types.KindSAMLIdPServiceProvider, types.VerbList); err != nil {
		return nil, "", trace.Wrap(err)
	}

	return a.authServer.ListSAMLIdPServiceProviders(ctx, pageSize, nextToken)
}

// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
func (a *ServerWithRoles) GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	if err := a.action(apidefaults.Namespace, types.KindSAMLIdPServiceProvider, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetSAMLIdPServiceProvider(ctx, name)
}

// CreateSAMLIdPServiceProvider creates a new SAML IdP service provider resource.
func (a *ServerWithRoles) CreateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) (err error) {
	defer func() {
		code := events.SAMLIdPServiceProviderCreateCode
		if err != nil {
			code = events.SAMLIdPServiceProviderCreateFailureCode
		}

		if emitErr := a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &apievents.SAMLIdPServiceProviderCreate{
			Metadata: apievents.Metadata{
				Type: events.SAMLIdPServiceProviderCreateEvent,
				Code: code,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				Name:      sp.GetName(),
				UpdatedBy: authz.ClientUsername(ctx),
			},
			SAMLIdPServiceProviderMetadata: apievents.SAMLIdPServiceProviderMetadata{
				ServiceProviderEntityID: sp.GetEntityID(),
				AttributeMapping:        typesAttrMapToEventAttrMap(sp.GetAttributeMapping()),
			},
		}); emitErr != nil {
			log.WithError(trace.NewAggregate(emitErr, err)).Warn("Failed to emit SAML IdP service provider created event.")
		}
	}()

	if err = a.action(apidefaults.Namespace, types.KindSAMLIdPServiceProvider, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err = a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}

	if err := services.ValidateSAMLIdPACSURLAndRelayStateInputs(sp); err != nil {
		return trace.Wrap(err)
	}

	if sp.GetEntityDescriptor() != "" {
		if err := services.ValidateAndFilterEntityDescriptor(sp, services.SAMLACSInputStrictFilter); err != nil {
			return trace.Wrap(err)
		}
	}

	err = a.authServer.CreateSAMLIdPServiceProvider(ctx, sp)
	return trace.Wrap(err)
}

// UpdateSAMLIdPServiceProvider updates an existing SAML IdP service provider resource.
func (a *ServerWithRoles) UpdateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) (err error) {
	defer func() {
		code := events.SAMLIdPServiceProviderUpdateCode
		if err != nil {
			code = events.SAMLIdPServiceProviderUpdateFailureCode
		}

		if emitErr := a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &apievents.SAMLIdPServiceProviderUpdate{
			Metadata: apievents.Metadata{
				Type: events.SAMLIdPServiceProviderUpdateEvent,
				Code: code,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				Name:      sp.GetName(),
				UpdatedBy: authz.ClientUsername(ctx),
			},
			SAMLIdPServiceProviderMetadata: apievents.SAMLIdPServiceProviderMetadata{
				ServiceProviderEntityID: sp.GetEntityID(),
				AttributeMapping:        typesAttrMapToEventAttrMap(sp.GetAttributeMapping()),
			},
		}); emitErr != nil {
			log.WithError(trace.NewAggregate(emitErr, err)).Warn("Failed to emit SAML IdP service provider updated event.")
		}
	}()

	if err := a.action(apidefaults.Namespace, types.KindSAMLIdPServiceProvider, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := a.context.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}

	if err := services.ValidateSAMLIdPACSURLAndRelayStateInputs(sp); err != nil {
		return trace.Wrap(err)
	}

	if err := services.ValidateAndFilterEntityDescriptor(sp, services.SAMLACSInputStrictFilter); err != nil {
		return trace.Wrap(err)
	}

	err = a.authServer.UpdateSAMLIdPServiceProvider(ctx, sp)
	return trace.Wrap(err)
}

func typesAttrMapToEventAttrMap(attributeMapping []*types.SAMLAttributeMapping) map[string]string {
	amap := make(map[string]string, len(attributeMapping))
	for _, attribute := range attributeMapping {
		amap[attribute.Name] = attribute.Value
	}
	return amap
}

// DeleteSAMLIdPServiceProvider removes the specified SAML IdP service provider resource.
func (a *ServerWithRoles) DeleteSAMLIdPServiceProvider(ctx context.Context, name string) (err error) {
	var entityID string
	defer func() {
		code := events.SAMLIdPServiceProviderDeleteCode
		if err != nil {
			code = events.SAMLIdPServiceProviderDeleteFailureCode
		}

		if emitErr := a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &apievents.SAMLIdPServiceProviderDelete{
			Metadata: apievents.Metadata{
				Type: events.SAMLIdPServiceProviderDeleteEvent,
				Code: code,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				Name:      name,
				UpdatedBy: authz.ClientUsername(ctx),
			},
			SAMLIdPServiceProviderMetadata: apievents.SAMLIdPServiceProviderMetadata{
				ServiceProviderEntityID: entityID,
			},
		}); emitErr != nil {
			log.WithError(trace.NewAggregate(emitErr, err)).Warn("Failed to emit SAML IdP service provider deleted event.")
		}
	}()

	if err := a.action(apidefaults.Namespace, types.KindSAMLIdPServiceProvider, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	// Get the service provider so we can emit its entity ID later.
	sp, err := a.authServer.GetSAMLIdPServiceProvider(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}

	name = sp.GetName()
	entityID = sp.GetEntityID()

	// Delete the actual service provider.
	err = a.authServer.DeleteSAMLIdPServiceProvider(ctx, name)
	return trace.Wrap(err)
}

// DeleteAllSAMLIdPServiceProviders removes all SAML IdP service providers.
func (a *ServerWithRoles) DeleteAllSAMLIdPServiceProviders(ctx context.Context) (err error) {
	defer func() {
		code := events.SAMLIdPServiceProviderDeleteAllCode
		if err != nil {
			code = events.SAMLIdPServiceProviderDeleteAllFailureCode
		}

		if emitErr := a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &apievents.SAMLIdPServiceProviderDeleteAll{
			Metadata: apievents.Metadata{
				Type: events.SAMLIdPServiceProviderDeleteAllEvent,
				Code: code,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				UpdatedBy: authz.ClientUsername(ctx),
			},
		}); emitErr != nil {
			log.WithError(trace.NewAggregate(emitErr, err)).Warn("Failed to emit SAML IdP service provider deleted all event.")
		}
	}()

	if err := a.action(apidefaults.Namespace, types.KindSAMLIdPServiceProvider, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	err = a.authServer.DeleteAllSAMLIdPServiceProviders(ctx)
	return trace.Wrap(err)
}

func (a *ServerWithRoles) checkAccessToUserGroup(userGroup types.UserGroup) error {
	return a.context.Checker.CheckAccess(
		userGroup,
		// MFA is not required for operations on user group resources.
		services.AccessState{MFAVerified: true})
}

// ListUserGroups returns a paginated list of user group resources.
func (a *ServerWithRoles) ListUserGroups(ctx context.Context, pageSize int, nextToken string) ([]types.UserGroup, string, error) {
	if err := a.action(apidefaults.Namespace, types.KindUserGroup, types.VerbList); err != nil {
		return nil, "", trace.Wrap(err)
	}

	// We have to set a default here.
	if pageSize == 0 {
		pageSize = local.GroupMaxPageSize
	}

	// Because access to user groups is determined by label, we'll need to calculate the entire list of
	// user groups and then check access to those user groups.
	var filteredUserGroups []types.UserGroup

	// Use the default page size since we're assembling our pages manually here.
	userGroups, nextToken, err := a.authServer.ListUserGroups(ctx, 0, nextToken)
	for {
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		for _, userGroup := range userGroups {
			err := a.checkAccessToUserGroup(userGroup)
			if err != nil && !trace.IsAccessDenied(err) {
				return nil, "", trace.Wrap(err)
			} else if err == nil {
				filteredUserGroups = append(filteredUserGroups, userGroup)
			}
		}

		if nextToken == "" {
			break
		}

		userGroups, nextToken, err = a.authServer.ListUserGroups(ctx, 0, nextToken)
	}

	numUserGroups := len(filteredUserGroups)
	if numUserGroups <= pageSize {
		return filteredUserGroups, "", nil
	}

	return filteredUserGroups[:pageSize], backend.NextPaginationKey(filteredUserGroups[pageSize-1]), nil
}

// GetUserGroup returns the specified user group resources.
func (a *ServerWithRoles) GetUserGroup(ctx context.Context, name string) (types.UserGroup, error) {
	if err := a.action(apidefaults.Namespace, types.KindUserGroup, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	userGroup, err := a.authServer.GetUserGroup(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.checkAccessToUserGroup(userGroup); err != nil {
		return nil, trace.Wrap(err)
	}

	return userGroup, nil
}

// CreateUserGroup creates a new user group resource.
func (a *ServerWithRoles) CreateUserGroup(ctx context.Context, userGroup types.UserGroup) error {
	if err := a.action(apidefaults.Namespace, types.KindUserGroup, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	if err := a.checkAccessToUserGroup(userGroup); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.CreateUserGroup(ctx, userGroup)
}

// UpdateUserGroup updates an existing user group resource.
func (a *ServerWithRoles) UpdateUserGroup(ctx context.Context, userGroup types.UserGroup) error {
	if err := a.action(apidefaults.Namespace, types.KindUserGroup, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	previousUserGroup, err := a.authServer.GetUserGroup(ctx, userGroup.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.checkAccessToUserGroup(previousUserGroup); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.UpdateUserGroup(ctx, userGroup)
}

// DeleteUserGroup removes the specified user group resource.
func (a *ServerWithRoles) DeleteUserGroup(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindUserGroup, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	previousUserGroup, err := a.authServer.GetUserGroup(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.checkAccessToUserGroup(previousUserGroup); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteUserGroup(ctx, name)
}

// DeleteAllUserGroups removes all user groups.
func (a *ServerWithRoles) DeleteAllUserGroups(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindUserGroup, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.context.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteAllUserGroups(ctx)
}

// GetHeadlessAuthentication gets a headless authentication from the backend.
func (a *ServerWithRoles) GetHeadlessAuthentication(ctx context.Context, name string) (*types.HeadlessAuthentication, error) {
	if !authz.IsLocalUser(a.context) {
		return nil, trace.AccessDenied("non-local user roles cannot get headless authentication resources")
	}
	username := a.context.User.GetName()

	headlessAuthn, err := a.authServer.GetHeadlessAuthentication(ctx, username, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return headlessAuthn, nil
}

// GetHeadlessAuthenticationFromWatcher gets a headless authentication from the headless
// authentication watcher.
func (a *ServerWithRoles) GetHeadlessAuthenticationFromWatcher(ctx context.Context, name string) (*types.HeadlessAuthentication, error) {
	if !authz.IsLocalUser(a.context) {
		return nil, trace.AccessDenied("non-local user roles cannot get headless authentication resources")
	}
	username := a.context.User.GetName()

	headlessAuthn, err := a.authServer.GetHeadlessAuthenticationFromWatcher(ctx, username, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return headlessAuthn, nil
}

// UpsertHeadlessAuthenticationStub creates a headless authentication stub for the user
// that will expire after the standard callback timeout. Headless login processes will
// look for this stub before inserting the headless authentication resource into the
// backend as a form of indirect authorization.
func (a *ServerWithRoles) UpsertHeadlessAuthenticationStub(ctx context.Context) error {
	if !authz.IsLocalUser(a.context) {
		return trace.AccessDenied("non-local user roles cannot create headless authentication stubs")
	}
	username := a.context.User.GetName()

	err := a.authServer.UpsertHeadlessAuthenticationStub(ctx, username)
	return trace.Wrap(err)
}

// UpdateHeadlessAuthenticationState updates a headless authentication state.
func (a *ServerWithRoles) UpdateHeadlessAuthenticationState(ctx context.Context, name string, state types.HeadlessAuthenticationState, mfaResp *proto.MFAAuthenticateResponse) error {
	if !authz.IsLocalUser(a.context) {
		return trace.AccessDenied("non-local user roles cannot approve or deny headless authentication resources")
	}
	username := a.context.User.GetName()

	headlessAuthn, err := a.authServer.GetHeadlessAuthentication(ctx, username, name)
	if err != nil {
		return trace.Wrap(err)
	}

	if !headlessAuthn.State.IsPending() {
		return trace.AccessDenied("cannot update a headless authentication state from a non-pending state")
	}

	// Shallow copy headless authn for compare and swap below.
	replaceHeadlessAuthn := *headlessAuthn
	replaceHeadlessAuthn.State = state

	eventCode := ""
	switch state {
	case types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED:
		// The user must authenticate with MFA to change the state to approved.
		if mfaResp == nil {
			err = trace.BadParameter("expected MFA auth challenge response")
			emitHeadlessLoginEvent(ctx, events.UserHeadlessLoginApprovedFailureCode, a.authServer.emitter, headlessAuthn, err)
			return err
		}

		// Only WebAuthn is supported in headless login flow for superior phishing prevention.
		if _, ok := mfaResp.Response.(*proto.MFAAuthenticateResponse_Webauthn); !ok {
			err = trace.BadParameter("expected WebAuthn challenge response, but got %T", mfaResp.Response)
			emitHeadlessLoginEvent(ctx, events.UserHeadlessLoginApprovedFailureCode, a.authServer.emitter, headlessAuthn, err)
			return err
		}

		requiredExt := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_HEADLESS_LOGIN}
		mfaData, err := a.authServer.ValidateMFAAuthResponse(ctx, mfaResp, headlessAuthn.User, requiredExt)
		if err != nil {
			emitHeadlessLoginEvent(ctx, events.UserHeadlessLoginApprovedFailureCode, a.authServer.emitter, headlessAuthn, err)
			return trace.Wrap(err)
		}

		replaceHeadlessAuthn.MfaDevice = mfaData.Device
		eventCode = events.UserHeadlessLoginApprovedCode
	case types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED:
		eventCode = events.UserHeadlessLoginRejectedCode
		// continue to compare and swap without MFA.
	default:
		return trace.AccessDenied("cannot update a headless authentication state to %v", state.String())
	}

	_, err = a.authServer.CompareAndSwapHeadlessAuthentication(ctx, headlessAuthn, &replaceHeadlessAuthn)
	if err != nil && eventCode == events.UserHeadlessLoginApprovedCode {
		eventCode = events.UserHeadlessLoginApprovedFailureCode
	}
	emitHeadlessLoginEvent(ctx, eventCode, a.authServer.emitter, headlessAuthn, err)
	return trace.Wrap(err)
}

// MaintainHeadlessAuthenticationStub maintains a headless authentication stub for the user.
// Headless login processes will look for this stub before inserting the headless authentication
// resource into the backend as a form of indirect authorization.
func (a *ServerWithRoles) MaintainHeadlessAuthenticationStub(ctx context.Context) error {
	if !authz.IsLocalUser(a.context) {
		return trace.AccessDenied("non-local user roles cannot create headless authentication stubs")
	}
	username := a.context.User.GetName()

	// Create a stub and re-create it each time it expires.
	// Authorization is handled by UpsertHeadlessAuthenticationStub.
	if err := a.authServer.UpsertHeadlessAuthenticationStub(ctx, username); err != nil {
		return trace.Wrap(err)
	}

	ticker := time.NewTicker(defaults.HeadlessLoginTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := a.authServer.UpsertHeadlessAuthenticationStub(ctx, username); err != nil {
				return trace.Wrap(err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// WatchPendingHeadlessAuthentications creates a watcher for pending headless authentication for the current user.
func (a *ServerWithRoles) WatchPendingHeadlessAuthentications(ctx context.Context) (types.Watcher, error) {
	if !authz.IsLocalUser(a.context) {
		return nil, trace.AccessDenied("non-local user roles cannot watch headless authentications")
	}
	username := a.context.User.GetName()

	// Authorization is handled by NewWatcher.
	filter := types.HeadlessAuthenticationFilter{
		Username: username,
		State:    types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING,
	}

	return a.NewWatcher(ctx, types.Watch{
		Name: username,
		Kinds: []types.WatchKind{{
			Kind:   types.KindHeadlessAuthentication,
			Filter: filter.IntoMap(),
		}},
	})
}

// ExportUpgradeWindows is used to load derived upgrade window values for agents that
// need to export schedules to external upgraders.
func (a *ServerWithRoles) ExportUpgradeWindows(ctx context.Context, req proto.ExportUpgradeWindowsRequest) (proto.ExportUpgradeWindowsResponse, error) {
	// Ensure that caller is a teleport server
	role, ok := a.context.Identity.(authz.BuiltinRole)
	if !ok || !role.IsServer() {
		return proto.ExportUpgradeWindowsResponse{}, trace.AccessDenied("agent maintenance schedule is only accessible to teleport built-in servers")
	}

	return a.authServer.ExportUpgradeWindows(ctx, req)
}

// GetClusterMaintenanceConfig gets the current maintenance config singleton.
func (a *ServerWithRoles) GetClusterMaintenanceConfig(ctx context.Context) (types.ClusterMaintenanceConfig, error) {
	if err := a.action(apidefaults.Namespace, types.KindClusterMaintenanceConfig, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetClusterMaintenanceConfig(ctx)
}

// UpdateClusterMaintenanceConfig updates the current maintenance config singleton.
func (a *ServerWithRoles) UpdateClusterMaintenanceConfig(ctx context.Context, cmc types.ClusterMaintenanceConfig) error {
	if err := a.action(apidefaults.Namespace, types.KindClusterMaintenanceConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	if modules.GetModules().Features().Cloud {
		// maintenance configuration in cloud is derived from values stored in
		// an external cloud-specific database.
		return trace.NotImplemented("cloud clusters do not support custom cluster maintenance resources")
	}

	return a.authServer.UpdateClusterMaintenanceConfig(ctx, cmc)
}

func (a *ServerWithRoles) DeleteClusterMaintenanceConfig(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindClusterMaintenanceConfig, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	if modules.GetModules().Features().Cloud {
		// maintenance configuration in cloud is derived from values stored in
		// an external cloud-specific database.
		return trace.NotImplemented("cloud clusters do not support custom cluster maintenance resources")
	}

	return a.authServer.DeleteClusterMaintenanceConfig(ctx)
}

func emitHeadlessLoginEvent(ctx context.Context, code string, emitter apievents.Emitter, headlessAuthn *types.HeadlessAuthentication, err error) {
	clientAddr := ""
	if code == events.UserHeadlessLoginRequestedCode {
		clientAddr = headlessAuthn.ClientIpAddress
	} else if c, err := authz.ClientSrcAddrFromContext(ctx); err == nil {
		clientAddr = c.String()
	}

	message := ""
	if code != events.UserHeadlessLoginRequestedCode {
		// For events.UserHeadlessLoginRequestedCode remote.addr will be the IP of requester.
		// For other events that IP will be different because user will be approving the request from another machine,
		// so we mentioned requester IP in the message.
		message = fmt.Sprintf("Headless login was requested from the address %s", headlessAuthn.ClientIpAddress)
	}
	errorMessage := ""
	if err != nil {
		errorMessage = trace.Unwrap(err).Error()
	}
	event := apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: code,
		},
		Method: events.LoginMethodHeadless,
		UserMetadata: apievents.UserMetadata{
			User: headlessAuthn.User,
		},
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: clientAddr,
		},
		Status: apievents.Status{
			Success:     code == events.UserHeadlessLoginApprovedCode,
			UserMessage: message,
			Error:       errorMessage,
		},
	}

	if emitErr := emitter.EmitAuditEvent(ctx, &event); emitErr != nil {
		log.WithError(err).Warnf("Failed to emit %q login event, code %q: %v", events.LoginMethodHeadless, code, emitErr)
	}
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
		log.WithError(err).Warnf("Failed to emit %v login failure event: %v", method, emitErr)
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

// checkOktaUserAccess gates access to update operations on user records based
// on the origin label on the supplied user record.
//
// # See okta.CheckAccess() for the actual access rules
//
// TODO(tcsc): Delete in 16.0.0 when user management is removed from `ServerWithRoles`
func checkOktaUserAccess(ctx context.Context, authzCtx *authz.Context, users services.UsersService, existingUsername string, verb string) error {
	existingUser, err := users.GetUser(ctx, existingUsername, false)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return okta.CheckAccess(authzCtx, existingUser, verb)
}

// checkOktaLockTarget prevents the okta service from locking users that are not
// controlled by the Okta service.
func checkOktaLockTarget(ctx context.Context, authzCtx *authz.Context, users services.UserGetter, lock types.Lock) error {
	const errorMsg = "Okta service may only lock okta user"

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleOkta)) {
		return nil
	}

	target := lock.Target()
	switch {
	case !target.Equals(types.LockTarget{User: target.User}):
		return trace.BadParameter(errorMsg)

	case target.User == "":
		return trace.BadParameter(errorMsg)
	}

	targetUser, err := users.GetUser(ctx, target.User, false /* withSecrets */)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.AccessDenied(errorMsg)
		}
		return trace.Wrap(err)
	}

	if targetUser.Origin() != types.OriginOkta {
		return trace.AccessDenied(errorMsg)
	}

	return nil
}

// checkOktaLockAccess gates access to update operations on lock records based
// on the origin label on the supplied user record.
func checkOktaLockAccess(ctx context.Context, authzCtx *authz.Context, locks services.LockGetter, existingLockName string, verb string) error {
	existingLock, err := locks.GetLock(ctx, existingLockName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return okta.CheckAccess(authzCtx, existingLock, verb)
}
