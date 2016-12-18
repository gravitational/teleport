/*
Copyright 2015 Gravitational, Inc.

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
	"io"
	"net/url"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"

	"github.com/tstranex/u2f"
)

type AuthWithRoles struct {
	authServer *AuthServer
	checker    services.AccessChecker
	user       string
	sessions   session.Service
	alog       events.IAuditLog
}

func (a *AuthWithRoles) action(namespace string, resourceKind, action string) error {
	return a.checker.CheckResourceAction(namespace, resourceKind, action)
}

// currentUserAction is a special checker that allows certain actions for users
// even if they are not admins, e.g. update their own passwords,
// or generate certificates, otherwise it will require admin privileges
func (a *AuthWithRoles) currentUserAction(username string) error {
	if username == a.user {
		return nil
	}
	return a.checker.CheckResourceAction(
		defaults.Namespace, services.KindUser, services.ActionWrite)
}

func (a *AuthWithRoles) GetSessions(namespace string) ([]session.Session, error) {
	if err := a.action(namespace, services.KindSession, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.sessions.GetSessions(namespace)
}

func (a *AuthWithRoles) GetSession(namespace string, id session.ID) (*session.Session, error) {
	if err := a.action(namespace, services.KindSession, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.sessions.GetSession(namespace, id)
}

func (a *AuthWithRoles) CreateSession(s session.Session) error {
	if err := a.action(s.Namespace, services.KindSession, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.CreateSession(s)
}

func (a *AuthWithRoles) UpdateSession(req session.UpdateRequest) error {
	if err := a.action(req.Namespace, services.KindSession, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.UpdateSession(req)
}

func (a *AuthWithRoles) UpsertCertAuthority(ca services.CertAuthority, ttl time.Duration) error {
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertCertAuthority(ca, ttl)
}

func (a *AuthWithRoles) GetCertAuthorities(caType services.CertAuthType, loadKeys bool) ([]*services.CertAuthority, error) {
	if loadKeys {
		// loading private key implies admin access, what in our case == Write access to them
		if err := a.action(defaults.Namespace, services.KindCertAuthority, services.ActionWrite); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := a.action(defaults.Namespace, services.KindCertAuthority, services.ActionRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetCertAuthorities(caType, loadKeys)
}

func (a *AuthWithRoles) GetCertAuthority(id services.CertAuthID, loadKeys bool) (*services.CertAuthority, error) {
	if loadKeys {
		// loading private key implies admin access, what in our case == Write access to them
		if err := a.action(defaults.Namespace, services.KindCertAuthority, services.ActionWrite); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := a.action(defaults.Namespace, services.KindCertAuthority, services.ActionRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetCertAuthority(id, loadKeys)
}

func (a *AuthWithRoles) GetDomainName() (string, error) {
	// anyone can read it, no harm in that
	return a.authServer.GetDomainName()
}

func (a *AuthWithRoles) GetU2FAppID() (string, error) {
	// authenticated users can all read this
	return a.authServer.GetU2FAppID()
}

func (a *AuthWithRoles) DeleteCertAuthority(id services.CertAuthID) error {
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteCertAuthority(id)
}

func (a *AuthWithRoles) GenerateToken(roles teleport.Roles, ttl time.Duration) (string, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.ActionWrite); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.GenerateToken(roles, ttl)
}

func (a *AuthWithRoles) RegisterUsingToken(token, hostID string, role teleport.Role) (*PackedKeys, error) {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterUsingToken(token, hostID, role)
}

func (a *AuthWithRoles) RegisterNewAuthServer(token string) error {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterNewAuthServer(token)
}

func (a *AuthWithRoles) UpsertNode(s services.Server, ttl time.Duration) error {
	if err := a.action(s.Namespace, services.KindNode, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertNode(s, ttl)
}

func (a *AuthWithRoles) GetNodes(namespace string) ([]services.Server, error) {
	if err := a.action(namespace, services.KindNode, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNodes(namespace)
}

func (a *AuthWithRoles) UpsertAuthServer(s services.Server, ttl time.Duration) error {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertAuthServer(s, ttl)
}

func (a *AuthWithRoles) GetAuthServers() ([]services.Server, error) {
	if err := a.action(defaults.Namespace, services.KindAuthServer, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetAuthServers()
}

func (a *AuthWithRoles) UpsertProxy(s services.Server, ttl time.Duration) error {
	if err := a.action(defaults.Namespace, services.KindProxy, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertProxy(s, ttl)
}

func (a *AuthWithRoles) GetProxies() ([]services.Server, error) {
	if err := a.action(defaults.Namespace, services.KindProxy, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetProxies()
}

func (a *AuthWithRoles) UpsertReverseTunnel(r services.ReverseTunnel, ttl time.Duration) error {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertReverseTunnel(r, ttl)
}

func (a *AuthWithRoles) GetReverseTunnels() ([]services.ReverseTunnel, error) {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnels()
}

func (a *AuthWithRoles) DeleteReverseTunnel(domainName string) error {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteReverseTunnel(domainName)
}

func (a *AuthWithRoles) DeleteToken(token string) error {
	if err := a.action(defaults.Namespace, services.KindToken, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteToken(token)
}

func (a *AuthWithRoles) GetTokens() ([]services.ProvisionToken, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetTokens()
}

func (a *AuthWithRoles) GetToken(token string) (*services.ProvisionToken, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetToken(token)
}

func (a *AuthWithRoles) UpsertToken(token string, roles teleport.Roles, ttl time.Duration) error {
	if err := a.action(defaults.Namespace, services.KindToken, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertToken(token, roles, ttl)
}

func (a *AuthWithRoles) UpsertPassword(user string, password []byte) (hotpURL string, hotpQR []byte, err error) {
	if err := a.currentUserAction(user); err != nil {
		return "", nil, trace.Wrap(err)
	}
	return a.authServer.UpsertPassword(user, password)
}

func (a *AuthWithRoles) CheckPassword(user string, password []byte, hotpToken string) error {
	if err := a.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CheckPassword(user, password, hotpToken)
}

func (a *AuthWithRoles) SignIn(user string, password []byte) (*Session, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.SignIn(user, password)
}

func (a *AuthWithRoles) PreAuthenticatedSignIn(user string) (*Session, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.PreAuthenticatedSignIn(user)
}

func (a *AuthWithRoles) GetU2FSignRequest(user string, password []byte) (*u2f.SignRequest, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.U2FSignRequest(user, password)
}

func (a *AuthWithRoles) CreateWebSession(user string) (*Session, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateWebSession(user)
}

func (a *AuthWithRoles) ExtendWebSession(user, prevSessionID string) (*Session, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.ExtendWebSession(user, prevSessionID)
}

func (a *AuthWithRoles) GetWebSessionInfo(user string, sid string) (*Session, error) {
	if err := a.action(defaults.Namespace, services.KindWebSession, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetWebSessionInfo(user, sid)
}

func (a *AuthWithRoles) DeleteWebSession(user string, sid string) error {
	if err := a.action(defaults.Namespace, services.KindWebSession, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteWebSession(user, sid)
}

func (a *AuthWithRoles) GetUsers() ([]services.User, error) {
	if err := a.action(defaults.Namespace, services.KindUser, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetUsers()
}

func (a *AuthWithRoles) GetUser(name string) (services.User, error) {
	if err := a.action(defaults.Namespace, services.KindUser, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.Identity.GetUser(name)
}

func (a *AuthWithRoles) DeleteUser(user string) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteUser(user)
}

func (a *AuthWithRoles) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	if err := a.action(defaults.Namespace, services.KindKeyPair, services.ActionWrite); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return a.authServer.GenerateKeyPair(pass)
}

func (a *AuthWithRoles) GenerateHostCert(
	key []byte, hostname, authDomain string, roles teleport.Roles,
	ttl time.Duration) ([]byte, error) {

	if err := a.action(defaults.Namespace, services.KindHostCert, services.ActionWrite); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateHostCert(key, hostname, authDomain, roles, ttl)
}

func (a *AuthWithRoles) GenerateUserCert(key []byte, user string, ttl time.Duration) ([]byte, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.AccessDenied("%v cannot request a certificate for %v", a.user, user)
	}
	// check signing TTL and return a list of allowed logins
	allowedLogins, err := a.checker.CheckLogins(ttl)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateUserCert(key, user, allowedLogins, ttl)
}

func (a *AuthWithRoles) CreateSignupToken(user services.User) (token string, e error) {
	if err := a.action(defaults.Namespace, services.KindUser, services.ActionWrite); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.CreateSignupToken(user)
}

func (a *AuthWithRoles) GetSignupTokenData(token string) (user string, QRImg []byte, hotpFirstValues []string, e error) {
	// signup token are their own authz resource
	return a.authServer.GetSignupTokenData(token)
}

func (a *AuthWithRoles) GetSignupU2FRegisterRequest(token string) (u2fRegisterRequest *u2f.RegisterRequest, e error) {
	// signup token are their own authz resource
	return a.authServer.CreateSignupU2FRegisterRequest(token)
}

func (a *AuthWithRoles) CreateUserWithToken(token, password, hotpToken string) (*Session, error) {
	// tokens are their own authz mechanism, no need to double check
	return a.authServer.CreateUserWithToken(token, password, hotpToken)
}

func (a *AuthWithRoles) CreateUserWithU2FToken(token string, password string, u2fRegisterResponse u2f.RegisterResponse) (*Session, error) {
	// signup tokens are their own authz resource
	return a.authServer.CreateUserWithU2FToken(token, password, u2fRegisterResponse)
}

func (a *AuthWithRoles) UpsertUser(u services.User) error {
	if err := a.action(defaults.Namespace, services.KindUser, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertUser(u)
}

func (a *AuthWithRoles) UpsertOIDCConnector(connector services.OIDCConnector, ttl time.Duration) error {
	if err := a.action(defaults.Namespace, services.KindOIDC, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.Identity.UpsertOIDCConnector(connector, ttl)
}

func (a *AuthWithRoles) GetOIDCConnector(id string, withSecrets bool) (*services.OIDCConnector, error) {
	if withSecrets {
		if err := a.action(defaults.Namespace, services.KindOIDC, services.ActionWrite); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := a.action(defaults.Namespace, services.KindOIDC, services.ActionRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetOIDCConnector(id, withSecrets)
}

func (a *AuthWithRoles) GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error) {
	if withSecrets {
		if err := a.action(defaults.Namespace, services.KindOIDC, services.ActionWrite); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := a.action(defaults.Namespace, services.KindOIDC, services.ActionRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetOIDCConnectors(withSecrets)
}

func (a *AuthWithRoles) CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error) {
	if err := a.action(defaults.Namespace, services.KindOIDCRequest, services.ActionWrite); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateOIDCAuthRequest(req)
}

func (a *AuthWithRoles) ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	// auth callback is it's own authz, no need to check extra permissions
	return a.authServer.ValidateOIDCAuthCallback(q)
}

func (a *AuthWithRoles) DeleteOIDCConnector(connectorID string) error {
	if err := a.action(defaults.Namespace, services.KindOIDC, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.Identity.DeleteOIDCConnector(connectorID)
}

func (a *AuthWithRoles) EmitAuditEvent(eventType string, fields events.EventFields) error {
	if err := a.action(defaults.Namespace, services.KindEvent, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.EmitAuditEvent(eventType, fields)
}

func (a *AuthWithRoles) PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error {
	if err := a.action(namespace, services.KindSession, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.PostSessionChunk(namespace, sid, reader)
}

func (a *AuthWithRoles) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	if err := a.action(namespace, services.KindSession, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.alog.GetSessionChunk(namespace, sid, offsetBytes, maxBytes)
}

func (a *AuthWithRoles) GetSessionEvents(namespace string, sid session.ID, afterN int) ([]events.EventFields, error) {
	if err := a.action(namespace, services.KindSession, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.alog.GetSessionEvents(namespace, sid, afterN)
}

func (a *AuthWithRoles) SearchEvents(from, to time.Time, query string) ([]events.EventFields, error) {
	if err := a.action(defaults.Namespace, services.KindEvent, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.alog.SearchEvents(from, to, query)
}

// GetNamespaces returns a list of namespaces
func (a *AuthWithRoles) GetNamespaces() ([]services.Namespace, error) {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNamespaces()
}

// GetNamespace returns namespace by name
func (a *AuthWithRoles) GetNamespace(name string) (*services.Namespace, error) {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNamespace(name)
}

// UpsertNamespace upserts namespace
func (a *AuthWithRoles) UpsertNamespace(ns services.Namespace) error {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertNamespace(ns)
}

// DeleteNamespace deletes namespace by name
func (a *AuthWithRoles) DeleteNamespace(name string) error {
	if err := a.action(defaults.Namespace, services.KindNamespace, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteNamespace(name)
}

// GetRoles returns a list of roles
func (a *AuthWithRoles) GetRoles() ([]services.Role, error) {
	if err := a.action(defaults.Namespace, services.KindRole, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetRoles()
}

// UpsertRole creates or updates role
func (a *AuthWithRoles) UpsertRole(role services.Role) error {
	if err := a.action(defaults.Namespace, services.KindRole, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertRole(role)
}

// GetRole returns role by name
func (a *AuthWithRoles) GetRole(name string) (services.Role, error) {
	if err := a.action(defaults.Namespace, services.KindRole, services.ActionRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetRole(name)
}

// DeleteRole deletes role by name
func (a *AuthWithRoles) DeleteRole(name string) error {
	if err := a.action(defaults.Namespace, services.KindRole, services.ActionWrite); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteRole(name)
}

// NewAuthWithRoles creates new auth server with access control
func NewAuthWithRoles(authServer *AuthServer,
	checker services.AccessChecker,
	user string,
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
