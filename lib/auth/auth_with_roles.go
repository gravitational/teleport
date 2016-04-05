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
	"net/url"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"

	"github.com/codahale/lunk"
	"github.com/gravitational/trace"
)

type AuthWithRoles struct {
	authServer  *AuthServer
	permChecker PermissionChecker
	elog        events.Log
	sessions    session.Service
	role        teleport.Role
	recorder    recorder.Recorder
}

func NewAuthWithRoles(authServer *AuthServer, permChecker PermissionChecker,
	elog events.Log, sessions session.Service,
	role teleport.Role, recorder recorder.Recorder) *AuthWithRoles {

	return &AuthWithRoles{
		authServer:  authServer,
		permChecker: permChecker,
		sessions:    sessions,
		role:        role,
		recorder:    recorder,
		elog:        elog,
	}
}

func (a *AuthWithRoles) GetSessions() ([]session.Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetSessions); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.sessions.GetSessions()
	}
}

func (a *AuthWithRoles) GetSession(id session.ID) (*session.Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetSession); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.sessions.GetSession(id)
	}
}
func (a *AuthWithRoles) CreateSession(s session.Session) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertSession); err != nil {
		return trace.Wrap(err)
	} else {
		return a.sessions.CreateSession(s)
	}
}
func (a *AuthWithRoles) UpdateSession(req session.UpdateRequest) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertSession); err != nil {
		return trace.Wrap(err)
	} else {
		return a.sessions.UpdateSession(req)
	}
}
func (a *AuthWithRoles) UpsertParty(id session.ID, p session.Party, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertParty); err != nil {
		return trace.Wrap(err)
	} else {
		return a.sessions.UpsertParty(id, p, ttl)
	}
}
func (a *AuthWithRoles) UpsertCertAuthority(ca services.CertAuthority, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertCertAuthority); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.UpsertCertAuthority(ca, ttl)
	}
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
	} else {
		return a.authServer.GetLocalDomain()
	}
}

