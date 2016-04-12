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
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
)

// SessionContext is a context associated with users'
// web session, it stores connected client that persists
// between requests for example to avoid connecting
// to the auth server on every page hit
type SessionContext struct {
	sync.Mutex
	*log.Entry
	sess    *auth.Session
	user    string
	clt     *auth.TunClient
	parent  *sessionCache
	closers []io.Closer
}

func (c *SessionContext) getConnectHandler(sessionID session.ID) (*connectHandler, error) {
	c.Lock()
	defer c.Unlock()

	for _, closer := range c.closers {
		handler, ok := closer.(*connectHandler)
		if ok && handler.req.SessionID == sessionID {
			return handler, nil
		}
	}
	return nil, trace.NotFound("no connected streams")
}

func (c *SessionContext) UpdateSessionTerminal(sessionID session.ID, params session.TerminalParams) error {
	err := c.clt.UpdateSession(session.UpdateRequest{ID: sessionID, TerminalParams: &params})
	if err != nil {
		return trace.Wrap(err)
	}
	handler, err := c.getConnectHandler(sessionID)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(handler.resizePTYWindow(params))
}

func (c *SessionContext) AddClosers(closers ...io.Closer) {
	c.Lock()
	defer c.Unlock()
	c.closers = append(c.closers, closers...)
}

func (c *SessionContext) TransferClosers() []io.Closer {
	c.Lock()
	defer c.Unlock()
	closers := c.closers
	c.closers = nil
	return closers
}

func (c *SessionContext) Invalidate() error {
	return c.parent.InvalidateSession(c)
}

// GetClient returns the client connected to the auth server
func (c *SessionContext) GetClient() (auth.ClientI, error) {
	return c.clt, nil
}

// GetUser returns the authenticated teleport user
func (c *SessionContext) GetUser() string {
	return c.user
}

// GetWebSession returns a web session
func (c *SessionContext) GetWebSession() *auth.Session {
	return c.sess
}

// ExtendWebSession creates a new web session for this user
// based on the previous session
func (c *SessionContext) ExtendWebSession() (*auth.Session, error) {
	sess, err := c.clt.ExtendWebSession(c.user, c.sess.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

// GetAgent returns agent that can we used to answer challenges
// for the web to ssh connection
func (c *SessionContext) GetAgent() (auth.AgentCloser, error) {
	a, err := c.clt.GetAgent()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a, nil
}

// Close cleans up connections associated with requests
func (c *SessionContext) Close() error {
	closers := c.TransferClosers()
	for _, closer := range closers {
		c.Infof("closing %v", closer)
		closer.Close()
	}
	if c.clt != nil {
		return trace.Wrap(c.clt.Close())
	}
	return nil
}

// newSessionCache returns new instance of the session cache
func newSessionCache(servers []utils.NetAddr) (*sessionCache, error) {
	m, err := ttlmap.New(1024, ttlmap.CallOnExpire(closeContext))
	if err != nil {
		return nil, err
	}
	cache := &sessionCache{
		contexts:    m,
		authServers: servers,
		closer:      utils.NewCloseBroadcaster(),
	}
	// periodically close expired and unused sessions
	go cache.expireSessions()
	return cache, nil
}

// sessionCache handles web session authentication,
// and holds in memory contexts associated with each session
type sessionCache struct {
	sync.Mutex
	contexts    *ttlmap.TTLMap
	authServers []utils.NetAddr
	closer      *utils.CloseBroadcaster
}

// Close closes all allocated resources and stops goroutines
func (s *sessionCache) Close() error {
	return s.closer.Close()
}

// closeContext is called when session context expires from
// cache and will clean up connections
func closeContext(key string, val interface{}) {
	go func() {
		log.Infof("[WEB] closing context %v", key)
		ctx, ok := val.(*SessionContext)
		if !ok {
			log.Warningf("warning, not valid value type %T", val)
			return
		}
		if err := ctx.Close(); err != nil {
			log.Infof("failed to close context: %v", err)
		}
	}()
}

func (s *sessionCache) expireSessions() {
	ticker := time.NewTicker(time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.clearExpiredSessions()
			case <-s.closer.C:
				return
			}
		}
	}()
}

func (s *sessionCache) clearExpiredSessions() {
	s.Lock()
	defer s.Unlock()
	expired := s.contexts.RemoveExpired(10)
	if expired != 0 {
		log.Infof("[WEB] removed %v expired sessions", expired)
	}
}

