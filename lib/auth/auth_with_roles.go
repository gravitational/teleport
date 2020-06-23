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

package auth

import (
	"context"
	"net/url"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/proto"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/wrappers"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"github.com/tstranex/u2f"
)

// AuthWithRoles is a wrapper around auth service
// methods that focuses on authorizing every request
type AuthWithRoles struct {
	authServer *AuthServer
	checker    services.AccessChecker
	user       services.User
	sessions   session.Service
	alog       events.IAuditLog
	identity   tlsca.Identity
}

func (a *AuthWithRoles) actionWithContext(ctx *services.Context, namespace string, resource string, action string) error {
	return a.checker.CheckAccessToRule(ctx, namespace, resource, action, false)
}

func (a *AuthWithRoles) action(namespace string, resource string, action string) error {
	return a.checker.CheckAccessToRule(&services.Context{User: a.user}, namespace, resource, action, false)
}

// currentUserAction is a special checker that allows certain actions for users
// even if they are not admins, e.g. update their own passwords,
// or generate certificates, otherwise it will require admin privileges
func (a *AuthWithRoles) currentUserAction(username string) error {
	if a.hasLocalUserRole(a.checker) && username == a.user.GetName() {
		return nil
	}
	return a.checker.CheckAccessToRule(&services.Context{User: a.user},
		defaults.Namespace, services.KindUser, services.VerbCreate, true)
}

