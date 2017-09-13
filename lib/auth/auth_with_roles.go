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
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/tstranex/u2f"
)

type AuthWithRoles struct {
	authServer *AuthServer
	checker    services.AccessChecker
	user       services.User
	sessions   session.Service
	alog       events.IAuditLog
}

func (a *AuthWithRoles) action(namespace string, resource string, action string) error {
	return a.checker.CheckAccessToRule(&services.Context{User: a.user}, namespace, resource, action)
}

// currentUserAction is a special checker that allows certain actions for users
// even if they are not admins, e.g. update their own passwords,
// or generate certificates, otherwise it will require admin privileges
func (a *AuthWithRoles) currentUserAction(username string) error {
	if username == a.user.GetName() {
		return nil
	}
	return a.checker.CheckAccessToRule(&services.Context{User: a.user},
		defaults.Namespace, services.KindUser, services.VerbCreate)
}

// authConnectorAction is a special checker that grants access to auth
// connectors. It first checks if you have access to the specific connector.
// If not, it checks if the requester has the meta KindAuthConnector access
// (which grants access to all connectors).
func (a *AuthWithRoles) authConnectorAction(namespace string, resource string, verb string) error {
	if err := a.checker.CheckAccessToRule(&services.Context{User: a.user}, namespace, resource, verb); err != nil {
		if err := a.checker.CheckAccessToRule(&services.Context{User: a.user}, namespace, services.KindAuthConnector, verb); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
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
	return trace.BadParameter("not implemented")
}

func (a *AuthWithRoles) UpsertCertAuthority(ca services.CertAuthority) error {
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertCertAuthority(ca)
}

func (a *AuthWithRoles) GetCertAuthorities(caType services.CertAuthType, loadKeys bool) ([]services.CertAuthority, error) {
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

	return a.authServer.GetCertAuthorities(caType, loadKeys)
}

func (a *AuthWithRoles) GetCertAuthority(id services.CertAuthID, loadKeys bool) (services.CertAuthority, error) {
	if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}
	if loadKeys {
		if err := a.action(defaults.Namespace, services.KindCertAuthority, services.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetCertAuthority(id, loadKeys)
}

func (a *AuthWithRoles) GetDomainName() (string, error) {
	// anyone can read it, no harm in that
	return a.authServer.GetDomainName()
}

func (a *AuthWithRoles) GetLocalClusterName() (string, error) {
	// anyone can read it, no harm in that
	return a.authServer.GetLocalClusterName()
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
	return trace.BadParameter("not implemented")
}

func (a *AuthWithRoles) DeactivateCertAuthority(id services.CertAuthID) error {
	return trace.BadParameter("not implemented")
}

func (a *AuthWithRoles) GenerateToken(roles teleport.Roles, ttl time.Duration) (string, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.GenerateToken(roles, ttl)
}

func (a *AuthWithRoles) RegisterUsingToken(token, hostID string, nodeName string, role teleport.Role) (*PackedKeys, error) {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterUsingToken(token, hostID, nodeName, role)
}

func (a *AuthWithRoles) RegisterNewAuthServer(token string) error {
	// tokens have authz mechanism  on their own, no need to check
	return a.authServer.RegisterNewAuthServer(token)
}

func (a *AuthWithRoles) UpsertNode(s services.Server) error {
	if err := a.action(s.GetNamespace(), services.KindNode, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(s.GetNamespace(), services.KindNode, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertNode(s)
}

func (a *AuthWithRoles) GetNodes(namespace string) ([]services.Server, error) {
	if err := a.action(namespace, services.KindNode, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNodes(namespace)
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

func (a *AuthWithRoles) UpsertReverseTunnel(r services.ReverseTunnel) error {
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindReverseTunnel, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertReverseTunnel(r)
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

func (a *AuthWithRoles) GetTokens() ([]services.ProvisionToken, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetTokens()
}

func (a *AuthWithRoles) GetToken(token string) (*services.ProvisionToken, error) {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetToken(token)
}

func (a *AuthWithRoles) UpsertToken(token string, roles teleport.Roles, ttl time.Duration) error {
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindToken, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertToken(token, roles, ttl)
}

func (a *AuthWithRoles) UpsertPassword(user string, password []byte) error {
	if err := a.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertPassword(user, password)
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

func (a *AuthWithRoles) SignIn(user string, password []byte) (services.WebSession, error) {
	if err := a.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.SignIn(user, password)
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
		return nil, trace.Wrap(err)
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
	key []byte, hostID, nodeName, clusterName string, roles teleport.Roles,
	ttl time.Duration) ([]byte, error) {

	if err := a.action(defaults.Namespace, services.KindHostCert, services.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateHostCert(key, hostID, nodeName, clusterName, roles, ttl)
}

func (a *AuthWithRoles) GenerateUserCert(key []byte, username string, ttl time.Duration, compatibility string) ([]byte, error) {
	if err := a.currentUserAction(username); err != nil {
		return nil, trace.AccessDenied("%v cannot request a certificate for %v", a.user.GetName(), username)
	}
	// notice that user requesting the certificate and the user currently
	// authenticated may differ (e.g. admin generates certificate for the user scenario)
	// so we fetch user's permissions
	checker := a.checker
	var user services.User
	var err error
	if a.user.GetName() != username {
		user, err = a.GetUser(username)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		checker, err = services.FetchRoles(user.GetRoles(), a.authServer, user.GetTraits())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		user = a.user
	}

	// adjust session ttl to the smaller of two values: the session
	// ttl requested in tsh or the session ttl for the role.
	sessionTTL := checker.AdjustSessionTTL(ttl)

	// check signing TTL and return a list of allowed logins
	allowedLogins, err := checker.CheckLoginDuration(sessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateUserCert(
		key, user, allowedLogins, sessionTTL, checker.CanForwardAgents(), compatibility)
}

func (a *AuthWithRoles) CreateSignupToken(user services.UserV1) (token string, e error) {
	if err := a.action(defaults.Namespace, services.KindUser, services.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.CreateSignupToken(user)
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

func (a *AuthWithRoles) PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error {
	if err := a.action(namespace, services.KindEvent, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(namespace, services.KindEvent, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.PostSessionChunk(namespace, sid, reader)
}

func (a *AuthWithRoles) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	if err := a.action(namespace, services.KindSession, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionChunk(namespace, sid, offsetBytes, maxBytes)
}

func (a *AuthWithRoles) GetSessionEvents(namespace string, sid session.ID, afterN int) ([]events.EventFields, error) {
	if err := a.action(namespace, services.KindSession, services.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.GetSessionEvents(namespace, sid, afterN)
}

func (a *AuthWithRoles) SearchEvents(from, to time.Time, query string) ([]events.EventFields, error) {
	if err := a.action(defaults.Namespace, services.KindEvent, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.SearchEvents(from, to, query)
}

func (a *AuthWithRoles) SearchSessionEvents(from, to time.Time) ([]events.EventFields, error) {
	if err := a.action(defaults.Namespace, services.KindSession, services.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return a.alog.SearchSessionEvents(from, to)
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
func (a *AuthWithRoles) CreateRole(role services.Role, ttl time.Duration) error {
	return trace.BadParameter("not implemented")
}

// UpsertRole creates or updates role
func (a *AuthWithRoles) UpsertRole(role services.Role, ttl time.Duration) error {
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindRole, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertRole(role, ttl)
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

// GetClusterName gets the name of the cluster.
func (a *AuthWithRoles) GetClusterName() (services.ClusterName, error) {
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

// DeleteAllCertAuthorities deletes all certificate authorities of a certain type
func (a *AuthWithRoles) DeleteAllCertAuthorities(caType services.CertAuthType) error {
	return trace.BadParameter("not implemented")
}

// DeleteAllCertNamespaces deletes all namespaces
func (a *AuthWithRoles) DeleteAllNamespaces() error {
	return trace.BadParameter("not implemented")
}

// DeleteAllReverseTunnels deletes all reverse tunnels
func (a *AuthWithRoles) DeleteAllReverseTunnels() error {
	return trace.BadParameter("not implemented")
}

// DeleteAllProxies deletes all proxies
func (a *AuthWithRoles) DeleteAllProxies() error {
	return trace.BadParameter("not implemented")
}

// DeleteAllNodes deletes all nodes in a given namespace
func (a *AuthWithRoles) DeleteAllNodes(namespace string) error {
	return trace.BadParameter("not implemented")
}

// DeleteAllRoles deletes all roles
func (a *AuthWithRoles) DeleteAllRoles() error {
	return trace.BadParameter("not implemented")
}

// DeleteAllUsers deletes all users
func (a *AuthWithRoles) DeleteAllUsers() error {
	return trace.BadParameter("not implemented")
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

func (a *AuthWithRoles) UpsertTrustedCluster(tc services.TrustedCluster) error {
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := a.action(defaults.Namespace, services.KindTrustedCluster, services.VerbUpdate); err != nil {
		return trace.Wrap(err)
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

func (a *AuthWithRoles) Close() error {
	return a.authServer.Close()
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
