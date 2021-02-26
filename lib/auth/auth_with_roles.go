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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/jwt"
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

func (a *ServerWithRoles) actionWithContext(ctx *services.Context, namespace string, resource string, action string) error {
	return utils.OpaqueAccessDenied(a.context.Checker.CheckAccessToRule(ctx, namespace, resource, action, false))
}

func (a *ServerWithRoles) action(namespace string, resource string, action string) error {
	return utils.OpaqueAccessDenied(a.context.Checker.CheckAccessToRule(&services.Context{User: a.context.User}, namespace, resource, action, false))
}

// currentUserAction is a special checker that allows certain actions for users
// even if they are not admins, e.g. update their own passwords,
// or generate certificates, otherwise it will require admin privileges
func (a *ServerWithRoles) currentUserAction(username string) error {
	if hasLocalUserRole(a.context.Checker) && username == a.context.User.GetName() {
		return nil
	}
	return utils.OpaqueAccessDenied(
		a.context.Checker.CheckAccessToRule(&services.Context{User: a.context.User},
			defaults.Namespace, services.KindUser, services.VerbCreate, true),
	)
}

// authConnectorAction is a special checker that grants access to auth
// connectors. It first checks if you have access to the specific connector.
// If not, it checks if the requester has the meta KindAuthConnector access
// (which grants access to all connectors).
func (a *ServerWithRoles) authConnectorAction(namespace string, resource string, verb string) error {
	if err := a.context.Checker.CheckAccessToRule(&services.Context{User: a.context.User}, namespace, resource, verb, true); err != nil {
		if err := a.context.Checker.CheckAccessToRule(&services.Context{User: a.context.User}, namespace, services.KindAuthConnector, verb, false); err != nil {
			return utils.OpaqueAccessDenied(err)
		}
	}
	return nil
}

// hasBuiltinRole checks the type of the role set returned and the name.
// Returns true if role set is builtin and the name matches.
func (a *ServerWithRoles) hasBuiltinRole(name string) bool {
	return hasBuiltinRole(a.context.Checker, name)
}