// authConnectorAction is a special checker that grants access to auth
// connectors. It first checks if you have access to the specific connector.
// If not, it checks if the requester has the meta KindAuthConnector access
// (which grants access to all connectors).
func (a *AuthWithRoles) authConnectorAction(namespace string, resource string, verb string) error {
	if err := a.checker.CheckAccessToRule(&services.Context{User: a.user}, namespace, resource, verb, false); err != nil {
		if err := a.checker.CheckAccessToRule(&services.Context{User: a.user}, namespace, services.KindAuthConnector, verb, false); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// hasBuiltinRole checks the type of the role set returned and the name.
// Returns true if role set is builtin and the name matches.
func (a *AuthWithRoles) hasBuiltinRole(name string) bool {
	return hasBuiltinRole(a.checker, name)
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
func (a *AuthWithRoles) hasRemoteBuiltinRole(name string) bool {
	if _, ok := a.checker.(RemoteBuiltinRoleSet); !ok {
		return false
	}
	if !a.checker.HasRole(name) {
		return false
	}

	return true
}

// hasLocalUserRole checks if the type of the role set is a local user or not.
func (a *AuthWithRoles) hasLocalUserRole(checker services.AccessChecker) bool {
	if _, ok := checker.(LocalUserRoleSet); !ok {
		return false
	}
	return true
}

// AuthenticateWebUser authenticates web user, creates and  returns web session
// in case if authentication is successful
func (a *AuthWithRoles) AuthenticateWebUser(req AuthenticateUserRequest) (services.WebSession, error) {
	// authentication request has it's own authentication, however this limits the requests
	// types to proxies to make it harder to break
	if !a.hasBuiltinRole(string(teleport.RoleProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.AuthenticateWebUser(req)
}

// AuthenticateSSHUser authenticates SSH console user, creates and  returns a pair of signed TLS and SSH
// short lived certificates as a result
func (a *AuthWithRoles) AuthenticateSSHUser(req AuthenticateSSHRequest) (*SSHLoginResponse, error) {
	// authentication request has it's own authentication, however this limits the requests
	// types to proxies to make it harder to break
	if !a.hasBuiltinRole(string(teleport.RoleProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.AuthenticateSSHUser(req)
}

func (a *AuthWithRoles) GetSessions(namespace string) ([]session.Session, error) {
	if err := a.action(namespace, services.KindSSHSession, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.sessions.GetSessions(namespace)
}

func (a *AuthWithRoles) GetSession(namespace string, id session.ID) (*session.Session, error) {
	if err := a.action(namespace, services.KindSSHSession, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.sessions.GetSession(namespace, id)
}

func (a *AuthWithRoles) CreateSession(s session.Session) error {
	if err := a.action(s.Namespace, services.KindSSHSession, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.CreateSession(s)
}

func (a *AuthWithRoles) UpdateSession(req session.UpdateRequest) error {
	if err := a.action(req.Namespace, services.KindSSHSession, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.UpdateSession(req)
}

// DeleteSession removes an active session from the backend.
func (a *AuthWithRoles) DeleteSession(namespace string, id session.ID) error {
	if err := a.action(namespace, services.KindSSHSession, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.DeleteSession(namespace, id)
}

func (a *AuthWithRoles) CreateCertAuthority(ca services.CertAuthority) error {
	return trace.NotImplemented("not implemented")
}

// RotateCertAuthority starts or restarts certificate authority rotation process.
func (a *AuthWithRoles) RotateCertAuthority(req RotateRequest) error {
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
func (a *AuthWithRoles) RotateExternalCertAuthority(ca services.CertAuthority) error {
	if ca == nil {
		return trace.BadParameter("missing certificate authority")
	}
	ctx := &services.Context{User: a.user, Resource: ca}
	if err := a.actionWithContext(ctx, defaults.Namespace, services.KindCertAuthority, services.VerbRotate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.RotateExternalCertAuthority(ca)
}

// UpsertCertAuthority updates existing cert authority or updates the existing one.
func (a *AuthWithRoles) UpsertCertAuthority(ca services.CertAuthority) error {
	if ca == nil {
		return trace.BadParameter("missing certificate authority")
	}
	ctx := &services.Context{User: a.user, Resource: ca}
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
func (a *AuthWithRoles) CompareAndSwapCertAuthority(new, existing services.CertAuthority) error {
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CompareAndSwapCertAuthority(new, existing)
}

func (a *AuthWithRoles) GetCertAuthorities(caType services.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]services.CertAuthority, error) {
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

func (a *AuthWithRoles) GetCertAuthority(id services.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (services.CertAuthority, error) {
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

func (a *AuthWithRoles) GetDomainName() (string, error) {
	// anyone can read it, no harm in that
	return a.authServer.GetDomainName()
}

func (a *AuthWithRoles) GetLocalClusterName() (string, error) {
	// anyone can read it, no harm in that
	return a.authServer.GetLocalClusterName()
}

// GetClusterCACert returns the CAs for the local cluster without signing keys.
func (a *AuthWithRoles) GetClusterCACert() (*LocalCAResponse, error) {
	// Allow all roles to get the local CA.
	return a.authServer.GetClusterCACert()
}

func (a *AuthWithRoles) UpsertLocalClusterName(clusterName string) error {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertLocalClusterName(clusterName)
}

func (a *AuthWithRoles) DeleteCertAuthority(id services.CertAuthID) error {
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteCertAuthority(id)
}

func (a *AuthWithRoles) ActivateCertAuthority(id services.CertAuthID) error {
	return trace.NotImplemented("not implemented")
}

func (a *AuthWithRoles) DeactivateCertAuthority(id services.CertAuthID) error {
	return trace.NotImplemented("not implemented")
}

// GenerateToken generates multi-purpose authentication token.
func (a *AuthWithRoles) GenerateToken(ctx context.Context, req GenerateTokenRequest) (string, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.GenerateToken(ctx, req)
}

func (a *AuthWithRoles) RegisterUsingToken(req RegisterUsingTokenRequest) (*PackedKeys, error) {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterUsingToken(req)
}

func (a *AuthWithRoles) RegisterNewAuthServer(token string) error {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterNewAuthServer(token)
}

// GenerateServerKeys generates new host private keys and certificates (signed
// by the host certificate authority) for a node.
func (a *AuthWithRoles) GenerateServerKeys(req GenerateServerKeysRequest) (*PackedKeys, error) {
	clusterName, err := a.authServer.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// username is hostID + cluster name, so make sure server requests new keys for itself
	if a.user.GetName() != HostFQDN(req.HostID, clusterName) {
		return nil, trace.AccessDenied("username mismatch %q and %q", a.user.GetName(), HostFQDN(req.HostID, clusterName))
	}
	existingRoles, err := teleport.NewRoles(a.user.GetRoles())
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
func (a *AuthWithRoles) UpsertNodes(namespace string, servers []services.Server) error {
	if err := a.action(namespace, services.KindNode, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(namespace, services.KindNode, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertNodes(namespace, servers)
}

func (a *AuthWithRoles) UpsertNode(s services.Server) (*services.KeepAlive, error) {
	if err := a.action(s.GetNamespace(), services.KindNode, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(s.GetNamespace(), services.KindNode, services.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.UpsertNode(s)
}

func (a *AuthWithRoles) KeepAliveNode(ctx context.Context, handle services.KeepAlive) error {
	if !a.hasBuiltinRole(string(teleport.RoleNode)) {
		return trace.AccessDenied("[10] access denied")
	}
	clusterName, err := a.GetDomainName()
	if err != nil {
		return trace.Wrap(err)
	}
	serverName, err := ExtractHostID(a.user.GetName(), clusterName)
	if err != nil {
		return trace.AccessDenied("[10] access denied")
	}
	if serverName != handle.ServerName {
		return trace.AccessDenied("[10] access denied")
	}
	if err := a.action(defaults.Namespace, services.KindNode, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.KeepAliveNode(ctx, handle)
}

// NewWatcher returns a new event watcher
func (a *AuthWithRoles) NewWatcher(ctx context.Context, watch services.Watch) (services.Watcher, error) {
	if len(watch.Kinds) == 0 {
		return nil, trace.AccessDenied("can't setup global watch")
	}
	for _, kind := range watch.Kinds {
		switch kind.Kind {
		case services.KindNamespace:
			if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindUser:
			if err := a.action(defaults.Namespace, services.KindUser, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindRole:
			if err := a.action(defaults.Namespace, services.KindRole, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindNode:
			if err := a.action(defaults.Namespace, services.KindNode, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindProxy:
			if err := a.action(defaults.Namespace, services.KindProxy, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindAuthServer:
			if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindTunnelConnection:
			if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindReverseTunnel:
			if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindClusterConfig:
			if err := a.action(defaults.Namespace, services.KindClusterConfig, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindClusterName:
			if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindToken:
			if err := a.action(defaults.Namespace, services.KindToken, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		case services.KindStaticTokens:
			if err := a.action(defaults.Namespace, services.KindStaticTokens, services.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
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
		default:
			return nil, trace.AccessDenied("not authorized to watch %v events", kind.Kind)
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
func (a *AuthWithRoles) filterNodes(nodes []services.Server) ([]services.Server, error) {
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

	roleset, err := services.FetchRoles(a.user.GetRoles(), a.authServer, a.user.GetTraits())
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
NextNode:
	for _, node := range nodes {
		for login := range allowedLogins {
			err := roleset.CheckAccessToServer(login, node)
			if err == nil {
				filteredNodes = append(filteredNodes, node)
				continue NextNode
			}
		}
	}

	return filteredNodes, nil
}

// DeleteAllNodes deletes all nodes in a given namespace
func (a *AuthWithRoles) DeleteAllNodes(namespace string) error {
	if err := a.action(namespace, services.KindNode, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllNodes(namespace)
}

// DeleteNode deletes node in the namespace
func (a *AuthWithRoles) DeleteNode(namespace, node string) error {
	if err := a.action(namespace, services.KindNode, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteNode(namespace, node)
}

func (a *AuthWithRoles) GetNodes(namespace string, opts ...services.MarshalOption) ([]services.Server, error) {
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
		"user":           a.user.GetName(),
		"elapsed_fetch":  elapsedFetch,
		"elapsed_filter": elapsedFilter,
	}).Debugf(
		"GetServers(%v->%v) in %v.",
		len(nodes), len(filteredNodes), elapsedFetch+elapsedFilter)

	return filteredNodes, nil
}

func (a *AuthWithRoles) UpsertAuthServer(s services.Server) error {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertAuthServer(s)
}

func (a *AuthWithRoles) GetAuthServers() ([]services.Server, error) {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetAuthServers()
}

// DeleteAllAuthServers deletes all auth servers
func (a *AuthWithRoles) DeleteAllAuthServers() error {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllAuthServers()
}

// DeleteAuthServer deletes auth server by name
func (a *AuthWithRoles) DeleteAuthServer(name string) error {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAuthServer(name)
}

func (a *AuthWithRoles) UpsertProxy(s services.Server) error {
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertProxy(s)
}

func (a *AuthWithRoles) GetProxies() ([]services.Server, error) {
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetProxies()
}

// DeleteAllProxies deletes all proxies
func (a *AuthWithRoles) DeleteAllProxies() error {
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllProxies()
}

// DeleteProxy deletes proxy by name
func (a *AuthWithRoles) DeleteProxy(name string) error {
	if err := a.action(defaults.Namespace, services.KindProxy, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteProxy(name)
}

func (a *AuthWithRoles) UpsertReverseTunnel(r services.ReverseTunnel) error {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertReverseTunnel(r)
}

func (a *AuthWithRoles) GetReverseTunnel(name string, opts ...services.MarshalOption) (services.ReverseTunnel, error) {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnel(name, opts...)
}

func (a *AuthWithRoles) GetReverseTunnels(opts ...services.MarshalOption) ([]services.ReverseTunnel, error) {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnels(opts...)
}

func (a *AuthWithRoles) DeleteReverseTunnel(domainName string) error {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteReverseTunnel(domainName)
}

func (a *AuthWithRoles) DeleteToken(token string) error {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteToken(token)
}

func (a *AuthWithRoles) GetTokens(opts ...services.MarshalOption) ([]services.ProvisionToken, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetTokens(opts...)
}

func (a *AuthWithRoles) GetToken(token string) (services.ProvisionToken, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetToken(token)
}

func (a *AuthWithRoles) UpsertToken(token services.ProvisionToken) error {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertToken(token)
}

func (a *AuthWithRoles) UpsertPassword(user string, password []byte) error {
	if err := a.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertPassword(user, password)
}

// ChangePassword updates users password based on the old password.
func (a *AuthWithRoles) ChangePassword(req services.ChangePasswordReq) error {
	if err := a.currentUserAction(req.User); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.ChangePassword(req)
}

func (a *AuthWithRoles) CheckPassword(user string, password []byte, otpToken string) error {
	if err := a.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CheckPassword(user, password, otpToken)
}

func (a *AuthWithRoles) UpsertTOTP(user string, otpSecret string) error {
	if err := a.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertTOTP(user, otpSecret)
}

func (a *AuthWithRoles) PreAuthenticatedSignIn(user string) (services.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.PreAuthenticatedSignIn(user, &a.identity)
}

func (a *AuthWithRoles) GetU2FSignRequest(user string, password []byte) (*u2f.SignRequest, error) {
	// we are already checking password here, no need to extra permission check
	// anyone who has user's password can generate sign request
	return a.authServer.U2FSignRequest(user, password)
}

func (a *AuthWithRoles) CreateWebSession(user string) (services.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateWebSession(user)
}

func (a *AuthWithRoles) ExtendWebSession(user, prevSessionID string) (services.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.ExtendWebSession(user, prevSessionID, &a.identity)
}

func (a *AuthWithRoles) GetWebSessionInfo(user string, sid string) (services.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetWebSessionInfo(user, sid)
}

func (a *AuthWithRoles) DeleteWebSession(user string, sid string) error {
	if err := a.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteWebSession(user, sid)
}

func (a *AuthWithRoles) GetAccessRequests(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error) {
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

func (a *AuthWithRoles) CreateAccessRequest(ctx context.Context, req services.AccessRequest) error {
	// An exception is made to allow users to create access *pending* requests for themselves.
	if !req.GetState().IsPending() || a.currentUserAction(req.GetUser()) != nil {
		if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbCreate); err != nil {
			return trace.Wrap(err)
		}
	}
	// Ensure that an access request cannot outlive the identity that creates it.
	if req.GetAccessExpiry().Before(a.authServer.GetClock().Now()) || req.GetAccessExpiry().After(a.identity.Expires) {
		req.SetAccessExpiry(a.identity.Expires)
	}
	return a.authServer.CreateAccessRequest(ctx, req)
}

func (a *AuthWithRoles) SetAccessRequestState(ctx context.Context, reqID string, state services.RequestState) error {
	if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetAccessRequestState(ctx, reqID, state)
}

// GetPluginData loads all plugin data matching the supplied filter.
func (a *AuthWithRoles) GetPluginData(ctx context.Context, filter services.PluginDataFilter) ([]services.PluginData, error) {
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
func (a *AuthWithRoles) UpdatePluginData(ctx context.Context, params services.PluginDataUpdateParams) error {
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
func (a *AuthWithRoles) Ping(ctx context.Context) (proto.PingResponse, error) {
	// The Ping method does not require special permissions since it only returns
	// basic status information.  This is an intentional design choice.  Alternative
	// methods should be used for relaying any sensitive information.
	cn, err := a.authServer.GetClusterName()
	if err != nil {
		return proto.PingResponse{}, trace.Wrap(err)
	}
	return proto.PingResponse{
		ClusterName:   cn.GetClusterName(),
		ServerVersion: teleport.Version,
	}, nil
}

type contextKey string

// WithDelegator creates a child context with the AccessRequestDelegator
// value set.  Optionally used by AuthServer.SetAccessRequestState to log
// a delegating identity.
func WithDelegator(ctx context.Context, delegator string) context.Context {
	return context.WithValue(ctx, contextKey(events.AccessRequestDelegator), delegator)
}

// getDelegator attempts to load the context value AccessRequestDelegator,
// returning the empty string if no value was found.
func getDelegator(ctx context.Context) string {
	delegator, ok := ctx.Value(contextKey(events.AccessRequestDelegator)).(string)
	if !ok {
		return ""
	}
	return delegator
}

func (a *AuthWithRoles) DeleteAccessRequest(ctx context.Context, name string) error {
	if err := a.action(defaults.Namespace, services.KindAccessRequest, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAccessRequest(ctx, name)
}

func (a *AuthWithRoles) GetUsers(withSecrets bool) ([]services.User, error) {
	if withSecrets {
		// TODO(fspmarshall): replace admin requirement with VerbReadWithSecrets once we've
		// migrated to that model.
		if !a.hasBuiltinRole(string(teleport.RoleAdmin)) {
			err := trace.AccessDenied("user %q requested access to all users with secrets", a.user.GetName())
			log.Warning(err)
			if err := a.authServer.EmitAuditEvent(events.UserLocalLoginFailure, events.EventFields{
				events.LoginMethod:        events.LoginMethodClientCert,
				events.AuthAttemptSuccess: false,
				// log the original internal error in audit log
				events.AuthAttemptErr: trace.Unwrap(err).Error(),
			}); err != nil {
				log.Warnf("Failed to emit local login failure event: %v", err)
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

func (a *AuthWithRoles) GetUser(name string, withSecrets bool) (services.User, error) {
	if withSecrets {
		// TODO(fspmarshall): replace admin requirement with VerbReadWithSecrets once we've
		// migrated to that model.
		if !a.hasBuiltinRole(string(teleport.RoleAdmin)) {
			err := trace.AccessDenied("user %q requested access to user %q with secrets", a.user.GetName(), name)
			log.Warning(err)
			if err := a.authServer.EmitAuditEvent(events.UserLocalLoginFailure, events.EventFields{
				events.LoginMethod:        events.LoginMethodClientCert,
				events.AuthAttemptSuccess: false,
				// log the original internal error in audit log
				events.AuthAttemptErr: trace.Unwrap(err).Error(),
			}); err != nil {
				log.Warnf("Failed to emit local login failure event: %v", err)
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
func (a *AuthWithRoles) DeleteUser(ctx context.Context, user string) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteUser(ctx, user)
}

func (a *AuthWithRoles) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	if err := a.action(defaults.Namespace, services.KindKeyPair, services.VerbCreate); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return a.authServer.GenerateKeyPair(pass)
}

func (a *AuthWithRoles) GenerateHostCert(
	key []byte, hostID, nodeName string, principals []string, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error) {

	if err := a.action(defaults.Namespace, services.KindHostCert, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateHostCert(key, hostID, nodeName, principals, clusterName, roles, ttl)
}

// NewKeepAliver returns a new instance of keep aliver
func (a *AuthWithRoles) NewKeepAliver(ctx context.Context) (services.KeepAliver, error) {
	return nil, trace.NotImplemented("not implemented")
}

// GenerateUserCerts generates users certificates
func (a *AuthWithRoles) GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
	var err error
	var roles []string
	var traits wrappers.Traits

	switch {
	case a.hasBuiltinRole(string(teleport.RoleAdmin)):
		// If it's an admin generating the certificate, the roles and traits for
		// the user have to be fetched from the backend. This should be safe since
		// this is typically done against a local user.
		user, err := a.GetUser(req.Username, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = user.GetRoles()
		traits = user.GetTraits()
	case req.Username == a.user.GetName():
		// user is requesting TTL for themselves,
		// limit the TTL to the duration of the session, to prevent
		// users renewing their certificates forever
		if a.identity.Expires.IsZero() {
			log.Warningf("Encountered identity with no expiry: %v and denied request. Must be internal logic error.", a.identity)
			return nil, trace.AccessDenied("access denied")
		}
		req.Expires = a.identity.Expires
		if req.Expires.Before(a.authServer.GetClock().Now()) {
			return nil, trace.AccessDenied("access denied: client credentials have expired, please relogin.")
		}
		// If the user is generating a certificate, the roles and traits come from
		// the logged in identity.
		roles, traits, err = services.ExtractFromIdentity(a.authServer, &a.identity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		err := trace.AccessDenied("user %q has requested to generate certs for %q.", a.user.GetName(), req.Username)
		log.Warning(err)
		if err := a.authServer.EmitAuditEvent(events.UserLocalLoginFailure, events.EventFields{
			events.LoginMethod:        events.LoginMethodClientCert,
			events.AuthAttemptSuccess: false,
			// log the original internal error in audit log
			events.AuthAttemptErr: trace.Unwrap(err).Error(),
		}); err != nil {
			log.Warnf("Failed to emit local login failure event: %v", err)
		}
		// this error is vague on purpose, it should not happen unless someone is trying something out of loop
		return nil, trace.AccessDenied("this request can be only executed by an admin")
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
			if err := services.ValidateAccessRequest(a.authServer, accessReq); err != nil {
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

	// Extract the user and role set for whom the certificate will be generated.
	user, err := a.GetUser(req.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(roles, a.authServer, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate certificate, note that the roles TTL will be ignored because
	// the request is coming from "tctl auth sign" itself.
	certs, err := a.authServer.generateUserCert(certRequest{
		user:            user,
		ttl:             req.Expires.Sub(a.authServer.GetClock().Now()),
		compatibility:   req.Format,
		publicKey:       req.PublicKey,
		overrideRoleTTL: a.hasBuiltinRole(string(teleport.RoleAdmin)),
		routeToCluster:  req.RouteToCluster,
		checker:         checker,
		traits:          traits,
		activeRequests: services.RequestIDs{
			AccessRequests: req.AccessRequests,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.Certs{
		SSH: certs.ssh,
		TLS: certs.tls,
	}, nil
}

func (a *AuthWithRoles) GetSignupU2FRegisterRequest(token string) (u2fRegisterRequest *u2f.RegisterRequest, e error) {
	// signup token are their own authz resource
	return a.authServer.CreateSignupU2FRegisterRequest(token)
}

func (a *AuthWithRoles) CreateResetPasswordToken(ctx context.Context, req CreateResetPasswordTokenRequest) (services.ResetPasswordToken, error) {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.EmitAuditEvent(events.ResetPasswordTokenCreated, events.EventFields{
		events.FieldName:             req.Name,
		events.ResetPasswordTokenTTL: req.TTL.String(),
		events.EventUser:             a.user.GetName(),
	}); err != nil {
		log.Warnf("Failed to emit create reset password token event: %v", err)
	}

	return a.authServer.CreateResetPasswordToken(ctx, req)
}

func (a *AuthWithRoles) GetResetPasswordToken(ctx context.Context, tokenID string) (services.ResetPasswordToken, error) {
	// tokens are their own authz mechanism, no need to double check
	return a.authServer.GetResetPasswordToken(ctx, tokenID)
}

func (a *AuthWithRoles) RotateResetPasswordTokenSecrets(ctx context.Context, tokenID string) (services.ResetPasswordTokenSecrets, error) {
	// tokens are their own authz mechanism, no need to double check
	return a.authServer.RotateResetPasswordTokenSecrets(ctx, tokenID)
}

func (a *AuthWithRoles) ChangePasswordWithToken(ctx context.Context, req ChangePasswordWithTokenRequest) (services.WebSession, error) {
	// Token is it's own authentication, no need to double check.
	return a.authServer.ChangePasswordWithToken(ctx, req)
}

// CreateUser inserts a new user entry in a backend.
func (a *AuthWithRoles) CreateUser(ctx context.Context, user services.User) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}

	user.SetCreatedBy(services.CreatedBy{
		User: services.UserRef{Name: a.user.GetName()},
	})

	return a.authServer.CreateUser(ctx, user)
}

// UpdateUser updates an existing user in a backend.
// Captures the auth user who modified the user record.
func (a *AuthWithRoles) UpdateUser(ctx context.Context, user services.User) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.UpdateUser(ctx, user)
}

func (a *AuthWithRoles) UpsertUser(u services.User) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	createdBy := u.GetCreatedBy()
	if createdBy.IsEmpty() {
		u.SetCreatedBy(services.CreatedBy{
			User: services.UserRef{Name: a.user.GetName()},
		})
	}
	return a.authServer.UpsertUser(u)
}

// UpsertOIDCConnector creates or updates an OIDC connector.
func (a *AuthWithRoles) UpsertOIDCConnector(ctx context.Context, connector services.OIDCConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertOIDCConnector(ctx, connector)
}

func (a *AuthWithRoles) GetOIDCConnector(id string, withSecrets bool) (services.OIDCConnector, error) {
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

func (a *AuthWithRoles) GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error) {
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

func (a *AuthWithRoles) CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error) {
	if err := a.action(defaults.Namespace, services.KindOIDCRequest, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateOIDCAuthRequest(req)
}

func (a *AuthWithRoles) ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	return a.authServer.ValidateOIDCAuthCallback(q)
}

func (a *AuthWithRoles) DeleteOIDCConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteOIDCConnector(ctx, connectorID)
}

func (a *AuthWithRoles) CreateSAMLConnector(ctx context.Context, connector services.SAMLConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertSAMLConnector(ctx, connector)
}

// UpsertSAMLConnector creates or updates a SAML connector.
func (a *AuthWithRoles) UpsertSAMLConnector(ctx context.Context, connector services.SAMLConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertSAMLConnector(ctx, connector)
}

func (a *AuthWithRoles) GetSAMLConnector(id string, withSecrets bool) (services.SAMLConnector, error) {
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

func (a *AuthWithRoles) GetSAMLConnectors(withSecrets bool) ([]services.SAMLConnector, error) {
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

func (a *AuthWithRoles) CreateSAMLAuthRequest(req services.SAMLAuthRequest) (*services.SAMLAuthRequest, error) {
	if err := a.action(defaults.Namespace, services.KindSAMLRequest, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateSAMLAuthRequest(req)
}

func (a *AuthWithRoles) ValidateSAMLResponse(re string) (*SAMLAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	return a.authServer.ValidateSAMLResponse(re)
}

// DeleteSAMLConnector deletes a SAML connector by name.
func (a *AuthWithRoles) DeleteSAMLConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteSAMLConnector(ctx, connectorID)
}

func (a *AuthWithRoles) CreateGithubConnector(connector services.GithubConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateGithubConnector(connector)
}

// UpsertGithubConnector creates or updates a Github connector.
func (a *AuthWithRoles) UpsertGithubConnector(ctx context.Context, connector services.GithubConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.upsertGithubConnector(ctx, connector)
}

func (a *AuthWithRoles) GetGithubConnector(id string, withSecrets bool) (services.GithubConnector, error) {
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

func (a *AuthWithRoles) GetGithubConnectors(withSecrets bool) ([]services.GithubConnector, error) {
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
func (a *AuthWithRoles) DeleteGithubConnector(ctx context.Context, connectorID string) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.deleteGithubConnector(ctx, connectorID)
}

func (a *AuthWithRoles) CreateGithubAuthRequest(req services.GithubAuthRequest) (*services.GithubAuthRequest, error) {
	if err := a.action(defaults.Namespace, services.KindGithubRequest, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateGithubAuthRequest(req)
}

func (a *AuthWithRoles) ValidateGithubAuthCallback(q url.Values) (*GithubAuthResponse, error) {
	return a.authServer.ValidateGithubAuthCallback(q)
}

func (a *AuthWithRoles) EmitAuditEvent(event events.Event, fields events.EventFields) error {
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.EmitAuditEvent(event, fields)
}

func (a *AuthWithRoles) PostSessionSlice(slice events.SessionSlice) error {
	if err := a.action(slice.Namespace, services.KindEvent, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(slice.Namespace, services.KindEvent, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.PostSessionSlice(slice)
}

func (a *AuthWithRoles) UploadSessionRecording(r events.SessionRecording) error {
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

func (a *AuthWithRoles) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	if err := a.action(namespace, services.KindSession, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionChunk(namespace, sid, offsetBytes, maxBytes)
}

func (a *AuthWithRoles) GetSessionEvents(namespace string, sid session.ID, afterN int, includePrintEvents bool) ([]events.EventFields, error) {
	if err := a.action(namespace, services.KindSession, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionEvents(namespace, sid, afterN, includePrintEvents)
}

func (a *AuthWithRoles) SearchEvents(from, to time.Time, query string, limit int) ([]events.EventFields, error) {
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.SearchEvents(from, to, query, limit)
}

func (a *AuthWithRoles) SearchSessionEvents(from, to time.Time, limit int) ([]events.EventFields, error) {
	if err := a.action(defaults.Namespace, services.KindSession, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.SearchSessionEvents(from, to, limit)
}

// GetNamespaces returns a list of namespaces
func (a *AuthWithRoles) GetNamespaces() ([]services.Namespace, error) {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNamespaces()
}

// GetNamespace returns namespace by name
func (a *AuthWithRoles) GetNamespace(name string) (*services.Namespace, error) {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNamespace(name)
}

// UpsertNamespace upserts namespace
func (a *AuthWithRoles) UpsertNamespace(ns services.Namespace) error {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertNamespace(ns)
}

// DeleteNamespace deletes namespace by name
func (a *AuthWithRoles) DeleteNamespace(name string) error {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteNamespace(name)
}

// GetRoles returns a list of roles
func (a *AuthWithRoles) GetRoles() ([]services.Role, error) {
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetRoles()
}

// CreateRole creates a role.
func (a *AuthWithRoles) CreateRole(role services.Role) error {
	return trace.NotImplemented("not implemented")
}

// UpsertRole creates or updates role.
func (a *AuthWithRoles) UpsertRole(ctx context.Context, role services.Role) error {
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.upsertRole(ctx, role)
}

// GetRole returns role by name
func (a *AuthWithRoles) GetRole(name string) (services.Role, error) {
	// Current-user exception: we always allow users to read roles
	// that they hold.  This requirement is checked first to avoid
	// misleading denial messages in the logs.
	if !utils.SliceContainsStr(a.user.GetRoles(), name) {
		if err := a.action(defaults.Namespace, services.KindRole, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetRole(name)
}

// DeleteRole deletes role by name
func (a *AuthWithRoles) DeleteRole(ctx context.Context, name string) error {
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteRole(ctx, name)
}

// GetClusterConfig gets cluster level configuration.
func (a *AuthWithRoles) GetClusterConfig(opts ...services.MarshalOption) (services.ClusterConfig, error) {
	if err := a.action(defaults.Namespace, services.KindClusterConfig, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetClusterConfig(opts...)
}

// DeleteClusterConfig deletes cluster config
func (a *AuthWithRoles) DeleteClusterConfig() error {
	if err := a.action(defaults.Namespace, services.KindClusterConfig, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteClusterConfig()
}

// DeleteClusterName deletes cluster name
func (a *AuthWithRoles) DeleteClusterName() error {
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteClusterName()
}

// DeleteStaticTokens deletes static tokens
func (a *AuthWithRoles) DeleteStaticTokens() error {
	if err := a.action(defaults.Namespace, services.KindStaticTokens, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteStaticTokens()
}

// SetClusterConfig sets cluster level configuration.
func (a *AuthWithRoles) SetClusterConfig(c services.ClusterConfig) error {
	if err := a.action(defaults.Namespace, services.KindClusterConfig, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindClusterConfig, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetClusterConfig(c)
}

// GetClusterName gets the name of the cluster.
func (a *AuthWithRoles) GetClusterName(opts ...services.MarshalOption) (services.ClusterName, error) {
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetClusterName()
}

// SetClusterName sets the name of the cluster. SetClusterName can only be called once.
func (a *AuthWithRoles) SetClusterName(c services.ClusterName) error {
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetClusterName(c)
}

// UpsertClusterName sets the name of the cluster.
func (a *AuthWithRoles) UpsertClusterName(c services.ClusterName) error {
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindClusterName, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertClusterName(c)
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (a *AuthWithRoles) GetStaticTokens() (services.StaticTokens, error) {
	if err := a.action(defaults.Namespace, services.KindStaticTokens, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetStaticTokens()
}

// SetStaticTokens sets the list of static tokens used to provision nodes.
func (a *AuthWithRoles) SetStaticTokens(s services.StaticTokens) error {
	if err := a.action(defaults.Namespace, services.KindStaticTokens, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindStaticTokens, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.SetStaticTokens(s)
}

func (a *AuthWithRoles) GetAuthPreference() (services.AuthPreference, error) {
	if err := a.action(defaults.Namespace, services.KindClusterAuthPreference, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetAuthPreference()
}

func (a *AuthWithRoles) SetAuthPreference(cap services.AuthPreference) error {
	if err := a.action(defaults.Namespace, services.KindClusterAuthPreference, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindClusterAuthPreference, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.SetAuthPreference(cap)
}

// DeleteAllTokens deletes all tokens
func (a *AuthWithRoles) DeleteAllTokens() error {
	return trace.NotImplemented("not implemented")
}

// DeleteAllCertAuthorities deletes all certificate authorities of a certain type
func (a *AuthWithRoles) DeleteAllCertAuthorities(caType services.CertAuthType) error {
	return trace.NotImplemented("not implemented")
}

// DeleteAllCertNamespaces deletes all namespaces
func (a *AuthWithRoles) DeleteAllNamespaces() error {
	return trace.NotImplemented("not implemented")
}

// DeleteAllReverseTunnels deletes all reverse tunnels
func (a *AuthWithRoles) DeleteAllReverseTunnels() error {
	return trace.NotImplemented("not implemented")
}

// DeleteAllRoles deletes all roles
func (a *AuthWithRoles) DeleteAllRoles() error {
	return trace.NotImplemented("not implemented")
}

// DeleteAllUsers deletes all users
func (a *AuthWithRoles) DeleteAllUsers() error {
	return trace.NotImplemented("not implemented")
}

func (a *AuthWithRoles) GetTrustedClusters() ([]services.TrustedCluster, error) {
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetTrustedClusters()
}

func (a *AuthWithRoles) GetTrustedCluster(name string) (services.TrustedCluster, error) {
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.GetTrustedCluster(name)
}

// UpsertTrustedCluster creates or updates a trusted cluster.
func (a *AuthWithRoles) UpsertTrustedCluster(ctx context.Context, tc services.TrustedCluster) (services.TrustedCluster, error) {
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.UpsertTrustedCluster(ctx, tc)
}

func (a *AuthWithRoles) ValidateTrustedCluster(validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	// the token provides it's own authorization and authentication
	return a.authServer.validateTrustedCluster(validateRequest)
}

// DeleteTrustedCluster deletes a trusted cluster by name.
func (a *AuthWithRoles) DeleteTrustedCluster(ctx context.Context, name string) error {
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteTrustedCluster(ctx, name)
}

func (a *AuthWithRoles) UpsertTunnelConnection(conn services.TunnelConnection) error {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertTunnelConnection(conn)
}

func (a *AuthWithRoles) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]services.TunnelConnection, error) {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetTunnelConnections(clusterName, opts...)
}

func (a *AuthWithRoles) GetAllTunnelConnections(opts ...services.MarshalOption) ([]services.TunnelConnection, error) {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetAllTunnelConnections(opts...)
}

func (a *AuthWithRoles) DeleteTunnelConnection(clusterName string, connName string) error {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteTunnelConnection(clusterName, connName)
}

func (a *AuthWithRoles) DeleteTunnelConnections(clusterName string) error {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteTunnelConnections(clusterName)
}

func (a *AuthWithRoles) DeleteAllTunnelConnections() error {
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTunnelConnection, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllTunnelConnections()
}

func (a *AuthWithRoles) CreateRemoteCluster(conn services.RemoteCluster) error {
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateRemoteCluster(conn)
}

func (a *AuthWithRoles) GetRemoteCluster(clusterName string) (services.RemoteCluster, error) {
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetRemoteCluster(clusterName)
}

func (a *AuthWithRoles) GetRemoteClusters(opts ...services.MarshalOption) ([]services.RemoteCluster, error) {
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetRemoteClusters(opts...)
}

func (a *AuthWithRoles) DeleteRemoteCluster(clusterName string) error {
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteRemoteCluster(clusterName)
}

func (a *AuthWithRoles) DeleteAllRemoteClusters() error {
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbList); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindRemoteCluster, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteAllRemoteClusters()
}

// ProcessKubeCSR processes CSR request against Kubernetes CA, returns
// signed certificate if successful.
func (a *AuthWithRoles) ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error) {
	// limits the requests types to proxies to make it harder to break
	if !a.hasBuiltinRole(string(teleport.RoleProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}
	return a.authServer.ProcessKubeCSR(req)
}

func (a *AuthWithRoles) Close() error {
	return a.authServer.Close()
}

func (a *AuthWithRoles) WaitForDelivery(context.Context) error {
	return nil
}

// NewAdminAuthServer returns auth server authorized as admin,
// used for auth server cached access
func NewAdminAuthServer(authServer *AuthServer, sessions session.Service, alog events.IAuditLog) (ClientI, error) {
	ctx, err := NewAdminContext()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AuthWithRoles{
		authServer: authServer,
		checker:    ctx.Checker,
		user:       ctx.User,
		alog:       alog,
		sessions:   sessions,
	}, nil
}
