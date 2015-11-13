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
	"time"

	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	websession "github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
)

type AuthWithRoles struct {
	a    *AuthServer
	c    PermissionChecker
	elog events.Log
	se   session.SessionServer
	role string
	rec  recorder.Recorder
}

func NewAuthWithRoles(a *AuthServer, c PermissionChecker,
	elog events.Log, se session.SessionServer,
	role string, rec recorder.Recorder) *AuthWithRoles {

	return &AuthWithRoles{
		a:    a,
		c:    c,
		se:   se,
		role: role,
		rec:  rec,
	}
}

func (a *AuthWithRoles) GetSessions() ([]session.Session, error) {
	if err := a.c.HasPermission(a.role, ActionGetSessions); err != nil {
		return nil, err
	} else {
		return a.se.GetSessions()
	}
}

func (a *AuthWithRoles) GetSession(id string) (*session.Session, error) {
	if err := a.c.HasPermission(a.role, ActionGetSession); err != nil {
		return nil, err
	} else {
		return a.se.GetSession(id)
	}
}
func (a *AuthWithRoles) DeleteSession(id string) error {
	if err := a.c.HasPermission(a.role, ActionDeleteSession); err != nil {
		return err
	} else {
		return a.se.DeleteSession(id)
	}
}
func (a *AuthWithRoles) UpsertSession(id string, ttl time.Duration) error {
	if err := a.c.HasPermission(a.role, ActionUpsertSession); err != nil {
		return err
	} else {
		return a.se.UpsertSession(id, ttl)
	}
}
func (a *AuthWithRoles) UpsertParty(id string, p session.Party, ttl time.Duration) error {
	if err := a.c.HasPermission(a.role, ActionUpsertParty); err != nil {
		return err
	} else {
		return a.se.UpsertParty(id, p, ttl)
	}
}
func (a *AuthWithRoles) UpsertRemoteCert(cert services.RemoteCert, ttl time.Duration) error {
	if err := a.c.HasPermission(a.role, ActionUpsertRemoteCert); err != nil {
		return err
	} else {
		return a.a.UpsertRemoteCert(cert, ttl)
	}
}
func (a *AuthWithRoles) GetRemoteCerts(ctype string, fqdn string) ([]services.RemoteCert, error) {
	if err := a.c.HasPermission(a.role, ActionGetRemoteCerts); err != nil {
		return nil, err
	} else {
		return a.a.GetRemoteCerts(ctype, fqdn)
	}
}
func (a *AuthWithRoles) DeleteRemoteCert(ctype string, fqdn, id string) error {
	if err := a.c.HasPermission(a.role, ActionDeleteRemoteCert); err != nil {
		return err
	} else {
		return a.a.DeleteRemoteCert(ctype, fqdn, id)
	}
}
func (a *AuthWithRoles) GenerateToken(fqdn, role string, ttl time.Duration) (string, error) {
	if err := a.c.HasPermission(a.role, ActionGenerateToken); err != nil {
		return "", err
	} else {
		return a.a.GenerateToken(fqdn, role, ttl)
	}
}
func (a *AuthWithRoles) RegisterUsingToken(token, fqdn, role string) (keys PackedKeys, e error) {
	if err := a.c.HasPermission(a.role, ActionRegisterUsingToken); err != nil {
		return PackedKeys{}, err
	} else {
		return a.a.RegisterUsingToken(token, fqdn, role)
	}
}
func (a *AuthWithRoles) RegisterNewAuthServer(fqdn, token string,
	publicSealKey encryptor.Key) (masterKey encryptor.Key, e error) {

	if err := a.c.HasPermission(a.role, ActionRegisterNewAuthServer); err != nil {
		return encryptor.Key{}, err
	} else {
		return a.a.RegisterNewAuthServer(fqdn, token, publicSealKey)
	}
}
func (a *AuthWithRoles) Log(id lunk.EventID, e lunk.Event) {
	if err := a.c.HasPermission(a.role, ActionLog); err != nil {
		return
	} else {
		a.elog.Log(id, e)
	}
}
func (a *AuthWithRoles) LogEntry(en lunk.Entry) error {
	if err := a.c.HasPermission(a.role, ActionLogEntry); err != nil {
		return err
	} else {
		return a.elog.LogEntry(en)
	}
}
func (a *AuthWithRoles) GetEvents(filter events.Filter) ([]lunk.Entry, error) {
	if err := a.c.HasPermission(a.role, ActionGetEvents); err != nil {
		return nil, err
	} else {
		return a.elog.GetEvents(filter)
	}
}
func (a *AuthWithRoles) GetChunkWriter(id string) (recorder.ChunkWriteCloser, error) {
	if err := a.c.HasPermission(a.role, ActionGetChunkWriter); err != nil {
		return nil, err
	} else {
		return a.rec.GetChunkWriter(id)
	}
}
func (a *AuthWithRoles) GetChunkReader(id string) (recorder.ChunkReadCloser, error) {
	if err := a.c.HasPermission(a.role, ActionGetChunkReader); err != nil {
		return nil, err
	} else {
		return a.rec.GetChunkReader(id)
	}
}
func (a *AuthWithRoles) UpsertServer(s services.Server, ttl time.Duration) error {
	if err := a.c.HasPermission(a.role, ActionUpsertServer); err != nil {
		return err
	} else {
		return a.a.UpsertServer(s, ttl)
	}
}
func (a *AuthWithRoles) GetServers() ([]services.Server, error) {
	if err := a.c.HasPermission(a.role, ActionGetServers); err != nil {
		return nil, err
	} else {
		return a.a.GetServers()
	}
}
func (a *AuthWithRoles) UpsertWebTun(wt services.WebTun, ttl time.Duration) error {
	if err := a.c.HasPermission(a.role, ActionUpsertWebTun); err != nil {
		return err
	} else {
		return a.a.UpsertWebTun(wt, ttl)
	}
}
func (a *AuthWithRoles) GetWebTuns() ([]services.WebTun, error) {
	if err := a.c.HasPermission(a.role, ActionGetWebTuns); err != nil {
		return nil, err
	} else {
		return a.a.GetWebTuns()
	}
}
func (a *AuthWithRoles) GetWebTun(prefix string) (*services.WebTun, error) {
	if err := a.c.HasPermission(a.role, ActionGetWebTun); err != nil {
		return nil, err
	} else {
		return a.a.GetWebTun(prefix)
	}
}
func (a *AuthWithRoles) DeleteWebTun(prefix string) error {
	if err := a.c.HasPermission(a.role, ActionDeleteWebTun); err != nil {
		return err
	} else {
		return a.a.DeleteWebTun(prefix)
	}
}
func (a *AuthWithRoles) UpsertPassword(user string, password []byte) (hotpURL string, hotpQR []byte, err error) {
	if err := a.c.HasPermission(a.role, ActionUpsertPassword); err != nil {
		return "", nil, err
	} else {
		return a.a.UpsertPassword(user, password)
	}
}
func (a *AuthWithRoles) CheckPassword(user string, password []byte, hotpToken string) error {
	if err := a.c.HasPermission(a.role, ActionCheckPassword); err != nil {
		return err
	} else {
		return a.a.CheckPassword(user, password, hotpToken)
	}
}
func (a *AuthWithRoles) SignIn(user string, password []byte) (*Session, error) {
	if err := a.c.HasPermission(a.role, ActionSignIn); err != nil {
		return nil, err
	} else {
		return a.a.SignIn(user, password)
	}
}
func (a *AuthWithRoles) GetWebSession(user string, sid websession.SecureID) (*Session, error) {
	if err := a.c.HasPermission(a.role, ActionGetWebSession); err != nil {
		return nil, err
	} else {
		return a.a.GetWebSession(user, sid)
	}
}
func (a *AuthWithRoles) GetWebSessionsKeys(user string) ([]services.AuthorizedKey, error) {
	if err := a.c.HasPermission(a.role, ActionGetWebSessionsKeys); err != nil {
		return nil, err
	} else {
		return a.a.GetWebSessionsKeys(user)
	}
}
func (a *AuthWithRoles) DeleteWebSession(user string, sid websession.SecureID) error {
	if err := a.c.HasPermission(a.role, ActionDeleteWebSession); err != nil {
		return err
	} else {
		return a.a.DeleteWebSession(user, sid)
	}
}
func (a *AuthWithRoles) GetUsers() ([]string, error) {
	if err := a.c.HasPermission(a.role, ActionGetUsers); err != nil {
		return nil, err
	} else {
		return a.a.GetUsers()
	}
}
func (a *AuthWithRoles) DeleteUser(user string) error {
	if err := a.c.HasPermission(a.role, ActionDeleteUser); err != nil {
		return err
	} else {
		return a.a.DeleteUser(user)
	}
}
func (a *AuthWithRoles) UpsertUserKey(username string, key services.AuthorizedKey, ttl time.Duration) ([]byte, error) {
	if err := a.c.HasPermission(a.role, ActionUpsertUserKey); err != nil {
		return nil, err
	} else {
		return a.a.UpsertUserKey(username, key, ttl)
	}
}
func (a *AuthWithRoles) GetUserKeys(user string) ([]services.AuthorizedKey, error) {
	if err := a.c.HasPermission(a.role, ActionGetUserKeys); err != nil {
		return nil, err
	} else {
		return a.a.GetUserKeys(user)
	}
}
func (a *AuthWithRoles) DeleteUserKey(username string, id string) error {
	if err := a.c.HasPermission(a.role, ActionDeleteUserKey); err != nil {
		return err
	} else {
		return a.a.DeleteUserKey(username, id)
	}
}
func (a *AuthWithRoles) GetHostCAPub() ([]byte, error) {
	if err := a.c.HasPermission(a.role, ActionGetHostCAPub); err != nil {
		return nil, err
	} else {
		return a.a.GetHostCAPub()
	}
}
func (a *AuthWithRoles) GetUserCAPub() ([]byte, error) {
	if err := a.c.HasPermission(a.role, ActionGetUserCAPub); err != nil {
		return nil, err
	} else {
		return a.a.GetUserCAPub()
	}
}
func (a *AuthWithRoles) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	if err := a.c.HasPermission(a.role, ActionGenerateKeyPair); err != nil {
		return nil, nil, err
	} else {
		return a.a.GenerateKeyPair(pass)
	}
}
func (a *AuthWithRoles) GenerateHostCert(
	key []byte, id, hostname, role string,
	ttl time.Duration) ([]byte, error) {

	if err := a.c.HasPermission(a.role, ActionGenerateHostCert); err != nil {
		return nil, err
	} else {
		return a.a.GenerateHostCert(key, id, hostname, role, ttl)
	}
}
func (a *AuthWithRoles) GenerateUserCert(key []byte, id, user string, ttl time.Duration) ([]byte, error) {
	if err := a.c.HasPermission(a.role, ActionGenerateUserCert); err != nil {
		return nil, err
	} else {
		return a.a.GenerateUserCert(key, id, user, ttl)
	}
}
func (a *AuthWithRoles) ResetHostCA(pass string) error {
	if err := a.c.HasPermission(a.role, ActionResetHostCA); err != nil {
		return err
	} else {
		return a.a.ResetHostCA(pass)
	}
}
func (a *AuthWithRoles) ResetUserCA(pass string) error {
	if err := a.c.HasPermission(a.role, ActionResetUserCA); err != nil {
		return err
	} else {
		return a.a.ResetUserCA(pass)
	}
}