// hasBuiltinRole checks the type of the role set returned and the name.
// Returns true if role set is builtin and the name matches.
func hasBuiltinRole(checker services.AccessChecker, name string) bool {
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
func (a *ServerWithRoles) AuthenticateWebUser(req AuthenticateUserRequest) (services.WebSession, error) {
	// authentication request has it's own authentication, however this limits the requests
	// types to proxies to make it harder to break
	if !a.hasBuiltinRole(string(teleport.RoleProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.AuthenticateWebUser(req)
}

// AuthenticateSSHUser authenticates SSH console user, creates and  returns a pair of signed TLS and SSH
// short lived certificates as a result
func (a *ServerWithRoles) AuthenticateSSHUser(req AuthenticateSSHRequest) (*SSHLoginResponse, error) {
	// authentication request has it's own authentication, however this limits the requests
	// types to proxies to make it harder to break
	if !a.hasBuiltinRole(string(teleport.RoleProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.AuthenticateSSHUser(req)
}

func (a *ServerWithRoles) GetSessions(namespace string) ([]session.Session, error) {
	if err := a.action(namespace, services.KindSSHSession, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.sessions.GetSessions(namespace)
}

func (a *ServerWithRoles) GetSession(namespace string, id session.ID) (*session.Session, error) {
	if err := a.action(namespace, services.KindSSHSession, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.sessions.GetSession(namespace, id)
}

func (a *ServerWithRoles) CreateSession(s session.Session) error {
	if err := a.action(s.Namespace, services.KindSSHSession, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.CreateSession(s)
}

func (a *ServerWithRoles) UpdateSession(req session.UpdateRequest) error {
	if err := a.action(req.Namespace, services.KindSSHSession, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.UpdateSession(req)
}

// DeleteSession removes an active session from the backend.
func (a *ServerWithRoles) DeleteSession(namespace string, id session.ID) error {
	if err := a.action(namespace, services.KindSSHSession, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.DeleteSession(namespace, id)
}

// CreateCertAuthority not implemented: can only be called locally.
func (a *ServerWithRoles) CreateCertAuthority(ca services.CertAuthority) error {
	return trace.NotImplemented(notImplementedMessage)
}

// RotateCertAuthority starts or restarts certificate authority rotation process.
func (a *ServerWithRoles) RotateCertAuthority(req RotateRequest) error {
	if err := req.CheckAndSetDefaults(a.authServer.clock); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.RotateCertAuthority(req)
}

// RotateExternalCertAuthority rotates external certificate authority,
// this method is called by a remote trusted cluster and is used to update
// only public keys and certificates of the certificate authority.
func (a *ServerWithRoles) RotateExternalCertAuthority(ca services.CertAuthority) error {
	if ca == nil {
		return trace.BadParameter("missing certificate authority")
	}
	ctx := &services.Context{User: a.context.User, Resource: ca}
	if err := a.actionWithContext(ctx, defaults.Namespace, services.KindCertAuthority, services.VerbRotate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.RotateExternalCertAuthority(ca)
}

// UpsertCertAuthority updates existing cert authority or updates the existing one.
func (a *ServerWithRoles) UpsertCertAuthority(ca services.CertAuthority) error {
	if ca == nil {
		return trace.BadParameter("missing certificate authority")
	}
	ctx := &services.Context{User: a.context.User, Resource: ca}
	if err := a.actionWithContext(ctx, defaults.Namespace, services.KindCertAuthority, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.actionWithContext(ctx, defaults.Namespace, services.KindCertAuthority, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertCertAuthority(ca)
}

// CompareAndSwapCertAuthority updates existing cert authority if the existing cert authority
// value matches the value stored in the backend.
func (a *ServerWithRoles) CompareAndSwapCertAuthority(new, existing services.CertAuthority) error {
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CompareAndSwapCertAuthority(new, existing)
}

func (a *ServerWithRoles) GetCertAuthorities(caType services.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]services.CertAuthority, error) {
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if loadKeys {
		if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetCertAuthorities(caType, loadKeys, opts...)
}

func (a *ServerWithRoles) GetCertAuthority(id services.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (services.CertAuthority, error) {
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if loadKeys {
		if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbRead); err != nil {
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

// GetClusterCACert returns the CAs for the local cluster without signing keys.
func (a *ServerWithRoles) GetClusterCACert() (*LocalCAResponse, error) {
	// Allow all roles to get the local CA.
	return a.authServer.GetClusterCACert()
}

func (a *ServerWithRoles) UpsertLocalClusterName(clusterName string) error {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertLocalClusterName(clusterName)
}

func (a *ServerWithRoles) DeleteCertAuthority(id services.CertAuthID) error {
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteCertAuthority(id)
}

// ActivateCertAuthority not implemented: can only be called locally.
func (a *ServerWithRoles) ActivateCertAuthority(id services.CertAuthID) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeactivateCertAuthority not implemented: can only be called locally.
func (a *ServerWithRoles) DeactivateCertAuthority(id services.CertAuthID) error {
	return trace.NotImplemented(notImplementedMessage)
}

// GenerateToken generates multi-purpose authentication token.
func (a *ServerWithRoles) GenerateToken(ctx context.Context, req GenerateTokenRequest) (string, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.GenerateToken(ctx, req)
}

func (a *ServerWithRoles) RegisterUsingToken(req RegisterUsingTokenRequest) (*PackedKeys, error) {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterUsingToken(req)
}

func (a *ServerWithRoles) RegisterNewAuthServer(token string) error {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterNewAuthServer(token)
}

// GenerateServerKeys generates new host private keys and certificates (signed
// by the host certificate authority) for a node.
func (a *ServerWithRoles) GenerateServerKeys(req GenerateServerKeysRequest) (*PackedKeys, error) {
	clusterName, err := a.authServer.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// username is hostID + cluster name, so make sure server requests new keys for itself
	if a.context.User.GetName() != HostFQDN(req.HostID, clusterName) {
		return nil, trace.AccessDenied("username mismatch %q and %q", a.context.User.GetName(), HostFQDN(req.HostID, clusterName))
	}
	existingRoles, err := teleport.NewRoles(a.context.User.GetRoles())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// prohibit privilege escalations through role changes
	if !existingRoles.Equals(req.Roles) {
		return nil, trace.AccessDenied("roles do not match: %v and %v", existingRoles, req.Roles)
	}
	return a.authServer.GenerateServerKeys(req)
}

// UpsertNodes bulk upserts nodes into the backend.
func (a *ServerWithRoles) UpsertNodes(namespace string, servers []services.Server) error {
	if err := a.action(namespace, services.KindNode, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(namespace, services.KindNode, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertNodes(namespace, servers)
}

func (a *ServerWithRoles) UpsertNode(s services.Server) (*services.KeepAlive, error) {
	if err := a.action(s.GetNamespace(), services.KindNode, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(s.GetNamespace(), services.KindNode, services.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.UpsertNode(s)
}

// DELETE IN: 5.1.0
//
// This logic has moved to KeepAliveServer.
func (a *ServerWithRoles) KeepAliveNode(ctx context.Context, handle services.KeepAlive) error {
	if !a.hasBuiltinRole(string(teleport.RoleNode)) {
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
	if err := a.action(defaults.Namespace, services.KindNode, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.KeepAliveNode(ctx, handle)
}

// KeepAliveServer updates expiry time of a server resource.
func (a *ServerWithRoles) KeepAliveServer(ctx context.Context, handle services.KeepAlive) error {
	clusterName, err := a.GetDomainName()
	if err != nil {
		return trace.Wrap(err)
	}
	serverName, err := ExtractHostID(a.context.User.GetName(), clusterName)
	if err != nil {
		return trace.AccessDenied("access denied")
	}

	switch handle.GetType() {
	case teleport.KeepAliveNode:
		if serverName != handle.Name {
			return trace.AccessDenied("access denied")
		}
		if !a.hasBuiltinRole(string(teleport.RoleNode)) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(defaults.Namespace, services.KindNode, services.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	case teleport.KeepAliveApp:
		if serverName != handle.Name {
			return trace.AccessDenied("access denied")
		}
		if !a.hasBuiltinRole(string(teleport.RoleApp)) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(defaults.Namespace, services.KindAppServer, services.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	case teleport.KeepAliveDatabase:
		// There can be multiple database servers per host so they send their
		// host ID in a separate field because unlike SSH nodes the resource
		// name cannot be the host ID.
		if serverName != handle.HostID {
			return trace.AccessDenied("access denied")
		}
		if !a.hasBuiltinRole(string(teleport.RoleDatabase)) {
			return trace.AccessDenied("access denied")
		}
		if err := a.action(defaults.Namespace, types.KindDatabaseServer, services.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unknown keep alive type %q", handle.Type)
	}

	return a.authServer.KeepAliveServer(ctx, handle)
}

// NewWatcher returns a new event watcher
func (a *ServerWithRoles) NewWatcher(ctx context.Context, watch services.Watch) (services.Watcher, error) {
	if len(watch.Kinds) == 0 {
		return nil, trace.AccessDenied("can't setup global watch")
	}
	for _, kind := range watch.Kinds {
		// Check the permissions for data of each kind. For watching, most
		// kinds of data just need a Read permission, but some have more
		// complicated logic.
		switch kind.Kind {
		case services.KindCertAuthority:
			if kind.LoadSecrets {
				if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbRead); err != nil {
					return nil, trace.Wrap(err)
				}
			} else {
				if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbReadNoSecrets); err != nil {
					return nil, trace.Wrap(err)
				}
			}
		case services.KindAccessRequest:
			var filter services.AccessRequestFilter
			if err := filter.FromMap(kind.Filter); err != nil {
				return nil, trace.Wrap(err)
			}
			if filter.User == "" || a.currentUserAction(filter.User) != nil {
				if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbRead); err != nil {
					return nil, trace.Wrap(err)
				}
			}
		case services.KindAppServer:
			if err := a.action(defaults.Namespace, services.KindAppServer, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindWebSession:
			var filter types.WebSessionFilter
			if err := filter.FromMap(kind.Filter); err != nil {
				return nil, trace.Wrap(err)
			}
			if filter.User == "" || a.currentUserAction(filter.User) != nil {
				if err := a.action(defaults.Namespace, services.KindWebSession, services.VerbRead); err != nil {
					return nil, trace.Wrap(err)
				}
			}
		case services.KindWebToken:
			if err := a.action(defaults.Namespace, services.KindWebToken, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindRemoteCluster:
			if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindDatabaseServer:
			if err := a.action(defaults.Namespace, types.KindDatabaseServer, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		default:
			if err := a.action(defaults.Namespace, kind.Kind, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	switch {
	case a.hasBuiltinRole(string(teleport.RoleProxy)):
		watch.QueueSize = defaults.ProxyQueueSize
	case a.hasBuiltinRole(string(teleport.RoleNode)):
		watch.QueueSize = defaults.NodeQueueSize
	}
	return a.authServer.NewWatcher(ctx, watch)
}

// filterNodes filters nodes based off the role of the logged in user.
func (a *ServerWithRoles) filterNodes(nodes []services.Server) ([]services.Server, error) {
	// For certain built-in roles, continue to allow full access and return
	// the full set of nodes to not break existing clusters during migration.
	//
	// In addition, allow proxy (and remote proxy) to access all nodes for it's
	// smart resolution address resolution. Once the smart resolution logic is
	// moved to the auth server, this logic can be removed.
	if a.hasBuiltinRole(string(teleport.RoleAdmin)) ||
		a.hasBuiltinRole(string(teleport.RoleProxy)) ||
		a.hasRemoteBuiltinRole(string(teleport.RoleRemoteProxy)) {
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
	filteredNodes := make([]services.Server, 0, len(nodes))
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
func (a *ServerWithRoles) DeleteAllNodes(namespace string) error {
	if err := a.action(namespace, services.KindNode, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllNodes(namespace)
}

// DeleteNode deletes node in the namespace
func (a *ServerWithRoles) DeleteNode(namespace, node string) error {
	if err := a.action(namespace, services.KindNode, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteNode(namespace, node)
}

func (a *ServerWithRoles) GetNodes(namespace string, opts ...services.MarshalOption) ([]services.Server, error) {
	if err := a.action(namespace, services.KindNode, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch full list of nodes in the backend.
	startFetch := time.Now()
	nodes, err := a.authServer.GetNodes(namespace, opts...)
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

func (a *ServerWithRoles) UpsertAuthServer(s services.Server) error {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertAuthServer(s)
}

func (a *ServerWithRoles) GetAuthServers() ([]services.Server, error) {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetAuthServers()
}

// DeleteAllAuthServers deletes all auth servers
func (a *ServerWithRoles) DeleteAllAuthServers() error {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllAuthServers()
}

// DeleteAuthServer deletes auth server by name
func (a *ServerWithRoles) DeleteAuthServer(name string) error {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAuthServer(name)
}

func (a *ServerWithRoles) UpsertProxy(s services.Server) error {
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertProxy(s)
}

func (a *ServerWithRoles) GetProxies() ([]services.Server, error) {
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetProxies()
}

// DeleteAllProxies deletes all proxies
func (a *ServerWithRoles) DeleteAllProxies() error {
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllProxies()
}

// DeleteProxy deletes proxy by name
func (a *ServerWithRoles) DeleteProxy(name string) error {
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteProxy(name)
}

func (a *ServerWithRoles) UpsertReverseTunnel(r services.ReverseTunnel) error {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertReverseTunnel(r)
}

func (a *ServerWithRoles) GetReverseTunnel(name string, opts ...services.MarshalOption) (services.ReverseTunnel, error) {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnel(name, opts...)
}

func (a *ServerWithRoles) GetReverseTunnels(opts ...services.MarshalOption) ([]services.ReverseTunnel, error) {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnels(opts...)
}

func (a *ServerWithRoles) DeleteReverseTunnel(domainName string) error {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteReverseTunnel(domainName)
}

func (a *ServerWithRoles) DeleteToken(token string) error {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteToken(token)
}

func (a *ServerWithRoles) GetTokens(opts ...services.MarshalOption) ([]services.ProvisionToken, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetTokens(opts...)
}

func (a *ServerWithRoles) GetToken(token string) (services.ProvisionToken, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetToken(token)
}

func (a *ServerWithRoles) UpsertToken(token services.ProvisionToken) error {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertToken(token)
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

func (a *ServerWithRoles) PreAuthenticatedSignIn(user string) (services.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.PreAuthenticatedSignIn(user, a.context.Identity.GetIdentity())
}

func (a *ServerWithRoles) GetMFAAuthenticateChallenge(user string, password []byte) (*MFAAuthenticateChallenge, error) {
	// we are already checking password here, no need to extra permission check
	// anyone who has user's password can generate sign request
	return a.authServer.GetMFAAuthenticateChallenge(user, password)
}

// CreateWebSession creates a new web session for the specified user
func (a *ServerWithRoles) CreateWebSession(user string) (services.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateWebSession(user)
}

// ExtendWebSession creates a new web session for a user based on a valid previous session.
// Additional roles are appended to initial roles if there is an approved access request.
// The new session expiration time will not exceed the expiration time of the old session.
func (a *ServerWithRoles) ExtendWebSession(user, prevSessionID, accessRequestID string) (services.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.ExtendWebSession(user, prevSessionID, accessRequestID, a.context.Identity.GetIdentity())
}

// GetWebSessionInfo returns the web session for the given user specified with sid.
// The session is stripped of any authentication details.
// Implements auth.WebUIService
func (a *ServerWithRoles) GetWebSessionInfo(ctx context.Context, user, sessionID string) (services.WebSession, error) {
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
		if err := r.c.action(defaults.Namespace, services.KindWebSession, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return r.ws.Get(ctx, req)
}

// List returns the list of all web sessions.
func (r *webSessionsWithRoles) List(ctx context.Context) ([]services.WebSession, error) {
	if err := r.c.action(defaults.Namespace, services.KindWebSession, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := r.c.action(defaults.Namespace, services.KindWebSession, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.ws.List(ctx)
}

// Upsert creates a new or updates the existing web session from the specified session.
// TODO(dmitri): this is currently only implemented for local invocations. This needs to be
// moved into a more appropriate API
func (*webSessionsWithRoles) Upsert(ctx context.Context, session services.WebSession) error {
	return trace.NotImplemented(notImplementedMessage)
}

// Delete removes the web session specified with req.
func (r *webSessionsWithRoles) Delete(ctx context.Context, req types.DeleteWebSessionRequest) error {
	if err := r.c.currentUserAction(req.User); err != nil {
		if err := r.c.action(defaults.Namespace, services.KindWebSession, services.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
	}
	return r.ws.Delete(ctx, req)
}

// DeleteAll removes all web sessions.
func (r *webSessionsWithRoles) DeleteAll(ctx context.Context) error {
	if err := r.c.action(defaults.Namespace, services.KindWebSession, services.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := r.c.action(defaults.Namespace, services.KindWebSession, services.VerbDelete); err != nil {
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
		if err := r.c.action(defaults.Namespace, services.KindWebToken, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return r.t.Get(ctx, req)
}

// List returns the list of all web tokens.
func (r *webTokensWithRoles) List(ctx context.Context) ([]types.WebToken, error) {
	if err := r.c.action(defaults.Namespace, services.KindWebToken, services.VerbList); err != nil {
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
		if err := r.c.action(defaults.Namespace, services.KindWebToken, services.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
	}
	return r.t.Delete(ctx, req)
}

// DeleteAll removes all web tokens.
func (r *webTokensWithRoles) DeleteAll(ctx context.Context) error {
	if err := r.c.action(defaults.Namespace, services.KindWebToken, services.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := r.c.action(defaults.Namespace, services.KindWebToken, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return r.t.DeleteAll(ctx)
}

type webTokensWithRoles struct {
	c accessChecker
	t types.WebTokenInterface
}

type accessChecker interface {
	action(namespace, resource, action string) error
	currentUserAction(user string) error
}

func (a *ServerWithRoles) GetAccessRequests(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error) {
	// An exception is made to allow users to get their own access requests.
	if filter.User == "" || a.currentUserAction(filter.User) != nil {
		if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbList); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetAccessRequests(ctx, filter)
}

func (a *ServerWithRoles) CreateAccessRequest(ctx context.Context, req services.AccessRequest) error {
	// An exception is made to allow users to create access *pending* requests for themselves.
	if !req.GetState().IsPending() || a.currentUserAction(req.GetUser()) != nil {
		if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbCreate); err != nil {
			return trace.Wrap(err)
		}
	}
	// Ensure that an access request cannot outlive the identity that creates it.
	if req.GetAccessExpiry().Before(a.authServer.GetClock().Now()) || req.GetAccessExpiry().After(a.context.Identity.GetIdentity().Expires) {
		req.SetAccessExpiry(a.context.Identity.GetIdentity().Expires)
	}
	return a.authServer.CreateAccessRequest(ctx, req)
}

func (a *ServerWithRoles) SetAccessRequestState(ctx context.Context, params services.AccessRequestUpdate) error {
	if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetAccessRequestState(ctx, params)
}

func (a *ServerWithRoles) GetAccessCapabilities(ctx context.Context, req services.AccessCapabilitiesRequest) (*services.AccessCapabilities, error) {
	// default to checking the capabilities of the caller
	if req.User == "" {
		req.User = a.context.User.GetName()
	}

	// all users can check their own capabilities
	if a.currentUserAction(req.User) != nil {
		if err := a.action(defaults.Namespace, services.KindUser, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := a.action(defaults.Namespace, services.KindRole, services.VerbList); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := a.action(defaults.Namespace, services.KindRole, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return a.authServer.GetAccessCapabilities(ctx, req)
}

// GetPluginData loads all plugin data matching the supplied filter.
func (a *ServerWithRoles) GetPluginData(ctx context.Context, filter services.PluginDataFilter) ([]services.PluginData, error) {
	switch filter.Kind {
	case services.KindAccessRequest:
		if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbList); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
		return a.authServer.GetPluginData(ctx, filter)
	default:
		return nil, trace.BadParameter("unsupported resource kind %q", filter.Kind)
	}
}

// UpdatePluginData updates a per-resource PluginData entry.
func (a *ServerWithRoles) UpdatePluginData(ctx context.Context, params services.PluginDataUpdateParams) error {
	switch params.Kind {
	case services.KindAccessRequest:
		if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbUpdate); err != nil {
			return trace.Wrap(err)
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
	if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAccessRequest(ctx, name)
}

func (a *ServerWithRoles) GetUsers(withSecrets bool) ([]services.User, error) {
	if withSecrets {
		// TODO(fspmarshall): replace admin requirement with VerbReadWithSecrets once we've
		// migrated to that model.
		if !a.hasBuiltinRole(string(teleport.RoleAdmin)) {
			err := trace.AccessDenied("user %q requested access to all users with secrets", a.context.User.GetName())
			log.Warning(err)
			if err := a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &events.UserLogin{
				Metadata: events.Metadata{
					Type: events.UserLoginEvent,
					Code: events.UserLocalLoginFailureCode,
				},
				Method: events.LoginMethodClientCert,
				Status: events.Status{
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
		if err := a.action(defaults.Namespace, services.KindUser, services.VerbList); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := a.action(defaults.Namespace, services.KindUser, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetUsers(withSecrets)
}

func (a *ServerWithRoles) GetUser(name string, withSecrets bool) (services.User, error) {
	if withSecrets {
		// TODO(fspmarshall): replace admin requirement with VerbReadWithSecrets once we've
		// migrated to that model.
		if !a.hasBuiltinRole(string(teleport.RoleAdmin)) {
			err := trace.AccessDenied("user %q requested access to user %q with secrets", a.context.User.GetName(), name)
			log.Warning(err)
			if err := a.authServer.emitter.EmitAuditEvent(a.authServer.closeCtx, &events.UserLogin{
				Metadata: events.Metadata{
					Type: events.UserLoginEvent,
					Code: events.UserLocalLoginFailureCode,
				},
				Method: events.LoginMethodClientCert,
				Status: events.Status{
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
			if err := a.action(defaults.Namespace, services.KindUser, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	return a.authServer.Identity.GetUser(name, withSecrets)
}

// DeleteUser deletes an existng user in a backend by username.
func (a *ServerWithRoles) DeleteUser(ctx context.Context, user string) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteUser(ctx, user)
}

func (a *ServerWithRoles) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	if err := a.action(defaults.Namespace, services.KindKeyPair, services.VerbCreate); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return a.authServer.GenerateKeyPair(pass)
}

func (a *ServerWithRoles) GenerateHostCert(
	key []byte, hostID, nodeName string, principals []string, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error) {

	if err := a.action(defaults.Namespace, services.KindHostCert, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateHostCert(key, hostID, nodeName, principals, clusterName, roles, ttl)
}

// NewKeepAliver not implemented: can only be called locally.
func (a *ServerWithRoles) NewKeepAliver(ctx context.Context) (services.KeepAliver, error) {
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
	if !a.hasBuiltinRole(string(teleport.RoleAdmin)) && !a.context.Checker.CanImpersonateSomeone() && req.Username != a.context.User.GetName() {
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
		roles = utils.Deduplicate(roles)
	}

	parsedRoles, err := services.FetchRoleList(roles, a.authServer, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// add implicit roles to the set and build a checker
	checker := services.NewRoleSet(parsedRoles...)

	switch {
	case a.hasBuiltinRole(string(teleport.RoleAdmin)):
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
			if err := a.authServer.emitter.EmitAuditEvent(a.CloseContext(), &events.UserLogin{
				Metadata: events.Metadata{
					Type: events.UserLoginEvent,
					Code: events.UserLocalLoginFailureCode,
				},
				Method: events.LoginMethodClientCert,
				Status: events.Status{
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
		overrideRoleTTL:   a.hasBuiltinRole(string(teleport.RoleAdmin)),
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

	return &proto.Certs{
		SSH: certs.ssh,
		TLS: certs.tls,
	}, nil
}

func (a *ServerWithRoles) GetSignupU2FRegisterRequest(token string) (*u2f.RegisterChallenge, error) {
	// signup token are their own authz resource
	return a.authServer.CreateSignupU2FRegisterRequest(token)
}

func (a *ServerWithRoles) CreateResetPasswordToken(ctx context.Context, req CreateResetPasswordTokenRequest) (services.ResetPasswordToken, error) {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateResetPasswordToken(ctx, req)
}

func (a *ServerWithRoles) GetResetPasswordToken(ctx context.Context, tokenID string) (services.ResetPasswordToken, error) {
	// tokens are their own authz mechanism, no need to double check
	return a.authServer.GetResetPasswordToken(ctx, tokenID)
}

func (a *ServerWithRoles) RotateResetPasswordTokenSecrets(ctx context.Context, tokenID string) (services.ResetPasswordTokenSecrets, error) {
	// tokens are their own authz mechanism, no need to double check
	return a.authServer.RotateResetPasswordTokenSecrets(ctx, tokenID)
}

func (a *ServerWithRoles) ChangePasswordWithToken(ctx context.Context, req ChangePasswordWithTokenRequest) (services.WebSession, error) {
	// Token is it's own authentication, no need to double check.
	return a.authServer.ChangePasswordWithToken(ctx, req)
}

// CreateUser inserts a new user entry in a backend.
func (a *ServerWithRoles) CreateUser(ctx context.Context, user services.User) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateUser(ctx, user)
}

// UpdateUser updates an existing user in a backend.
// Captures the auth user who modified the user record.
func (a *ServerWithRoles) UpdateUser(ctx context.Context, user services.User) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.UpdateUser(ctx, user)
}

func (a *ServerWithRoles) UpsertUser(u services.User) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	createdBy := u.GetCreatedBy()
	if createdBy.IsEmpty() {
		u.SetCreatedBy(services.CreatedBy{
			User: services.UserRef{Name: a.context.User.GetName()},
		})
	}
	return a.authServer.UpsertUser(u)
}

// UpsertOIDCConnector creates or updates an OIDC connector.
func (a *ServerWithRoles) UpsertOIDCConnector(ctx context.Context, connector services.OIDCConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	if modules.GetModules().Features().OIDC == false {
		return trace.AccessDenied("OIDC is only available in enterprise subscriptions")
	}

	return a.authServer.UpsertOIDCConnector(ctx, connector)
}

func (a *ServerWithRoles) GetOIDCConnector(id string, withSecrets bool) (services.OIDCConnector, error) {
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetOIDCConnector(id, withSecrets)
}

func (a *ServerWithRoles) GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error) {
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetOIDCConnectors(withSecrets)
}

func (a *ServerWithRoles) CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error) {
	if err := a.action(defaults.Namespace, services.KindOIDCRequest, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateOIDCAuthRequest(req)
}

func (a *ServerWithRoles) ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	return a.authServer.ValidateOIDCAuthCallback(q)
}

func (a *ServerWithRoles) DeleteOIDCConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteOIDCConnector(ctx, connectorID)
}

func (a *ServerWithRoles) CreateSAMLConnector(ctx context.Context, connector services.SAMLConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if modules.GetModules().Features().SAML == false {
		return trace.AccessDenied("SAML is only available in enterprise subscriptions")
	}
	return a.authServer.UpsertSAMLConnector(ctx, connector)
}

// UpsertSAMLConnector creates or updates a SAML connector.
func (a *ServerWithRoles) UpsertSAMLConnector(ctx context.Context, connector services.SAMLConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	if modules.GetModules().Features().SAML == false {
		return trace.AccessDenied("SAML is only available in enterprise subscriptions")
	}
	return a.authServer.UpsertSAMLConnector(ctx, connector)
}

func (a *ServerWithRoles) GetSAMLConnector(id string, withSecrets bool) (services.SAMLConnector, error) {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetSAMLConnector(id, withSecrets)
}

func (a *ServerWithRoles) GetSAMLConnectors(withSecrets bool) ([]services.SAMLConnector, error) {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetSAMLConnectors(withSecrets)
}

func (a *ServerWithRoles) CreateSAMLAuthRequest(req services.SAMLAuthRequest) (*services.SAMLAuthRequest, error) {
	if err := a.action(defaults.Namespace, services.KindSAMLRequest, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateSAMLAuthRequest(req)
}

func (a *ServerWithRoles) ValidateSAMLResponse(re string) (*SAMLAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	return a.authServer.ValidateSAMLResponse(re)
}

// DeleteSAMLConnector deletes a SAML connector by name.
func (a *ServerWithRoles) DeleteSAMLConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteSAMLConnector(ctx, connectorID)
}

func (a *ServerWithRoles) CreateGithubConnector(connector services.GithubConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkGithubConnector(connector); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateGithubConnector(connector)
}

func (a *ServerWithRoles) checkGithubConnector(connector services.GithubConnector) error {
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
func (a *ServerWithRoles) UpsertGithubConnector(ctx context.Context, connector services.GithubConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkGithubConnector(connector); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.upsertGithubConnector(ctx, connector)
}

func (a *ServerWithRoles) GetGithubConnector(id string, withSecrets bool) (services.GithubConnector, error) {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetGithubConnector(id, withSecrets)
}

func (a *ServerWithRoles) GetGithubConnectors(withSecrets bool) ([]services.GithubConnector, error) {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetGithubConnectors(withSecrets)
}

// DeleteGithubConnector deletes a Github connector by name.
func (a *ServerWithRoles) DeleteGithubConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.deleteGithubConnector(ctx, connectorID)
}

func (a *ServerWithRoles) CreateGithubAuthRequest(req services.GithubAuthRequest) (*services.GithubAuthRequest, error) {
	if err := a.action(defaults.Namespace, services.KindGithubRequest, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateGithubAuthRequest(req)
}

func (a *ServerWithRoles) ValidateGithubAuthCallback(q url.Values) (*GithubAuthResponse, error) {
	return a.authServer.ValidateGithubAuthCallback(q)
}

// EmitAuditEvent emits a single audit event
func (a *ServerWithRoles) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbCreate); err != nil {
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
func (a *ServerWithRoles) CreateAuditStream(ctx context.Context, sid session.ID) (events.Stream, error) {
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbUpdate); err != nil {
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
func (a *ServerWithRoles) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (events.Stream, error) {
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbUpdate); err != nil {
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
	stream   events.Stream
}

// Status returns channel receiving updates about stream status
// last event index that was uploaded and upload ID
func (s *streamWithRoles) Status() <-chan events.StreamStatus {
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

func (s *streamWithRoles) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
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
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.EmitAuditEventLegacy(event, fields)
}

func (a *ServerWithRoles) PostSessionSlice(slice events.SessionSlice) error {
	if err := a.action(slice.Namespace, services.KindEvent, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(slice.Namespace, services.KindEvent, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.PostSessionSlice(slice)
}

func (a *ServerWithRoles) UploadSessionRecording(r events.SessionRecording) error {
	if err := r.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(r.Namespace, services.KindEvent, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(r.Namespace, services.KindEvent, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.UploadSessionRecording(r)
}

func (a *ServerWithRoles) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	if err := a.action(namespace, services.KindSession, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionChunk(namespace, sid, offsetBytes, maxBytes)
}

func (a *ServerWithRoles) GetSessionEvents(namespace string, sid session.ID, afterN int, includePrintEvents bool) ([]events.EventFields, error) {
	if err := a.action(namespace, services.KindSession, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionEvents(namespace, sid, afterN, includePrintEvents)
}

func (a *ServerWithRoles) SearchEvents(from, to time.Time, query string, limit int) ([]events.EventFields, error) {
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.SearchEvents(from, to, query, limit)
}

func (a *ServerWithRoles) SearchSessionEvents(from, to time.Time, limit int) ([]events.EventFields, error) {
	if err := a.action(defaults.Namespace, services.KindSession, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.SearchSessionEvents(from, to, limit)
}

// GetNamespaces returns a list of namespaces
func (a *ServerWithRoles) GetNamespaces() ([]services.Namespace, error) {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNamespaces()
}

// GetNamespace returns namespace by name
func (a *ServerWithRoles) GetNamespace(name string) (*services.Namespace, error) {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNamespace(name)
}

// UpsertNamespace upserts namespace
func (a *ServerWithRoles) UpsertNamespace(ns services.Namespace) error {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertNamespace(ns)
}

// DeleteNamespace deletes namespace by name
func (a *ServerWithRoles) DeleteNamespace(name string) error {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteNamespace(name)
}

// GetRoles returns a list of roles
func (a *ServerWithRoles) GetRoles(ctx context.Context) ([]services.Role, error) {
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetRoles(ctx)
}

// CreateRole not implemented: can only be called locally.
func (a *ServerWithRoles) CreateRole(role services.Role) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpsertRole creates or updates role.
func (a *ServerWithRoles) UpsertRole(ctx context.Context, role services.Role) error {
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	// Some options are only available with enterprise subscription
	features := modules.GetModules().Features()
	options := role.GetOptions()

	switch {
	case features.AccessControls == false && options.MaxSessions > 0:
		return trace.AccessDenied(
			"role option max_sessions is only available in enterprise subscriptions")
	case features.AdvancedAccessWorkflows == false &&
		(options.RequestAccess == types.RequestStrategyReason || options.RequestAccess == types.RequestStrategyAlways):
		return trace.AccessDenied(
			"role option request_access: %v is only available in enterprise subscriptions", options.RequestAccess)
	}

	return a.authServer.upsertRole(ctx, role)
}

// GetRole returns role by name
func (a *ServerWithRoles) GetRole(ctx context.Context, name string) (services.Role, error) {
	// Current-user exception: we always allow users to read roles
	// that they hold.  This requirement is checked first to avoid
	// misleading denial messages in the logs.
	if !utils.SliceContainsStr(a.context.User.GetRoles(), name) {
		if err := a.action(defaults.Namespace, services.KindRole, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetRole(ctx, name)
}

// DeleteRole deletes role by name
func (a *ServerWithRoles) DeleteRole(ctx context.Context, name string) error {
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbDelete); err != nil {
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

// GetClusterConfig gets cluster level configuration.
func (a *ServerWithRoles) GetClusterConfig(opts ...services.MarshalOption) (services.ClusterConfig, error) {
	if err := a.action(defaults.Namespace, services.KindClusterConfig, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetClusterConfig(opts...)
}

// DeleteClusterConfig deletes cluster config
func (a *ServerWithRoles) DeleteClusterConfig() error {
	if err := a.action(defaults.Namespace, services.KindClusterConfig, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteClusterConfig()
}

// DeleteClusterName deletes cluster name
func (a *ServerWithRoles) DeleteClusterName() error {
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteClusterName()
}

// DeleteStaticTokens deletes static tokens
func (a *ServerWithRoles) DeleteStaticTokens() error {
	if err := a.action(defaults.Namespace, services.KindStaticTokens, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteStaticTokens()
}

// SetClusterConfig sets cluster level configuration.
func (a *ServerWithRoles) SetClusterConfig(c services.ClusterConfig) error {
	if err := a.action(defaults.Namespace, services.KindClusterConfig, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindClusterConfig, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetClusterConfig(c)
}

// GetClusterName gets the name of the cluster.
func (a *ServerWithRoles) GetClusterName(opts ...services.MarshalOption) (services.ClusterName, error) {
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetClusterName()
}

// SetClusterName sets the name of the cluster. SetClusterName can only be called once.
func (a *ServerWithRoles) SetClusterName(c services.ClusterName) error {
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetClusterName(c)
}

// UpsertClusterName sets the name of the cluster.
func (a *ServerWithRoles) UpsertClusterName(c services.ClusterName) error {
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertClusterName(c)
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (a *ServerWithRoles) GetStaticTokens() (services.StaticTokens, error) {
	if err := a.action(defaults.Namespace, services.KindStaticTokens, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetStaticTokens()
}

// SetStaticTokens sets the list of static tokens used to provision nodes.
func (a *ServerWithRoles) SetStaticTokens(s services.StaticTokens) error {
	if err := a.action(defaults.Namespace, services.KindStaticTokens, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindStaticTokens, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetStaticTokens(s)
}

func (a *ServerWithRoles) GetAuthPreference() (services.AuthPreference, error) {
	if err := a.action(defaults.Namespace, services.KindClusterAuthPreference, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetAuthPreference()
}

func (a *ServerWithRoles) SetAuthPreference(cap services.AuthPreference) error {
	if err := a.action(defaults.Namespace, services.KindClusterAuthPreference, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindClusterAuthPreference, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.SetAuthPreference(cap)
}

// DeleteAuthPreference not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAuthPreference(context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllTokens not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAllTokens() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllCertAuthorities not implemented: can only be called locally.
func (a *ServerWithRoles) DeleteAllCertAuthorities(caType services.CertAuthType) error {
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

func (a *ServerWithRoles) GetTrustedClusters() ([]services.TrustedCluster, error) {
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetTrustedClusters()
}

func (a *ServerWithRoles) GetTrustedCluster(name string) (services.TrustedCluster, error) {
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetTrustedCluster(name)
}

// UpsertTrustedCluster creates or updates a trusted cluster.
func (a *ServerWithRoles) UpsertTrustedCluster(ctx context.Context, tc services.TrustedCluster) (services.TrustedCluster, error) {
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbUpdate); err != nil {
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
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteTrustedCluster(ctx, name)
}

func (a *ServerWithRoles) UpsertTunnelConnection(conn services.TunnelConnection) error {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertTunnelConnection(conn)
}

func (a *ServerWithRoles) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]services.TunnelConnection, error) {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetTunnelConnections(clusterName, opts...)
}

func (a *ServerWithRoles) GetAllTunnelConnections(opts ...services.MarshalOption) ([]services.TunnelConnection, error) {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetAllTunnelConnections(opts...)
}

func (a *ServerWithRoles) DeleteTunnelConnection(clusterName string, connName string) error {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteTunnelConnection(clusterName, connName)
}

func (a *ServerWithRoles) DeleteTunnelConnections(clusterName string) error {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteTunnelConnections(clusterName)
}

func (a *ServerWithRoles) DeleteAllTunnelConnections() error {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllTunnelConnections()
}

func (a *ServerWithRoles) CreateRemoteCluster(conn services.RemoteCluster) error {
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateRemoteCluster(conn)
}

func (a *ServerWithRoles) UpdateRemoteCluster(ctx context.Context, rc services.RemoteCluster) error {
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpdateRemoteCluster(ctx, rc)
}

func (a *ServerWithRoles) GetRemoteCluster(clusterName string) (services.RemoteCluster, error) {
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbRead); err != nil {
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

func (a *ServerWithRoles) GetRemoteClusters(opts ...services.MarshalOption) ([]services.RemoteCluster, error) {
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	remoteClusters, err := a.authServer.GetRemoteClusters(opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.filterRemoteClustersForUser(remoteClusters)
}

// filterRemoteClustersForUser filters remote clusters based on what the current user is authorized to access
func (a *ServerWithRoles) filterRemoteClustersForUser(remoteClusters []services.RemoteCluster) ([]services.RemoteCluster, error) {
	filteredClusters := make([]services.RemoteCluster, 0, len(remoteClusters))
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
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteRemoteCluster(clusterName)
}

func (a *ServerWithRoles) DeleteAllRemoteClusters() error {
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllRemoteClusters()
}

// AcquireSemaphore acquires lease with requested resources from semaphore.
func (a *ServerWithRoles) AcquireSemaphore(ctx context.Context, params services.AcquireSemaphoreRequest) (*services.SemaphoreLease, error) {
	if err := a.action(defaults.Namespace, services.KindSemaphore, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindSemaphore, services.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.AcquireSemaphore(ctx, params)
}

// KeepAliveSemaphoreLease updates semaphore lease.
func (a *ServerWithRoles) KeepAliveSemaphoreLease(ctx context.Context, lease services.SemaphoreLease) error {
	if err := a.action(defaults.Namespace, services.KindSemaphore, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.KeepAliveSemaphoreLease(ctx, lease)
}

// CancelSemaphoreLease cancels semaphore lease early.
func (a *ServerWithRoles) CancelSemaphoreLease(ctx context.Context, lease services.SemaphoreLease) error {
	if err := a.action(defaults.Namespace, services.KindSemaphore, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CancelSemaphoreLease(ctx, lease)
}

// GetSemaphores returns a list of all semaphores matching the supplied filter.
func (a *ServerWithRoles) GetSemaphores(ctx context.Context, filter services.SemaphoreFilter) ([]services.Semaphore, error) {
	if err := a.action(defaults.Namespace, services.KindSemaphore, services.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindSemaphore, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetSemaphores(ctx, filter)
}

// DeleteSemaphore deletes a semaphore matching the supplied filter.
func (a *ServerWithRoles) DeleteSemaphore(ctx context.Context, filter services.SemaphoreFilter) error {
	if err := a.action(defaults.Namespace, services.KindSemaphore, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteSemaphore(ctx, filter)
}

// ProcessKubeCSR processes CSR request against Kubernetes CA, returns
// signed certificate if successful.
func (a *ServerWithRoles) ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error) {
	// limits the requests types to proxies to make it harder to break
	if !a.hasBuiltinRole(string(teleport.RoleProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.ProcessKubeCSR(req)
}

// GetDatabaseServers returns all registered database servers.
func (a *ServerWithRoles) GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error) {
	if err := a.action(namespace, types.KindDatabaseServer, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(namespace, types.KindDatabaseServer, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := a.authServer.GetDatabaseServers(ctx, namespace, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Filter out databases the caller doesn't have access to from each server.
	var filtered []types.DatabaseServer
	// MFA is not required to list the databases, but will be required to
	// connect to them.
	mfaParams := services.AccessMFAParams{Verified: true}
	for _, server := range servers {
		err := a.context.Checker.CheckAccessToDatabase(server, mfaParams, &services.DatabaseLabelsMatcher{Labels: server.GetAllLabels()})
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
	if err := a.action(server.GetNamespace(), types.KindDatabaseServer, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(server.GetNamespace(), types.KindDatabaseServer, types.VerbUpdate); err != nil {
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
	if err := a.action(namespace, types.KindDatabaseServer, types.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(namespace, types.KindDatabaseServer, types.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllDatabaseServers(ctx, namespace)
}

// SignDatabaseCSR generates a client certificate used by proxy when talking
// to a remote database service.
func (a *ServerWithRoles) SignDatabaseCSR(ctx context.Context, req *proto.DatabaseCSRRequest) (*proto.DatabaseCSRResponse, error) {
	// Only proxy is allowed to request this certificate when proxying
	// database client connection to a remote database service.
	if !a.hasBuiltinRole(string(teleport.RoleProxy)) {
		return nil, trace.AccessDenied("this request can only be executed by a proxy service")
	}
	return a.authServer.SignDatabaseCSR(ctx, req)
}

// GenerateDatabaseCert generates a certificate used by a database service
// to authenticate with the database instance
func (a *ServerWithRoles) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	// This certificate can be requested only by a database service when
	// initiating connection to a database instance, or by an admin when
	// generating certificates for a database instance.
	if !a.hasBuiltinRole(string(teleport.RoleDatabase)) && !a.hasBuiltinRole(string(teleport.RoleAdmin)) {
		return nil, trace.AccessDenied("this request can only be executed by a database service or an admin")
	}
	return a.authServer.GenerateDatabaseCert(ctx, req)
}

// GetAppServers gets all application servers.
func (a *ServerWithRoles) GetAppServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]services.Server, error) {
	if err := a.action(namespace, services.KindAppServer, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(namespace, services.KindAppServer, services.VerbRead); err != nil {
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
		filteredApps := make([]*services.App, 0, len(server.GetApps()))
		for _, app := range server.GetApps() {
			err := a.context.Checker.CheckAccessToApp(server.GetNamespace(), app, mfaParams)
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
func (a *ServerWithRoles) UpsertAppServer(ctx context.Context, server services.Server) (*services.KeepAlive, error) {
	if err := a.action(server.GetNamespace(), services.KindAppServer, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(server.GetNamespace(), services.KindAppServer, services.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.UpsertAppServer(ctx, server)
}

// DeleteAppServer removes an application server.
func (a *ServerWithRoles) DeleteAppServer(ctx context.Context, namespace string, name string) error {
	if err := a.action(namespace, services.KindAppServer, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.DeleteAppServer(ctx, namespace, name); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllAppServers removes all application servers.
func (a *ServerWithRoles) DeleteAllAppServers(ctx context.Context, namespace string) error {
	if err := a.action(namespace, services.KindAppServer, services.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(namespace, services.KindAppServer, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.DeleteAllAppServers(ctx, namespace); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetAppSession gets an application web session.
func (a *ServerWithRoles) GetAppSession(ctx context.Context, req services.GetAppSessionRequest) (services.WebSession, error) {
	session, err := a.authServer.GetAppSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Users can only fetch their own app sessions.
	if err := a.currentUserAction(session.GetUser()); err != nil {
		if err := a.action(defaults.Namespace, services.KindWebSession, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return session, nil
}

// GetAppSessions gets all application web sessions.
func (a *ServerWithRoles) GetAppSessions(ctx context.Context) ([]services.WebSession, error) {
	if err := a.action(defaults.Namespace, services.KindWebSession, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindWebSession, services.VerbRead); err != nil {
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
func (a *ServerWithRoles) CreateAppSession(ctx context.Context, req services.CreateAppSessionRequest) (services.WebSession, error) {
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
func (a *ServerWithRoles) UpsertAppSession(ctx context.Context, session services.WebSession) error {
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
		if err := a.action(defaults.Namespace, services.KindWebSession, services.VerbDelete); err != nil {
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
	if err := a.action(defaults.Namespace, services.KindWebSession, services.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindWebSession, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	if err := a.authServer.DeleteAllAppSessions(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GenerateAppToken creates a JWT token with application access.
func (a *ServerWithRoles) GenerateAppToken(ctx context.Context, req jwt.GenerateAppTokenRequest) (string, error) {
	if err := a.action(defaults.Namespace, services.KindJWT, services.VerbCreate); err != nil {
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
func (a *ServerWithRoles) UpsertKubeService(ctx context.Context, s services.Server) error {
	if err := a.action(defaults.Namespace, services.KindKubeService, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindKubeService, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	ap, err := a.authServer.GetAuthPreference()
	if err != nil {
		return trace.Wrap(err)
	}
	mfaParams := services.AccessMFAParams{
		Verified:       a.context.Identity.GetIdentity().MFAVerified != "",
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
func (a *ServerWithRoles) GetKubeServices(ctx context.Context) ([]services.Server, error) {
	if err := a.action(defaults.Namespace, services.KindKubeService, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindKubeService, services.VerbRead); err != nil {
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
		filtered := make([]*services.KubernetesCluster, 0, len(server.GetKubernetesClusters()))
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
	if err := a.action(defaults.Namespace, services.KindKubeService, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteKubeService(ctx, name)
}

// DeleteAllKubeService deletes all registered kubernetes services.
func (a *ServerWithRoles) DeleteAllKubeServices(ctx context.Context) error {
	if err := a.action(defaults.Namespace, services.KindKubeService, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllKubeServices(ctx)
}

// TODO(awly): decouple auth.ClientI from auth.ServerWithRoles, they exist on
// opposite sides of the connection.

// GetMFADevices exists to satisfy auth.ClientI but is not implemented here.
// Use auth.GRPCServer.GetMFADevices or client.Client.GetMFADevices instead.
func (a *ServerWithRoles) GetMFADevices(context.Context, *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error) {
	return nil, trace.NotImplemented("bug: GetMFADevices must not be called on auth.ServerWithRoles")
}

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
