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
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
)

type AuthWithRoles struct {
	authServer  *AuthServer
	permChecker PermissionChecker
	sessions    session.Service
	role        teleport.Role
	alog        events.IAuditLog
}

func (a *AuthWithRoles) GetSessions() ([]session.Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetSessions); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.sessions.GetSessions()
}

func (a *AuthWithRoles) GetSession(id session.ID) (*session.Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetSession); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.sessions.GetSession(id)

}
func (a *AuthWithRoles) CreateSession(s session.Session) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertSession); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.CreateSession(s)
}
func (a *AuthWithRoles) UpdateSession(req session.UpdateRequest) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertSession); err != nil {
		return trace.Wrap(err)
	}
	return a.sessions.UpdateSession(req)

}
func (a *AuthWithRoles) UpsertCertAuthority(ca services.CertAuthority, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertCertAuthority); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertCertAuthority(ca, ttl)

}
func (a *AuthWithRoles) GetCertAuthorities(caType services.CertAuthType, loadKeys bool) ([]*services.CertAuthority, error) {
	if loadKeys {
		if err := a.permChecker.HasPermission(a.role, ActionGetCertAuthoritiesWithSigningKeys); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := a.permChecker.HasPermission(a.role, ActionGetCertAuthorities); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.GetCertAuthorities(caType, loadKeys)
}

func (a *AuthWithRoles) GetLocalDomain() (string, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetLocalDomain); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.GetLocalDomain()
}

func (a *AuthWithRoles) DeleteCertAuthority(id services.CertAuthID) error {
	if err := a.permChecker.HasPermission(a.role, ActionDeleteCertAuthority); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteCertAuthority(id)
}
func (a *AuthWithRoles) GenerateToken(roles teleport.Roles, ttl time.Duration) (string, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGenerateToken); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.GenerateToken(roles, ttl)
}
func (a *AuthWithRoles) RegisterUsingToken(token, hostID string, role teleport.Role) (*PackedKeys, error) {
	if err := a.permChecker.HasPermission(a.role, ActionRegisterUsingToken); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.RegisterUsingToken(token, hostID, role)

}
func (a *AuthWithRoles) RegisterNewAuthServer(token string) error {
	if err := a.permChecker.HasPermission(a.role, ActionRegisterNewAuthServer); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.RegisterNewAuthServer(token)

}
func (a *AuthWithRoles) UpsertNode(s services.Server, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertServer); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertNode(s, ttl)

}
func (a *AuthWithRoles) GetNodes() ([]services.Server, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetServers); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetNodes()

}
func (a *AuthWithRoles) UpsertAuthServer(s services.Server, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertAuthServer); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertAuthServer(s, ttl)

}
func (a *AuthWithRoles) GetAuthServers() ([]services.Server, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetAuthServers); err != nil {
		return nil, err
	}
	return a.authServer.GetAuthServers()

}
func (a *AuthWithRoles) UpsertProxy(s services.Server, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertProxy); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertProxy(s, ttl)

}
func (a *AuthWithRoles) GetProxies() ([]services.Server, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetProxies); err != nil {
		return nil, err
	}
	return a.authServer.GetProxies()

}
func (a *AuthWithRoles) UpsertReverseTunnel(r services.ReverseTunnel, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertReverseTunnel); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertReverseTunnel(r, ttl)

}
func (a *AuthWithRoles) GetReverseTunnels() ([]services.ReverseTunnel, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetReverseTunnels); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetReverseTunnels()

}
func (a *AuthWithRoles) DeleteReverseTunnel(domainName string) error {
	if err := a.permChecker.HasPermission(a.role, ActionDeleteReverseTunnel); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteReverseTunnel(domainName)
}

func (a *AuthWithRoles) DeleteToken(token string) error {
	if err := a.permChecker.HasPermission(a.role, ActionGenerateToken); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteToken(token)
}

func (a *AuthWithRoles) GetTokens() ([]services.ProvisionToken, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGenerateToken); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetTokens()
}