func (a *AuthWithRoles) GetSealKeys() ([]encryptor.Key, error) {
	if err := a.c.HasPermission(a.role, ActionGetSealKeys); err != nil {
		return nil, err
	} else {
		return a.a.GetSealKeys()
	}
}

func (a *AuthWithRoles) GenerateSealKey(keyName string) (encryptor.Key, error) {
	if err := a.c.HasPermission(a.role, ActionGenerateSealKey); err != nil {
		return encryptor.Key{}, err
	} else {
		return a.a.GenerateSealKey(keyName)
	}
}

func (a *AuthWithRoles) DeleteSealKey(keyID string) error {
	if err := a.c.HasPermission(a.role, ActionDeleteSealKey); err != nil {
		return err
	} else {
		return a.a.DeleteSealKey(keyID)
	}
}

func (a *AuthWithRoles) AddSealKey(key encryptor.Key) error {
	if err := a.c.HasPermission(a.role, ActionAddSealKey); err != nil {
		return err
	} else {
		return a.a.AddSealKey(key)
	}
}

func (a *AuthWithRoles) GetSealKey(keyID string) (encryptor.Key, error) {
	if err := a.c.HasPermission(a.role, ActionGetSealKey); err != nil {
		return encryptor.Key{}, err
	} else {
		return a.a.GetSealKey(keyID)
	}
}