func (a *AuthWithRoles) DeleteCertAuthority(id services.CertAuthID) error {
	if err := a.permChecker.HasPermission(a.role, ActionDeleteCertAuthority); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.DeleteCertAuthority(id)
	}
}
func (a *AuthWithRoles) GenerateToken(role teleport.Role, ttl time.Duration) (string, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGenerateToken); err != nil {
		return "", trace.Wrap(err)
	} else {
		return a.authServer.GenerateToken(role, ttl)
	}
}
func (a *AuthWithRoles) RegisterUsingToken(token, hostID string, role teleport.Role) (*PackedKeys, error) {
	if err := a.permChecker.HasPermission(a.role, ActionRegisterUsingToken); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.RegisterUsingToken(token, hostID, role)
	}
}
func (a *AuthWithRoles) RegisterNewAuthServer(token string) error {
	if err := a.permChecker.HasPermission(a.role, ActionRegisterNewAuthServer); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.RegisterNewAuthServer(token)
	}
}
func (a *AuthWithRoles) Log(id lunk.EventID, e lunk.Event) {
	if err := a.permChecker.HasPermission(a.role, ActionLog); err != nil {
		return
	} else {
		a.elog.Log(id, e)
	}
}
func (a *AuthWithRoles) LogEntry(en lunk.Entry) error {
	if err := a.permChecker.HasPermission(a.role, ActionLogEntry); err != nil {
		return trace.Wrap(err)
	} else {
		return a.elog.LogEntry(en)
	}
}
func (a *AuthWithRoles) GetEvents(filter events.Filter) ([]lunk.Entry, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetEvents); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.elog.GetEvents(filter)
	}
}
func (a *AuthWithRoles) LogSession(sess session.Session) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertSession); err != nil {
		return trace.Wrap(err)
	} else {
		return a.elog.LogSession(sess)
	}
}
func (a *AuthWithRoles) GetSessionEvents(filter events.Filter) ([]session.Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetSessions); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.elog.GetSessionEvents(filter)
	}
}
func (a *AuthWithRoles) GetChunkWriter(id string) (recorder.ChunkWriteCloser, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetChunkWriter); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.recorder.GetChunkWriter(id)
	}
}
func (a *AuthWithRoles) GetChunkReader(id string) (recorder.ChunkReadCloser, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetChunkReader); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.recorder.GetChunkReader(id)
	}
}
func (a *AuthWithRoles) UpsertNode(s services.Server, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertServer); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.UpsertNode(s, ttl)
	}
}
func (a *AuthWithRoles) GetNodes() ([]services.Server, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetServers); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.GetNodes()
	}
}
func (a *AuthWithRoles) UpsertAuthServer(s services.Server, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertAuthServer); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.UpsertAuthServer(s, ttl)
	}
}
func (a *AuthWithRoles) GetAuthServers() ([]services.Server, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetAuthServers); err != nil {
		return nil, err
	} else {
		return a.authServer.GetAuthServers()
	}
}
func (a *AuthWithRoles) UpsertProxy(s services.Server, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertProxy); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.UpsertProxy(s, ttl)
	}
}
func (a *AuthWithRoles) GetProxies() ([]services.Server, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetProxies); err != nil {
		return nil, err
	} else {
		return a.authServer.GetProxies()
	}
}
func (a *AuthWithRoles) UpsertReverseTunnel(r services.ReverseTunnel, ttl time.Duration) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertReverseTunnel); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.UpsertReverseTunnel(r, ttl)
	}
}
func (a *AuthWithRoles) GetReverseTunnels() ([]services.ReverseTunnel, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetReverseTunnels); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.GetReverseTunnels()
	}
}
func (a *AuthWithRoles) DeleteReverseTunnel(domainName string) error {
	if err := a.permChecker.HasPermission(a.role, ActionDeleteReverseTunnel); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.DeleteReverseTunnel(domainName)
	}
}
func (a *AuthWithRoles) UpsertPassword(user string, password []byte) (hotpURL string, hotpQR []byte, err error) {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertPassword); err != nil {
		return "", nil, err
	} else {
		return a.authServer.UpsertPassword(user, password)
	}
}
func (a *AuthWithRoles) CheckPassword(user string, password []byte, hotpToken string) error {
	if err := a.permChecker.HasPermission(a.role, ActionCheckPassword); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.CheckPassword(user, password, hotpToken)
	}
}
func (a *AuthWithRoles) SignIn(user string, password []byte) (*Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionSignIn); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.SignIn(user, password)
	}
}
func (a *AuthWithRoles) CreateWebSession(user string, prevSessionID string) (*Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionCreateWebSession); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.CreateWebSession(user, prevSessionID)
	}
}
func (a *AuthWithRoles) GetWebSessionInfo(user string, sid string) (*Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetWebSession); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.GetWebSessionInfo(user, sid)
	}
}
func (a *AuthWithRoles) GetWebSessionsKeys(user string) ([]services.AuthorizedKey, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetWebSessionsKeys); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.GetWebSessionsKeys(user)
	}
}
func (a *AuthWithRoles) DeleteWebSession(user string, sid string) error {
	if err := a.permChecker.HasPermission(a.role, ActionDeleteWebSession); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.DeleteWebSession(user, sid)
	}
}
func (a *AuthWithRoles) GetUsers() ([]services.User, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetUsers); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.GetUsers()
	}
}
func (a *AuthWithRoles) DeleteUser(user string) error {
	if err := a.permChecker.HasPermission(a.role, ActionDeleteUser); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.DeleteUser(user)
	}
}
func (a *AuthWithRoles) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGenerateKeyPair); err != nil {
		return nil, nil, trace.Wrap(err)
	} else {
		return a.authServer.GenerateKeyPair(pass)
	}
}
func (a *AuthWithRoles) GenerateHostCert(
	key []byte, hostname, authDomain string, role teleport.Role,
	ttl time.Duration) ([]byte, error) {

	if err := a.permChecker.HasPermission(a.role, ActionGenerateHostCert); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.GenerateHostCert(key, hostname, authDomain, role, ttl)
	}
}
func (a *AuthWithRoles) GenerateUserCert(key []byte, user string, ttl time.Duration) ([]byte, error) {
	if err := a.permChecker.HasPermission(a.role, ActionGenerateUserCert); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.GenerateUserCert(key, user, ttl)
	}
}
func (a *AuthWithRoles) CreateSignupToken(user services.User) (token string, e error) {
	if err := a.permChecker.HasPermission(a.role, ActionCreateSignupToken); err != nil {
		return "", trace.Wrap(err)
	} else {
		return a.authServer.CreateSignupToken(user)
	}
}

func (a *AuthWithRoles) GetSignupTokenData(token string) (user string,
	QRImg []byte, hotpFirstValues []string, e error) {
	if err := a.permChecker.HasPermission(a.role, ActionGetSignupTokenData); err != nil {
		return "", nil, nil, trace.Wrap(err)
	} else {
		return a.authServer.GetSignupTokenData(token)
	}
}

func (a *AuthWithRoles) CreateUserWithToken(token, password, hotpToken string) (*Session, error) {
	if err := a.permChecker.HasPermission(a.role, ActionCreateUserWithToken); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return a.authServer.CreateUserWithToken(token, password, hotpToken)
	}
}

func (a *AuthWithRoles) UpsertUser(u services.User) error {
	if err := a.permChecker.HasPermission(a.role, ActionUpsertUser); err != nil {
		return trace.Wrap(err)
	} else {
		return a.authServer.UpsertUser(u)
	}
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
