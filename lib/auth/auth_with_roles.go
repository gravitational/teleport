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

	"github.com/gravitational/teleport"
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
	if hasLocalUserRole(a.context.Checker) && username == a.context.User.GetName() {
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

// hasBuiltinRole checks the type of the role set returned and the name.
// Returns true if role set is builtin and the name matches.
func (a *ServerWithRoles) hasBuiltinRole(name string) bool {
	return HasBuiltinRole(a.context.Checker, name)
}

// HasBuiltinRole checks the type of the role set returned and the name.
// Returns true if role set is builtin and the name matches.
func HasBuiltinRole(checker services.AccessChecker, name string) bool {
	if _, ok := checker.(BuiltinRoleSet); !ok {
		return false
	}
	if !checker.HasRole(name) {
		return false
	}

	return true
}

// hasRemoteBuiltinRole checks the type of the role set returned and the name.
// Returns true if role set is remote builtin and the name matches.
func (a *ServerWithRoles) hasRemoteBuiltinRole(name string) bool {
	if _, ok := a.context.Checker.(RemoteBuiltinRoleSet); !ok {
		return false
	}
	if !a.context.Checker.HasRole(name) {
		return false
	}
	return true
}

// hasRemoteUserRole checks if the type of the role set is a remote user or
// not.
func hasRemoteUserRole(checker services.AccessChecker) bool {
	_, ok := checker.(RemoteUserRoleSet)
	return ok
}

// hasLocalUserRole checks if the type of the role set is a local user or not.
func hasLocalUserRole(checker services.AccessChecker) bool {
	_, ok := checker.(LocalUserRoleSet)
	return ok
}

// AuthenticateWebUser authenticates web user, creates and returns a web session
// in case authentication is successful
func (a *ServerWithRoles) AuthenticateWebUser(req AuthenticateUserRequest) (types.WebSession, error) {
	// authentication request has it's own authentication, however this limits the requests
	// types to proxies to make it harder to break
	if !a.hasBuiltinRole(string(types.RoleProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.AuthenticateWebUser(req)
}

// AuthenticateSSHUser authenticates SSH console user, creates and  returns a pair of signed TLS and SSH
// short lived certificates as a result
func (a *ServerWithRoles) AuthenticateSSHUser(req AuthenticateSSHRequest) (*SSHLoginResponse, error) {
	// authentication request has it's own authentication, however this limits the requests
	// types to proxies to make it harder to break
	if !a.hasBuiltinRole(string(types.RoleProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.AuthenticateSSHUser(req)
}

func (a *ServerWithRoles) GetSessions(namespace string) ([]session.Session, error) {
	if err := a.action(namespace, types.KindSSHSession, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.sessions.GetSessions(namespace)
}

func (a *ServerWithRoles) GetSession(namespace string, id session.ID) (*session.Session, error) {
	if err := a.action(namespace, types.KindSSHSession, types.VerbRead); err != nil {
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
	if err := a.action(req.Namespace, types.KindSSHSession, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.UpdateSession(req)
}

// DeleteSession removes an active session from the backend.
func (a *ServerWithRoles) DeleteSession(namespace string, id session.ID) error {
	if err := a.action(namespace, types.KindSSHSession, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.DeleteSession(namespace, id)
}

// CreateCertAuthority not implemented: can only be called locally.
func (a *ServerWithRoles) CreateCertAuthority(ca types.CertAuthority) error {
	return trace.NotImplemented(notImplementedMessage)
}

// RotateCertAuthority starts or restarts certificate authority rotation process.
func (a *ServerWithRoles) RotateCertAuthority(req RotateRequest) error {
	if err := req.CheckAndSetDefaults(a.authServer.clock); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.RotateCertAuthority(req)
}

// RotateExternalCertAuthority rotates external certificate authority,
// this method is called by a remote trusted cluster and is used to update
// only public keys and certificates of the certificate authority.
func (a *ServerWithRoles) RotateExternalCertAuthority(ca types.CertAuthority) error {
	if ca == nil {
		return trace.BadParameter("missing certificate authority")
	}
	ctx := &services.Context{User: a.context.User, Resource: ca}
	if err := a.actionWithContext(ctx, apidefaults.Namespace, types.KindCertAuthority, types.VerbRotate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.RotateExternalCertAuthority(ca)
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

func (a *ServerWithRoles) GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error) {
	if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbList, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if loadKeys {
		if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetCertAuthorities(caType, loadKeys, opts...)
}

func (a *ServerWithRoles) GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
	if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if loadKeys {
		if err := a.action(apidefaults.Namespace, types.KindCertAuthority, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetCertAuthority(id, loadKeys, opts...)
}

func (a *ServerWithRoles) GetDomainName() (string, error) {
	// anyone can read it, no harm in that
	return a.authServer.GetDomainName()
}

func (a *ServerWithRoles) GetLocalClusterName() (string, error) {
	// anyone can read it, no harm in that
	return a.authServer.GetLocalClusterName()
}

// getClusterCACert returns the PEM-encoded TLS certs for the local cluster
// without signing keys. If the cluster has multiple TLS certs, they will all
// be concatenated.
func (a *ServerWithRoles) GetClusterCACert() (*LocalCAResponse, error) {
	// Allow all roles to get the CA certs.
	return a.authServer.GetClusterCACert()
}

func (a *ServerWithRoles) UpsertLocalClusterName(clusterName string) error {
	if err := a.action(apidefaults.Namespace, types.KindAuthServer, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertLocalClusterName(clusterName)
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
func (a *ServerWithRoles) GenerateToken(ctx context.Context, req GenerateTokenRequest) (string, error) {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.GenerateToken(ctx, req)
}

func (a *ServerWithRoles) RegisterUsingToken(req RegisterUsingTokenRequest) (*proto.Certs, error) {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterUsingToken(req)
}

func (a *ServerWithRoles) RegisterNewAuthServer(ctx context.Context, token string) error {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterNewAuthServer(ctx, token)
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
	existingRoles, err := types.NewTeleportRoles(a.context.User.GetRoles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// prohibit privilege escalations through role changes
	if !existingRoles.Equals(types.SystemRoles{req.Role}) {
		return nil, trace.AccessDenied("roles do not match: %v and %v", existingRoles, req.Role)
	}
	return a.authServer.GenerateHostCerts(ctx, req)
}

// UpsertNodes bulk upserts nodes into the backend.
func (a *ServerWithRoles) UpsertNodes(namespace string, servers []types.Server) error {
	if err := a.action(namespace, types.KindNode, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertNodes(namespace, servers)
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
	if !a.hasBuiltinRole(string(types.RoleNode)) {
		return trace.AccessDenied("[10] access denied")
	}
	clusterName, err := a.GetDomainName()
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
	clusterName, err := a.GetDomainName()
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
		if !a.hasBuiltinRole(string(types.RoleNode)) {
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
		if !a.hasBuiltinRole(string(types.RoleApp)) {
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
		if !a.hasBuiltinRole(string(types.RoleDatabase)) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(apidefaults.Namespace, types.KindDatabaseServer, types.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	case constants.KeepAliveWindowsDesktopService:
		if serverName != handle.Name {
			return trace.AccessDenied("access denied")
		}
		if !a.hasBuiltinRole(string(types.RoleWindowsDesktop)) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(apidefaults.Namespace, types.KindWindowsDesktopService, types.VerbUpdate); err != nil {
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
			if filter.User == "" || a.currentUserAction(filter.User) != nil {
				if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbRead); err != nil {
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
		default:
			if err := a.action(apidefaults.Namespace, kind.Kind, types.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	switch {
	case a.hasBuiltinRole(string(types.RoleProxy)):
		watch.QueueSize = defaults.ProxyQueueSize
	case a.hasBuiltinRole(string(types.RoleNode)):
		watch.QueueSize = defaults.NodeQueueSize
	}
	return a.authServer.NewWatcher(ctx, watch)
}

// filterNodes filters nodes based off the role of the logged in user.
func (a *ServerWithRoles) filterNodes(nodes []types.Server) ([]types.Server, error) {
	// For certain built-in roles, continue to allow full access and return
	// the full set of nodes to not break existing clusters during migration.
	//
	// In addition, allow proxy (and remote proxy) to access all nodes for it's
	// smart resolution address resolution. Once the smart resolution logic is
	// moved to the auth server, this logic can be removed.
	if a.hasBuiltinRole(string(types.RoleAdmin)) ||
		a.hasBuiltinRole(string(types.RoleProxy)) ||
		a.hasRemoteBuiltinRole(string(types.RoleRemoteProxy)) {
		return nodes, nil
	}

	roleset, err := services.FetchRoles(a.context.User.GetRoles(), a.authServer, a.context.User.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract all unique allowed logins across all roles.
	allowedLogins := make(map[string]bool)
	for _, role := range roleset {
		for _, login := range role.GetLogins(services.Allow) {
			allowedLogins[login] = true
		}
	}

	// Loop over all nodes and check if the caller has access.
	filteredNodes := make([]types.Server, 0, len(nodes))
	// MFA is not required to list the nodes, but will be required to connect
	// to them.
	mfaParams := services.AccessMFAParams{Verified: true}
NextNode:
	for _, node := range nodes {
		for login := range allowedLogins {
			err := roleset.CheckAccessToServer(login, node, mfaParams)
			if err == nil {
				filteredNodes = append(filteredNodes, node)
				continue NextNode
			}
		}
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

	// Run node through filter to check if it's for the connected identity.
	if filteredNodes, err := a.filterNodes([]types.Server{node}); err != nil {
		return nil, trace.Wrap(err)
	} else if len(filteredNodes) == 0 {
		return nil, trace.NotFound("not found")
	}

	return node, nil
}

func (a *ServerWithRoles) GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	if err := a.action(namespace, types.KindNode, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch full list of nodes in the backend.
	startFetch := time.Now()
	nodes, err := a.authServer.GetNodes(ctx, namespace, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	elapsedFetch := time.Since(startFetch)

	// Filter nodes to return the ones for the connected identity.
	startFilter := time.Now()
	filteredNodes, err := a.filterNodes(nodes)
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

// ListNodes returns a paginated list of nodes filtered by user access.
func (a *ServerWithRoles) ListNodes(ctx context.Context, namespace string, limit int, startKey string) (page []types.Server, nextKey string, err error) {
	if err := a.action(namespace, types.KindNode, types.VerbList); err != nil {
		return nil, "", trace.Wrap(err)
	}

	return a.filterAndListNodes(ctx, namespace, limit, startKey)
}

func (a *ServerWithRoles) filterAndListNodes(ctx context.Context, namespace string, limit int, startKey string) (page []types.Server, nextKey string, err error) {
	if limit <= 0 {
		return nil, "", trace.BadParameter("nonpositive parameter limit")
	}

	page = make([]types.Server, 0, limit)
	nextKey, err = a.authServer.IterateNodePages(ctx, namespace, limit, startKey, func(nextPage []types.Server) (bool, error) {
		// Retrieve and filter pages of nodes until we can fill a page or run out of nodes.
		filteredPage, err := a.filterNodes(nextPage)
		if err != nil {
			return false, trace.Wrap(err)
		}

		// We have more than enough nodes to fill the page, cut it to size.
		if len(filteredPage) > limit-len(page) {
			filteredPage = filteredPage[:limit-len(page)]
		}

		// Add filteredPage and break out of iterator if the page is now full.
		page = append(page, filteredPage...)
		return len(page) == limit, nil
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Filled a page, reset nextKey in case the last node was cut out.
	if len(page) == limit {
		nextKey = backend.NextPaginationKey(page[len(page)-1])
	}

	return page, nextKey, nil
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

func (a *ServerWithRoles) GetReverseTunnels(opts ...services.MarshalOption) ([]types.ReverseTunnel, error) {
	if err := a.action(apidefaults.Namespace, types.KindReverseTunnel, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnels(opts...)
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

func (a *ServerWithRoles) GetTokens(ctx context.Context, opts ...services.MarshalOption) ([]types.ProvisionToken, error) {
	if err := a.action(apidefaults.Namespace, types.KindToken, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetTokens(ctx, opts...)
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

func (a *ServerWithRoles) PreAuthenticatedSignIn(user string) (types.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.PreAuthenticatedSignIn(user, a.context.Identity.GetIdentity())
}

// CreateWebSession creates a new web session for the specified user
func (a *ServerWithRoles) CreateWebSession(user string) (types.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateWebSession(user)
}

// ExtendWebSession creates a new web session for a user based on a valid previous session.
// Additional roles are appended to initial roles if there is an approved access request.
// The new session expiration time will not exceed the expiration time of the old session.
func (a *ServerWithRoles) ExtendWebSession(req WebSessionReq) (types.WebSession, error) {
	if err := a.currentUserAction(req.User); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.ExtendWebSession(req, a.context.Identity.GetIdentity())
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
	if err := r.c.currentUserAction(req.User); err != nil {
		if err := r.c.action(apidefaults.Namespace, types.KindWebSession, types.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
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
	if !a.hasBuiltinRole(string(types.RoleAdmin)) {
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
		if !a.hasBuiltinRole(string(types.RoleAdmin)) {
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
		if !a.hasBuiltinRole(string(types.RoleAdmin)) {
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

// DeleteUser deletes an existng user in a backend by username.
func (a *ServerWithRoles) DeleteUser(ctx context.Context, user string) error {
	if err := a.action(apidefaults.Namespace, types.KindUser, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteUser(ctx, user)
}

func (a *ServerWithRoles) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	if err := a.action(apidefaults.Namespace, types.KindKeyPair, types.VerbCreate); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return a.authServer.GenerateKeyPair(pass)
}

func (a *ServerWithRoles) GenerateHostCert(
	key []byte, hostID, nodeName string, principals []string, clusterName string, role types.SystemRole, ttl time.Duration) ([]byte, error) {

	if err := a.action(apidefaults.Namespace, types.KindHostCert, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateHostCert(key, hostID, nodeName, principals, clusterName, role, ttl)
}

// NewKeepAliver not implemented: can only be called locally.
func (a *ServerWithRoles) NewKeepAliver(ctx context.Context) (types.KeepAliver, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// GenerateUserCerts generates users certificates
func (a *ServerWithRoles) GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
	return a.generateUserCerts(ctx, req)
}

func (a *ServerWithRoles) generateUserCerts(ctx context.Context, req proto.UserCertsRequest, opts ...certRequestOption) (*proto.Certs, error) {
	var err error
	var roles []string
	var traits wrappers.Traits

	// this prevents clients who have no chance at getting a cert and impersonating anyone
	// from enumerating local users and hitting database
	if !a.hasBuiltinRole(string(types.RoleAdmin)) && !a.context.Checker.CanImpersonateSomeone() && req.Username != a.context.User.GetName() {
		return nil, trace.AccessDenied("access denied: impersonation is not allowed")
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
	if a.context.Identity != nil && a.context.Identity.GetIdentity().Impersonator != "" {
		if len(req.AccessRequests) > 0 {
			return nil, trace.AccessDenied("access denied: impersonated user can not request new roles")
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
		expires := a.context.Identity.GetIdentity().Expires
		if expires.IsZero() {
			log.Warningf("Encountered identity with no expiry: %v and denied request. Must be internal logic error.", a.context.Identity)
			return nil, trace.AccessDenied("access denied")
		}
		if req.Expires.After(expires) {
			req.Expires = expires
		}
		if req.Expires.Before(a.authServer.GetClock().Now()) {
			return nil, trace.AccessDenied("access denied: client credentials have expired, please relogin.")
		}
	}

	// If the user is generating a certificate, the roles and traits come from the logged in identity.
	if req.Username == a.context.User.GetName() {
		roles, traits, err = services.ExtractFromIdentity(a.authServer, a.context.Identity.GetIdentity())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// Do not allow combining impersonation and access requests
		if len(req.AccessRequests) > 0 {
			log.WithError(err).Warningf("User %v tried to issue a cert for %v and added access requests. This is not supported.", a.context.User.GetName(), req.Username)
			return nil, trace.AccessDenied("access denied")
		}
		roles = user.GetRoles()
		traits = user.GetTraits()
	}

	if len(req.AccessRequests) > 0 {
		// add any applicable access request values.
		for _, reqID := range req.AccessRequests {
			accessReq, err := services.GetAccessRequest(ctx, a.authServer, reqID)
			if err != nil {
				if trace.IsNotFound(err) {
					return nil, trace.AccessDenied("invalid access request %q", reqID)
				}
				return nil, trace.Wrap(err)
			}
			if accessReq.GetUser() != req.Username {
				return nil, trace.AccessDenied("invalid access request %q", reqID)
			}
			if !accessReq.GetState().IsApproved() {
				if accessReq.GetState().IsDenied() {
					return nil, trace.AccessDenied("access-request %q has been denied", reqID)
				}
				return nil, trace.AccessDenied("access-request %q is awaiting approval", reqID)
			}
			if err := services.ValidateAccessRequestForUser(a.authServer, accessReq); err != nil {
				return nil, trace.Wrap(err)
			}
			aexp := accessReq.GetAccessExpiry()
			if aexp.Before(a.authServer.GetClock().Now()) {
				return nil, trace.AccessDenied("access-request %q is expired", reqID)
			}
			if aexp.Before(req.Expires) {
				// cannot generate a cert that would outlive the access request
				req.Expires = aexp
			}
			roles = append(roles, accessReq.GetRoles()...)
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
	checker := services.NewRoleSet(parsedRoles...)

	switch {
	case a.hasBuiltinRole(string(types.RoleAdmin)):
		// builtin admins can impersonate anyone
		// this is required for local tctl commands to work
	case req.Username == a.context.User.GetName():
		// users can impersonate themselves
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
		overrideRoleTTL:   a.hasBuiltinRole(string(types.RoleAdmin)),
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
		checker:           checker,
		traits:            traits,
		activeRequests: services.RequestIDs{
			AccessRequests: req.AccessRequests,
		},
	}
	if user.GetName() != a.context.User.GetName() {
		certReq.impersonator = a.context.User.GetName()
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
	default:
		return nil, trace.BadParameter("unsupported cert usage %q", req.Usage)
	}
	for _, o := range opts {
		o(&certReq)
	}
	certs, err := a.authServer.generateUserCert(certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certs, nil
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

func (a *ServerWithRoles) CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindOIDCRequest, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	oidcReq, err := a.authServer.CreateOIDCAuthRequest(req)
	if err != nil {
		emitSSOLoginFailureEvent(a.authServer.closeCtx, a.authServer.emitter, events.LoginMethodOIDC, err)
		return nil, trace.Wrap(err)
	}

	return oidcReq, nil
}

func (a *ServerWithRoles) ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	return a.authServer.ValidateOIDCAuthCallback(q)
}

func (a *ServerWithRoles) DeleteOIDCConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindOIDC, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteOIDCConnector(ctx, connectorID)
}

func (a *ServerWithRoles) CreateSAMLConnector(ctx context.Context, connector types.SAMLConnector) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if modules.GetModules().Features().SAML == false {
		return trace.AccessDenied("SAML is only available in enterprise subscriptions")
	}
	return a.authServer.UpsertSAMLConnector(ctx, connector)
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

func (a *ServerWithRoles) CreateSAMLAuthRequest(req services.SAMLAuthRequest) (*services.SAMLAuthRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindSAMLRequest, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	samlReq, err := a.authServer.CreateSAMLAuthRequest(req)
	if err != nil {
		emitSSOLoginFailureEvent(a.authServer.closeCtx, a.authServer.emitter, events.LoginMethodSAML, err)
		return nil, trace.Wrap(err)
	}

	return samlReq, nil
}

func (a *ServerWithRoles) ValidateSAMLResponse(re string) (*SAMLAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	return a.authServer.ValidateSAMLResponse(re)
}

// DeleteSAMLConnector deletes a SAML connector by name.
func (a *ServerWithRoles) DeleteSAMLConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindSAML, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteSAMLConnector(ctx, connectorID)
}

func (a *ServerWithRoles) CreateGithubConnector(connector types.GithubConnector) error {
	if err := a.authConnectorAction(apidefaults.Namespace, types.KindGithub, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkGithubConnector(connector); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateGithubConnector(connector)
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

func (a *ServerWithRoles) CreateGithubAuthRequest(req services.GithubAuthRequest) (*services.GithubAuthRequest, error) {
	if err := a.action(apidefaults.Namespace, types.KindGithubRequest, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	githubReq, err := a.authServer.CreateGithubAuthRequest(req)
	if err != nil {
		emitSSOLoginFailureEvent(a.authServer.closeCtx, a.authServer.emitter, events.LoginMethodGithub, err)
		return nil, trace.Wrap(err)
	}

	return githubReq, nil
}

func (a *ServerWithRoles) ValidateGithubAuthCallback(q url.Values) (*GithubAuthResponse, error) {
	return a.authServer.ValidateGithubAuthCallback(q)
}

// EmitAuditEvent emits a single audit event
func (a *ServerWithRoles) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	if err := a.action(apidefaults.Namespace, types.KindEvent, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	role, ok := a.context.Identity.(BuiltinRole)
	if !ok || !role.IsServer() {
		return trace.AccessDenied("this request can be only executed by proxy, node or auth")
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

// streamWithRoles verifies every event
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

func (a *ServerWithRoles) EmitAuditEventLegacy(event events.Event, fields events.EventFields) error {
	if err := a.action(apidefaults.Namespace, types.KindEvent, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.EmitAuditEventLegacy(event, fields)
}

func (a *ServerWithRoles) PostSessionSlice(slice events.SessionSlice) error {
	if err := a.action(slice.Namespace, types.KindEvent, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.PostSessionSlice(slice)
}

func (a *ServerWithRoles) UploadSessionRecording(r events.SessionRecording) error {
	if err := r.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(r.Namespace, types.KindEvent, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.UploadSessionRecording(r)
}

func (a *ServerWithRoles) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	if err := a.action(namespace, types.KindSession, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionChunk(namespace, sid, offsetBytes, maxBytes)
}

func (a *ServerWithRoles) GetSessionEvents(namespace string, sid session.ID, afterN int, includePrintEvents bool) ([]events.EventFields, error) {
	if err := a.action(namespace, types.KindSession, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionEvents(namespace, sid, afterN, includePrintEvents)
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
	features := modules.GetModules().Features()
	options := role.GetOptions()
	allowReq, allowRev := role.GetAccessRequestConditions(types.Allow), role.GetAccessReviewConditions(types.Allow)

	switch {
	case features.AccessControls == false && options.MaxSessions > 0:
		return trace.AccessDenied(
			"role option max_sessions is only available in enterprise subscriptions")
	case features.AdvancedAccessWorkflows == false &&
		(options.RequestAccess == types.RequestStrategyReason || options.RequestAccess == types.RequestStrategyAlways):
		return trace.AccessDenied(
			"role option request_access: %v is only available in enterprise subscriptions", options.RequestAccess)
	case features.AdvancedAccessWorkflows == false && len(allowReq.Thresholds) != 0:
		return trace.AccessDenied(
			"role field allow.request.thresholds is only available in enterprise subscriptions")
	case features.AdvancedAccessWorkflows == false && !allowRev.IsZero():
		return trace.AccessDenied(
			"role field allow.review_requests is only available in enterprise subscriptions")
	}

	// access predicate syntax is not checked as part of normal role validation in order
	// to allow the available namespaces to be extended without breaking compatibility with
	// older nodes/proxies (which do not need to ever evaluate said predicates).
	if err := services.ValidateAccessPredicates(role); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.UpsertRole(ctx, role)
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

func (a *ServerWithRoles) ValidateTrustedCluster(validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	// the token provides it's own authorization and authentication
	return a.authServer.validateTrustedCluster(validateRequest)
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
	if !a.hasBuiltinRole(string(types.RoleProxy)) {
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
	if !a.hasBuiltinRole(string(types.RoleProxy)) {
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
	// Check if this is a local cluster admin, or a datababase service, or a
	// user that is allowed to impersonate database service.
	if !a.hasBuiltinRole(string(types.RoleDatabase)) && !a.hasBuiltinRole(string(types.RoleAdmin)) {
		if err := a.canImpersonateBuiltinRole(types.RoleDatabase); err != nil {
			log.WithError(err).Warnf("User %v tried to generate database certificate but is not allowed to impersonate %q system role.",
				a.context.User.GetName(), types.RoleDatabase)
			return nil, trace.AccessDenied("access denied")
		}
	}
	return a.authServer.GenerateDatabaseCert(ctx, req)
}

// canImpersonateBuiltinRole checks if the current user can impersonate the
// provided system role.
func (a *ServerWithRoles) canImpersonateBuiltinRole(role types.SystemRole) error {
	roleCtx, err := NewBuiltinRoleContext(role)
	if err != nil {
		return trace.Wrap(err)
	}
	roleSet, ok := roleCtx.Checker.(BuiltinRoleSet)
	if !ok {
		return trace.BadParameter("expected BuiltinRoleSet, got %T", roleCtx.Checker)
	}
	err = a.context.Checker.CheckImpersonate(a.context.User, roleCtx.User, roleSet.RoleSet.WithoutImplicit())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a *ServerWithRoles) checkAccessToApp(app types.Application) error {
	return a.context.Checker.CheckAccessToApp(app.GetNamespace(), app,
		// MFA is not required for operations on database resources but
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
			err = a.context.Checker.CheckAccessToApp(server.GetNamespace(), appV3, mfaParams)
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

// UpsertAppSession not implemented: can only be called locally.
func (a *ServerWithRoles) UpsertAppSession(ctx context.Context, session types.WebSession) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAppSession removes an application web session.
func (a *ServerWithRoles) DeleteAppSession(ctx context.Context, req types.DeleteAppSessionRequest) error {
	session, err := a.authServer.GetAppSession(ctx, types.GetAppSessionRequest(req))
	if err != nil {
		return trace.Wrap(err)
	}
	// Users can only delete their own app sessions.
	if err := a.currentUserAction(session.GetUser()); err != nil {
		if err := a.action(apidefaults.Namespace, types.KindWebSession, types.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := a.authServer.DeleteAppSession(ctx, req); err != nil {
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

// GenerateAppToken creates a JWT token with application access.
func (a *ServerWithRoles) GenerateAppToken(ctx context.Context, req types.GenerateAppTokenRequest) (string, error) {
	if err := a.action(apidefaults.Namespace, types.KindJWT, types.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}

	session, err := a.authServer.generateAppToken(req.Username, req.Roles, req.URI, req.Expires)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return session, nil
}

func (a *ServerWithRoles) Close() error {
	return a.authServer.Close()
}

func (a *ServerWithRoles) WaitForDelivery(context.Context) error {
	return nil
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
	_, isService := a.context.Checker.(BuiltinRoleSet)
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
		if err := a.context.Checker.CheckAccessToKubernetes(s.GetNamespace(), kube, mfaParams); err != nil {
			return utils.OpaqueAccessDenied(err)
		}
	}
	return a.authServer.UpsertKubeService(ctx, s)
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

	// Loop over all servers, filter out kube clusters on each server and only
	// return the kube cluster the caller has access to.
	//
	// MFA is not required to list the clusters, but will be required to
	// connect to them.
	mfaParams := services.AccessMFAParams{Verified: true}
	for _, server := range servers {
		filtered := make([]*types.KubernetesCluster, 0, len(server.GetKubernetesClusters()))
		for _, kube := range server.GetKubernetesClusters() {
			if err := a.context.Checker.CheckAccessToKubernetes(server.GetNamespace(), kube, mfaParams); err != nil {
				if trace.IsAccessDenied(err) {
					continue
				}
				return nil, trace.Wrap(err)
			}
			filtered = append(filtered, kube)
		}
		server.SetKubernetesClusters(filtered)
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
	if !hasLocalUserRole(a.context.Checker) && !hasRemoteUserRole(a.context.Checker) {
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
func (a *ServerWithRoles) SearchSessionEvents(fromUTC, toUTC time.Time, limit int, order types.EventOrder, startKey string) (events []apievents.AuditEvent, lastKey string, err error) {
	if err := a.action(apidefaults.Namespace, types.KindSession, types.VerbList); err != nil {
		return nil, "", trace.Wrap(err)
	}

	events, lastKey, err = a.alog.SearchSessionEvents(fromUTC, toUTC, limit, order, startKey)
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
// channel if one is encountered. Otherwise it is simply closed when the stream ends.
// The event channel is not closed on error to prevent race conditions in downstream select statements.
func (a *ServerWithRoles) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	if err := a.action(apidefaults.Namespace, types.KindSession, types.VerbList); err != nil {
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
	return a.context.Checker.CheckAccessToDatabase(database,
		// MFA is not required for operations on database resources but
		// will be enforced at the connection time.
		services.AccessMFAParams{Verified: true},
		&services.DatabaseLabelsMatcher{Labels: database.GetAllLabels()})
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
func (a *ServerWithRoles) GetWindowsDesktops(ctx context.Context) ([]types.WindowsDesktop, error) {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktop, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	hosts, err := a.authServer.GetWindowsDesktops(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return hosts, nil
}

// GetWindowsDesktop returns a registered windows desktop host.
func (a *ServerWithRoles) GetWindowsDesktop(ctx context.Context, name string) (types.WindowsDesktop, error) {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktop, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	host, err := a.authServer.GetWindowsDesktop(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return host, nil
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
	return a.authServer.UpdateWindowsDesktop(ctx, s)
}

// DeleteWindowsDesktop removes the specified windows desktop host.
func (a *ServerWithRoles) DeleteWindowsDesktop(ctx context.Context, name string) error {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktop, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteWindowsDesktop(ctx, name)
}

// DeleteAllWindowsDesktops removes all registered windows desktop hosts.
func (a *ServerWithRoles) DeleteAllWindowsDesktops(ctx context.Context) error {
	if err := a.action(apidefaults.Namespace, types.KindWindowsDesktop, types.VerbList, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllWindowsDesktops(ctx)
}

// StartAccountRecovery is implemented by AuthService.StartAccountRecovery.
func (a *ServerWithRoles) StartAccountRecovery(ctx context.Context, req *proto.StartAccountRecoveryRequest) (types.UserToken, error) {
	return a.authServer.StartAccountRecovery(ctx, req)
}

// ApproveAccountRecovery is implemented by AuthService.ApproveAccountRecovery.
func (a *ServerWithRoles) ApproveAccountRecovery(ctx context.Context, req *proto.ApproveAccountRecoveryRequest) (types.UserToken, error) {
	// The token provides its own authorization and authentication.
	return a.authServer.ApproveAccountRecovery(ctx, req)
}

// CompleteAccountRecovery is implemented by AuthService.CompleteAccountRecovery.
func (a *ServerWithRoles) CompleteAccountRecovery(ctx context.Context, req *proto.CompleteAccountRecoveryRequest) error {
	// The token provides its own authorization and authentication.
	return a.authServer.CompleteAccountRecovery(ctx, req)
}

// CreateAccountRecoveryCodes is implemented by AuthService.CreateAccountRecoveryCodes.
func (a *ServerWithRoles) CreateAccountRecoveryCodes(ctx context.Context, req *proto.CreateAccountRecoveryCodesRequest) (*proto.CreateAccountRecoveryCodesResponse, error) {
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
func (a *ServerWithRoles) GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*types.RecoveryCodesV1, error) {
	// User in context can retrieve their own recovery codes.
	return a.authServer.GetAccountRecoveryCodes(ctx, req)
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

func emitSSOLoginFailureEvent(ctx context.Context, emitter apievents.Emitter, method string, err error) {
	emitErr := emitter.EmitAuditEvent(ctx, &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserSSOLoginFailureCode,
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
