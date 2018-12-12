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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"github.com/tstranex/u2f"
)

type AuthWithRoles struct {
	authServer *AuthServer
	checker    services.AccessChecker
	user       services.User
	sessions   session.Service
	alog       events.IAuditLog
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
	if username == a.user.GetName() {
		return nil
	}
	return a.checker.CheckAccessToRule(&services.Context{User: a.user},
		defaults.Namespace, services.KindUser, services.VerbCreate, false)
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

func (a *AuthWithRoles) GenerateToken(req GenerateTokenRequest) (string, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.GenerateToken(req)
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

		default:
			return nil, trace.AccessDenied("not authorized to watch %v events", kind.Kind)
		}
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

	// Fetch services.RoleSet for the identity of the logged in user.
	roles, err := services.FetchRoles(a.user.GetRoles(), a.authServer, a.user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract all unique allowed logins across all roles.
	allowedLogins := make(map[string]bool)
	for _, role := range roles {
		for _, login := range role.GetLogins(services.Allow) {
			allowedLogins[login] = true
		}
	}

	// Loop over all nodes and check if the caller has access.
	filteredNodes := make([]services.Server, 0, len(nodes))
NextNode:
	for _, node := range nodes {
		for login, _ := range allowedLogins {
			err := roles.CheckAccessToServer(login, node)
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

func (a *AuthWithRoles) GetReverseTunnel(name string) (services.ReverseTunnel, error) {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnel(name)
}

func (a *AuthWithRoles) GetReverseTunnels() ([]services.ReverseTunnel, error) {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnels()
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

func (a *AuthWithRoles) GetOTPData(user string) (string, []byte, error) {
	if err := a.currentUserAction(user); err != nil {
		return "", nil, trace.Wrap(err)
	}
	return a.authServer.GetOTPData(user)
}

func (a *AuthWithRoles) PreAuthenticatedSignIn(user string) (services.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.PreAuthenticatedSignIn(user)
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
	return a.authServer.ExtendWebSession(user, prevSessionID)
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

func (a *AuthWithRoles) GetUsers() ([]services.User, error) {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetUsers()
}

func (a *AuthWithRoles) GetUser(name string) (services.User, error) {
	if err := a.currentUserAction(name); err != nil {
		if err := a.action(defaults.Namespace, services.KindUser, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetUser(name)
}

func (a *AuthWithRoles) DeleteUser(user string) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteUser(user)
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

func (a *AuthWithRoles) GenerateUserCert(key []byte, username string, ttl time.Duration, compatibility string) ([]byte, error) {
	// This endpoint is only accessible to tctl.
	if !a.hasBuiltinRole(string(teleport.RoleAdmin)) {
		return nil, trace.AccessDenied("this request can be only executed by an admin")
	}

	// Extract the user and role set for whom the certificate will be generated.
	user, err := a.GetUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(user.GetRoles(), a.authServer, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate certificate, note that the roles TTL will be ignored because
	// the request is coming from "tctl auth sign" itself.
	certs, err := a.authServer.generateUserCert(certRequest{
		user:            user,
		roles:           checker,
		ttl:             ttl,
		compatibility:   compatibility,
		publicKey:       key,
		overrideRoleTTL: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certs.ssh, nil
}

func (a *AuthWithRoles) CreateSignupToken(user services.UserV1, ttl time.Duration) (token string, e error) {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.CreateSignupToken(user, ttl)
}

func (a *AuthWithRoles) GetSignupTokenData(token string) (user string, otpQRCode []byte, err error) {
	// signup token are their own authz resource
	return a.authServer.GetSignupTokenData(token)
}

func (a *AuthWithRoles) GetSignupToken(token string) (*services.SignupToken, error) {
	// signup token are their own authz resource
	return a.authServer.GetSignupToken(token)
}

func (a *AuthWithRoles) GetSignupU2FRegisterRequest(token string) (u2fRegisterRequest *u2f.RegisterRequest, e error) {
	// signup token are their own authz resource
	return a.authServer.CreateSignupU2FRegisterRequest(token)
}

func (a *AuthWithRoles) CreateUserWithOTP(token, password, otpToken string) (services.WebSession, error) {
	// tokens are their own authz mechanism, no need to double check
	return a.authServer.CreateUserWithOTP(token, password, otpToken)
}

func (a *AuthWithRoles) CreateUserWithoutOTP(token string, password string) (services.WebSession, error) {
	// tokens are their own authz mechanism, no need to double check
	return a.authServer.CreateUserWithoutOTP(token, password)
}

func (a *AuthWithRoles) CreateUserWithU2FToken(token string, password string, u2fRegisterResponse u2f.RegisterResponse) (services.WebSession, error) {
	// signup tokens are their own authz resource
	return a.authServer.CreateUserWithU2FToken(token, password, u2fRegisterResponse)
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

func (a *AuthWithRoles) UpsertOIDCConnector(connector services.OIDCConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertOIDCConnector(connector)
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

func (a *AuthWithRoles) DeleteOIDCConnector(connectorID string) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindOIDC, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteOIDCConnector(connectorID)
}

func (a *AuthWithRoles) CreateSAMLConnector(connector services.SAMLConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertSAMLConnector(connector)
}

func (a *AuthWithRoles) UpsertSAMLConnector(connector services.SAMLConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertSAMLConnector(connector)
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

func (a *AuthWithRoles) DeleteSAMLConnector(connectorID string) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindSAML, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteSAMLConnector(connectorID)
}

func (a *AuthWithRoles) CreateGithubConnector(connector services.GithubConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CreateGithubConnector(connector)
}

func (a *AuthWithRoles) UpsertGithubConnector(connector services.GithubConnector) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertGithubConnector(connector)
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

func (a *AuthWithRoles) DeleteGithubConnector(id string) error {
	if err := a.authConnectorAction(defaults.Namespace, services.KindGithub, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteGithubConnector(id)
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

func (a *AuthWithRoles) EmitAuditEvent(eventType string, fields events.EventFields) error {
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.EmitAuditEvent(eventType, fields)
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

// UpsertRole creates or updates role
func (a *AuthWithRoles) UpsertRole(role services.Role) error {
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertRole(role)
}

// GetRole returns role by name
func (a *AuthWithRoles) GetRole(name string) (services.Role, error) {
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbRead); err != nil {
		// allow user to read roles assigned to them
		log.Infof("%v %v %v", a.user, a.user.GetRoles(), name)
		if !utils.SliceContainsStr(a.user.GetRoles(), name) {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetRole(name)
}

// DeleteRole deletes role by name
func (a *AuthWithRoles) DeleteRole(name string) error {
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteRole(name)
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

func (a *AuthWithRoles) UpsertTrustedCluster(tc services.TrustedCluster) (services.TrustedCluster, error) {
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.authServer.UpsertTrustedCluster(tc)
}

func (a *AuthWithRoles) ValidateTrustedCluster(validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	// the token provides it's own authorization and authentication
	return a.authServer.validateTrustedCluster(validateRequest)
}

func (a *AuthWithRoles) DeleteTrustedCluster(name string) error {
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbDelete); err != nil {
		return trace.Wrap(err)
	}

	return a.authServer.DeleteTrustedCluster(name)
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
// signed certificate if sucessful.
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

// NewAuthWithRoles creates new auth server with access control
func NewAuthWithRoles(authServer *AuthServer,
	checker services.AccessChecker,
	user services.User,
	sessions session.Service,
	alog events.IAuditLog) *AuthWithRoles {
	return &AuthWithRoles{
		authServer: authServer,
		checker:    checker,
		sessions:   sessions,
		user:       user,
		alog:       alog,
	}
}
