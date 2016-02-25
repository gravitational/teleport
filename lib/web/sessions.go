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

package web

import (
	"net/http"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"github.com/mailgun/ttlmap"
	"golang.org/x/crypto/ssh"
)

// sessionContext is a context associated with users'
// web session, it stores connected client that persists
// between requests for example to avoid connecting
// to the auth server on every page hit
type sessionContext struct {
	sess *auth.Session
	user string
	clt  *auth.TunClient
}

// GetClient returns the client connected to the auth server
func (c *sessionContext) GetClient() (auth.ClientI, error) {
	return c.clt, nil
}

// GetUser returns the authenticated teleport user
func (c *sessionContext) GetUser() string {
	return c.user
}

// GetWebSession returns a web session
func (c *sessionContext) GetWebSession() *auth.Session {
	return c.sess
}

// GetAuthMethods returns authentication methods (credentials) that proxy
// can use to connect to servers
func (c *sessionContext) GetAuthMethods() ([]ssh.AuthMethod, error) {
	a, err := c.clt.GetAgent()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signers, err := a.Signers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signers...)}, nil
}

// Close cleans up connections associated with requests
func (c *sessionContext) Close() error {
	if c.clt != nil {
		return trace.Wrap(c.clt.Close())
	}
	return nil
}

// newSessionHandler returns new instance of the session handler
func newSessionHandler(secure bool, servers []utils.NetAddr) (*sessionHandler, error) {
	m, err := ttlmap.NewMap(1024, ttlmap.CallOnExpire(closeContext))
	if err != nil {
		return nil, err
	}
	return &sessionHandler{
		contexts:    m,
		authServers: servers,
	}, nil
}

// sessionHandler handles web session authentication,
// and holds in memory contexts associated with each session
type sessionHandler struct {
	secure      bool
	contexts    *ttlmap.TtlMap
	authServers []utils.NetAddr
}

// closeContext is called when session context expires from
// cache and will clean up connections
func closeContext(key string, val interface{}) {
	log.Infof("closing context %v", key)
	ctx := val.(*sessionContext)
	if err := ctx.Close(); err != nil {
		log.Infof("failed to close context: %v", err)
	}
}

func (s *sessionHandler) Auth(user, pass string, hotpToken string) (*auth.Session, error) {
	method, err := auth.NewWebPasswordAuth(user, []byte(pass), hotpToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient(s.authServers[0], user, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt.SignIn(user, []byte(pass))
}

func (s *sessionHandler) GetCertificate(c createSSHCertReq) (*SSHLoginResponse, error) {
	method, err := auth.NewWebPasswordAuth(c.User, []byte(c.Password),
		c.HOTPToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := auth.NewTunClient(s.authServers[0], c.User, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := clt.GenerateUserCert(c.PubKey, c.User, c.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostSigners, err := clt.GetCertAuthorities(services.HostCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signers := []services.CertAuthority{}
	for _, hs := range hostSigners {
		signers = append(signers, *hs)
	}

	return &SSHLoginResponse{
		Cert:        cert,
		HostSigners: signers,
	}, nil
}

func (s *sessionHandler) GetUserInviteInfo(token string) (user string,
	QRImg []byte, hotpFirstValues []string, e error) {

	method, err := auth.NewSignupTokenAuth(token)
	if err != nil {
		return "", nil, nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient(s.authServers[0], "tokenAuth", method)
	if err != nil {
		return "", nil, nil, trace.Wrap(err)
	}

	return clt.GetSignupTokenData(token)
}

func (s *sessionHandler) CreateNewUser(token, password, hotpToken string) (*auth.Session, error) {
	method, err := auth.NewSignupTokenAuth(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient(s.authServers[0], "tokenAuth", method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := clt.CreateUserWithToken(token, password, hotpToken)
	return sess, trace.Wrap(err)
}

func (s *sessionHandler) ValidateSession(user, sid string) (*sessionContext, error) {
	val, ok := s.contexts.Get(user + sid)
	if ok {
		return val.(*sessionContext), nil
	}
	method, err := auth.NewWebSessionAuth(user, []byte(sid))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient(s.authServers[0], user, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := clt.GetWebSessionInfo(user, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c := &sessionContext{
		clt:  clt,
		user: user,
		sess: sess,
	}
	const localCacheTTL = 600
	if err := s.contexts.Set(user+sid, c, localCacheTTL); err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

func (s *sessionHandler) SetSession(w http.ResponseWriter, user, sid string) error {
	d, err := EncodeCookie(user, sid)
	if err != nil {
		return err
	}
	c := &http.Cookie{
		Name:     "session",
		Value:    d,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
	}
	http.SetCookie(w, c)
	return nil
}

func (s *sessionHandler) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
	})
}