func (s *sessionCache) Auth(user, pass string, hotpToken string) (*auth.Session, error) {
	method, err := auth.NewWebPasswordAuth(user, []byte(pass), hotpToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient(s.authServers, user, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// we are always closing this client, because we will not be using
	// this connection initiated using password based credentials
	// down the road, so it's a one call client
	defer clt.Close()
	session, err := clt.SignIn(user, []byte(pass))
	if err != nil {
		defer clt.Close()
		return nil, trace.Wrap(err)
	}
	return session, nil
}

func (s *sessionCache) GetCertificate(c createSSHCertReq) (*SSHLoginResponse, error) {
	method, err := auth.NewWebPasswordAuth(c.User, []byte(c.Password),
		c.HOTPToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient(s.authServers, c.User, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := clt.GenerateUserCert(c.PubKey, c.User, c.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostSigners, err := clt.GetCertAuthorities(services.HostCA, false)
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

func (s *sessionCache) GetUserInviteInfo(token string) (user string,
	QRImg []byte, hotpFirstValues []string, e error) {

	method, err := auth.NewSignupTokenAuth(token)
	if err != nil {
		return "", nil, nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient(s.authServers, "tokenAuth", method)
	if err != nil {
		return "", nil, nil, trace.Wrap(err)
	}

	return clt.GetSignupTokenData(token)
}

func (s *sessionCache) CreateNewUser(token, password, hotpToken string) (*auth.Session, error) {
	method, err := auth.NewSignupTokenAuth(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient(s.authServers, "tokenAuth", method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := clt.CreateUserWithToken(token, password, hotpToken)
	return sess, trace.Wrap(err)
}

func (s *sessionCache) InvalidateSession(ctx *SessionContext) error {
	defer ctx.Close()
	if err := s.resetContext(ctx.GetUser(), ctx.GetWebSession().ID); err != nil {
		return trace.Wrap(err)
	}
	clt, err := ctx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}
	err = clt.DeleteWebSession(ctx.GetUser(), ctx.GetWebSession().ID)
	return trace.Wrap(err)
}

func (s *sessionCache) getContext(user, sid string) (*SessionContext, error) {
	s.Lock()
	defer s.Unlock()

	val, ok := s.contexts.Get(user + sid)
	if ok {
		return val.(*SessionContext), nil
	}
	return nil, trace.NotFound("sessionContext not found")
}

func (s *sessionCache) insertContext(user, sid string, ctx *SessionContext, ttl time.Duration) (*SessionContext, error) {
	s.Lock()
	defer s.Unlock()

	val, ok := s.contexts.Get(user + sid)
	if ok && val != nil { // nil means that we've just invalidated the context now and set it to nil in the cache
		return val.(*SessionContext), trace.AlreadyExists("exists")
	}
	if err := s.contexts.Set(user+sid, ctx, ttl); err != nil {
		return nil, trace.Wrap(err)
	}
	return ctx, nil
}

func (s *sessionCache) resetContext(user, sid string) error {
	s.Lock()
	defer s.Unlock()
	context, ok := s.contexts.Remove(user + sid)
	if ok {
		closeContext(user+sid, context)
	}
	return nil
}

func (s *sessionCache) ValidateSession(user, sid string) (*SessionContext, error) {
	ctx, err := s.getContext(user, sid)
	if err == nil {
		return ctx, nil
	}
	method, err := auth.NewWebSessionAuth(user, []byte(sid))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient(s.authServers, user, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := clt.GetWebSessionInfo(user, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()
	c := &SessionContext{
		clt:    clt,
		user:   user,
		sess:   sess,
		parent: s,
	}
	c.Entry = log.WithFields(log.Fields{
		"user": user,
		"sess": sess.ID[:4],
	})

	out, err := s.insertContext(user, sid, c, auth.WebSessionTTL)
	if err != nil {
		// this means that someone has just inserted the context, so
		// close our extra context and return
		if trace.IsAlreadyExists(err) {
			log.Infof("just created, returning the existing one")
			defer c.Close()
			return out, nil
		}
		return nil, trace.Wrap(err)
	}
	return out, nil
}

func (s *sessionCache) SetSession(w http.ResponseWriter, user, sid string) error {
	d, err := EncodeCookie(user, sid)
	if err != nil {
		return err
	}
	c := &http.Cookie{
		Name:     "session",
		Value:    d,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}
	http.SetCookie(w, c)
	return nil
}

func (s *sessionCache) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	})
}