func (a *AuthWithRoles) UpsertPassword(user string, password []byte) (hotpURL string, hotpQR []byte, err error) {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertPassword); err != nil {
		return "", nil, err
	}
	return a.authServer.UpsertPassword(user, password)
}
func (a *AuthWithRoles) CheckPassword(user string, password []byte, hotpToken string) error {
	if err := a.permChecker.HasPermission(a.role, ActionCheckPassword); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.CheckPassword(user, password, hotpToken)
}
func (a *AuthWithRoles) SignIn(user string, password []byte) (*Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionSignIn); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.SignIn(user, password)
}
func (a *AuthWithRoles) CreateWebSession(user string) (*Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionCreateWebSession); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateWebSession(user)
}
func (a *AuthWithRoles) ExtendWebSession(user, prevSessionID string) (*Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionExtendWebSession); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.ExtendWebSession(user, prevSessionID)
}
func (a *AuthWithRoles) GetWebSessionInfo(user string, sid string) (*Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetWebSession); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetWebSessionInfo(user, sid)

}
func (a *AuthWithRoles) DeleteWebSession(user string, sid string) error {
	if err := a.permChecker.HasPermission(a.role, ActionDeleteWebSession); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteWebSession(user, sid)
}
func (a *AuthWithRoles) GetUsers() ([]services.User, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetUsers); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GetUsers()
}
func (a *AuthWithRoles) GetUser(name string) (services.User, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetUser); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.Identity.GetUser(name)

}
func (a *AuthWithRoles) DeleteUser(user string) error {
	if err := a.permChecker.HasPermission(a.role, ActionDeleteUser); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.DeleteUser(user)

}
func (a *AuthWithRoles) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGenerateKeyPair); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return a.authServer.GenerateKeyPair(pass)

}
func (a *AuthWithRoles) GenerateHostCert(
	key []byte, hostname, authDomain string, roles teleport.Roles,
	ttl time.Duration) ([]byte, error) {

	if err := a.permChecker.HasPermission(a.role, ActionGenerateHostCert); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateHostCert(key, hostname, authDomain, roles, ttl)

}
func (a *AuthWithRoles) GenerateUserCert(key []byte, user string, ttl time.Duration) ([]byte, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGenerateUserCert); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.GenerateUserCert(key, user, ttl)

}
func (a *AuthWithRoles) CreateSignupToken(user services.User) (token string, e error) {
	if err := a.permChecker.HasPermission(a.role, ActionCreateSignupToken); err != nil {
		return "", trace.Wrap(err)
	}
	return a.authServer.CreateSignupToken(user)

}

func (a *AuthWithRoles) GetSignupTokenData(token string) (user string,
	QRImg []byte, hotpFirstValues []string, e error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetSignupTokenData); err != nil {
		return "", nil, nil, trace.Wrap(err)
	}
	return a.authServer.GetSignupTokenData(token)

}

func (a *AuthWithRoles) CreateUserWithToken(token, password, hotpToken string) (*Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionCreateUserWithToken); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateUserWithToken(token, password, hotpToken)

}

func (a *AuthWithRoles) UpsertUser(u services.User) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertUser); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.UpsertUser(u)

}

func (a *AuthWithRoles) UpsertOIDCConnector(connector services.OIDCConnector, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertOIDCConnector); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.Identity.UpsertOIDCConnector(connector, ttl)
}

func (a *AuthWithRoles) GetOIDCConnector(id string, withSecrets bool) (*services.OIDCConnector, error) {
	if withSecrets {
		if err := a.permChecker.HasPermission(a.role, ActionGetOIDCConnectorWithSecrets); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := a.permChecker.HasPermission(a.role, ActionGetOIDCConnectorWithoutSecrets); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetOIDCConnector(id, withSecrets)
}

func (a *AuthWithRoles) GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error) {
	if withSecrets {
		if err := a.permChecker.HasPermission(a.role, ActionGetOIDCConnectorsWithSecrets); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := a.permChecker.HasPermission(a.role, ActionGetOIDCConnectorsWithoutSecrets); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a.authServer.Identity.GetOIDCConnectors(withSecrets)
}

func (a *AuthWithRoles) CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error) {
	if err := a.permChecker.HasPermission(a.role, ActionCreateOIDCAuthRequest); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.CreateOIDCAuthRequest(req)
}

func (a *AuthWithRoles) ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	if err := a.permChecker.HasPermission(a.role, ActionValidateOIDCAuthCallback); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.authServer.ValidateOIDCAuthCallback(q)
}

func (a *AuthWithRoles) DeleteOIDCConnector(connectorID string) error {
	if err := a.permChecker.HasPermission(a.role, ActionDeleteOIDCConnector); err != nil {
		return trace.Wrap(err)
	}
	return a.authServer.Identity.DeleteOIDCConnector(connectorID)
}

func (a *AuthWithRoles) EmitAuditEvent(eventType string, fields events.EventFields) error {
	if err := a.permChecker.HasPermission(a.role, ActionEmitEvents); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.EmitAuditEvent(eventType, fields)
}

func (a *AuthWithRoles) PostSessionChunk(sid session.ID, reader io.Reader) error {
	if err := a.permChecker.HasPermission(a.role, ActionEmitEvents); err != nil {
		return trace.Wrap(err)
	}
	return a.alog.PostSessionChunk(sid, reader)
}

func (a *AuthWithRoles) GetSessionChunk(sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	if err := a.permChecker.HasPermission(a.role, ActionViewSession); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.alog.GetSessionChunk(sid, offsetBytes, maxBytes)
}

func (a *AuthWithRoles) GetSessionEvents(sid session.ID, afterN int) ([]events.EventFields, error) {
	if err := a.permChecker.HasPermission(a.role, ActionViewSession); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.alog.GetSessionEvents(sid, afterN)
}

func (a *AuthWithRoles) SearchEvents(from, to time.Time, query string) ([]events.EventFields, error) {
	if err := a.permChecker.HasPermission(a.role, ActionViewSession); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.alog.SearchEvents(from, to, query)
}

// test helper
func NewAuthWithRoles(authServer *AuthServer,
	permChecker PermissionChecker,
	sessions session.Service,
	role teleport.Role,
	alog events.IAuditLog) *AuthWithRoles {
	return &AuthWithRoles{
		authServer:  authServer,
		permChecker: permChecker,
		sessions:    sessions,
		role:        role,
		alog:        alog,
	}
}
