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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	log "github.com/sirupsen/logrus"

	"github.com/tstranex/u2f"
)

// SessionContext is a context associated with users'
// web session, it stores connected client that persists
// between requests for example to avoid connecting
// to the auth server on every page hit
type SessionContext struct {
	sync.Mutex
	*log.Entry
	sess    services.WebSession
	user    string
	clt     *auth.TunClient
	parent  *sessionCache
	closers []io.Closer
}

// getTerminal finds and returns an active web terminal for a given session:
func (c *SessionContext) getTerminal(sessionID session.ID) (*terminalHandler, error) {
	c.Lock()
	defer c.Unlock()

	for _, closer := range c.closers {
		term, ok := closer.(*terminalHandler)
		if ok && term.params.SessionID == sessionID {
			return term, nil
		}
	}
	return nil, trace.NotFound("no connected streams")
}

// UpdateSessionTerminal is called when a browser window is resized and
// we need to update PTY on the server side
func (c *SessionContext) UpdateSessionTerminal(
	siteAPI auth.ClientI, namespace string, sessionID session.ID, params session.TerminalParams) error {

	// update the session size on the auth server's side
	err := siteAPI.UpdateSession(session.UpdateRequest{
		ID:             sessionID,
		TerminalParams: &params,
		Namespace:      namespace,
	})
	if err != nil {
		log.Error(err)
	}
	// update the server-side PTY to match the browser window size
	term, err := c.getTerminal(sessionID)
	if err != nil {
		log.Error(err)
		return trace.Wrap(err)
	}
	return trace.Wrap(term.resizePTYWindow(params))
}

func (c *SessionContext) AddClosers(closers ...io.Closer) {
	c.Lock()
	defer c.Unlock()
	c.closers = append(c.closers, closers...)
}

func (c *SessionContext) RemoveCloser(closer io.Closer) {
	c.Lock()
	defer c.Unlock()
	for i := range c.closers {
		if c.closers[i] == closer {
			c.closers = append(c.closers[:i], c.closers[i+1:]...)
			return
		}
	}
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
func (c *SessionContext) GetWebSession() services.WebSession {
	return c.sess
}

// ExtendWebSession creates a new web session for this user
// based on the previous session
func (c *SessionContext) ExtendWebSession() (services.WebSession, error) {
	sess, err := c.clt.ExtendWebSession(c.user, c.sess.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

// GetAgent returns agent that can we used to answer challenges
// for the web to ssh connection
func (c *SessionContext) GetAgent() (auth.AgentCloser, error) {
	return c.clt.GetAgent()
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
	log.Infof("[WEB] closing session cache")
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
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.clearExpiredSessions()
		case <-s.closer.C:
			return
		}
	}
}

func (s *sessionCache) clearExpiredSessions() {
	s.Lock()
	defer s.Unlock()
	expired := s.contexts.RemoveExpired(10)
	if expired != 0 {
		log.Infof("[WEB] removed %v expired sessions", expired)
	}
}

func (s *sessionCache) AuthWithOTP(user, pass string, otpToken string) (services.WebSession, error) {
	method, err := auth.NewWebPasswordAuth(user, []byte(pass), otpToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// the act of creating a tunnel performs authentication. since we are
	// only using the tunnel for authentication, we close it right afterwards
	// because we will not be using this connection (initiated using password
	// based credentials) later.
	clt, err := auth.NewTunClient("web.client.password-and-otp", s.authServers, user, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	session, err := clt.SignIn(user, []byte(pass))
	if err != nil {
		defer clt.Close()
		return nil, trace.Wrap(err)
	}
	return session, nil
}

func (s *sessionCache) AuthWithoutOTP(user, pass string) (services.WebSession, error) {
	method, err := auth.NewWebPasswordWithoutOTPAuth(user, []byte(pass))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := auth.NewTunClient("web.client.password-only", s.authServers, user, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	session, err := clt.SignIn(user, []byte(pass))
	if err != nil {
		defer clt.Close()
		return nil, trace.Wrap(err)
	}
	return session, nil
}

func (s *sessionCache) GetU2FSignRequest(user, pass string) (*u2f.SignRequest, error) {
	method, err := auth.NewWebPasswordU2FSignAuth(user, []byte(pass))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient("web.get-u2f-sign-request", s.authServers, user, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// we are always closing this client, because we will not be using
	// this connection initiated using password based credentials
	// down the road, so it's a one call client
	defer clt.Close()
	u2fSignReq, err := clt.GetU2FSignRequest(user, []byte(pass))
	if err != nil {
		defer clt.Close()
		return nil, trace.Wrap(err)
	}
	return u2fSignReq, nil
}

func (s *sessionCache) AuthWithU2FSignResponse(user string, response *u2f.SignResponse) (services.WebSession, error) {
	method, err := auth.NewWebU2FSignResponseAuth(user, response)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient("web.client-u2f", s.authServers, user, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()
	session, err := clt.PreAuthenticatedSignIn(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return session, nil
}

func (s *sessionCache) GetCertificateWithoutOTP(c client.CreateSSHCertReq) (*client.SSHLoginResponse, error) {
	method, err := auth.NewWebPasswordWithoutOTPAuth(c.User, []byte(c.Password))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := auth.NewTunClient("web.session.password-only", s.authServers, c.User, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	return createCertificate(c.User, c.PubKey, c.TTL, c.Compatibility, clt)
}

func (s *sessionCache) GetCertificateWithOTP(c client.CreateSSHCertReq) (*client.SSHLoginResponse, error) {
	method, err := auth.NewWebPasswordAuth(c.User, []byte(c.Password), c.OTPToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := auth.NewTunClient("web.session.password+otp", s.authServers, c.User, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	return createCertificate(c.User, c.PubKey, c.TTL, c.Compatibility, clt)
}

func createCertificate(user string, pubkey []byte, ttl time.Duration, compatibility string, clt *auth.TunClient) (*client.SSHLoginResponse, error) {
	cert, err := clt.GenerateUserCert(pubkey, user, ttl, compatibility)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostSigners, err := clt.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signers, err := services.CertAuthoritiesToV1(hostSigners)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &client.SSHLoginResponse{
		Cert:        cert,
		HostSigners: signers,
	}, nil
}

func (s *sessionCache) GetCertificateWithU2F(c client.CreateSSHCertWithU2FReq) (*client.SSHLoginResponse, error) {
	method, err := auth.NewWebU2FSignResponseAuth(c.User, &c.U2FSignResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := auth.NewTunClient("web.session-u2f", s.authServers, c.User, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	return createCertificate(c.User, c.PubKey, c.TTL, c.Compatibility, clt)
}

func (s *sessionCache) GetUserInviteInfo(token string) (user string, otpQRCode []byte, err error) {
	method, err := auth.NewSignupTokenAuth(token)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	clt, err := auth.NewTunClient("web.get-user-invite", s.authServers, "tokenAuth", method)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	defer clt.Close()

	return clt.GetSignupTokenData(token)
}

func (s *sessionCache) GetUserInviteU2FRegisterRequest(token string) (u2fRegisterRequest *u2f.RegisterRequest, e error) {
	method, err := auth.NewSignupTokenAuth(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient("web.get-user-invite-u2f-register-request", s.authServers, "tokenAuth", method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	return clt.GetSignupU2FRegisterRequest(token)
}

func (s *sessionCache) CreateNewUser(token, password, otpToken string) (services.WebSession, error) {
	method, err := auth.NewSignupTokenAuth(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := auth.NewTunClient("web.create-user", s.authServers, "tokenAuth", method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	cap, err := clt.GetAuthPreference()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var webSession services.WebSession

	switch cap.GetSecondFactor() {
	case teleport.OFF:
		webSession, err = clt.CreateUserWithoutOTP(token, password)
	case teleport.OTP, teleport.TOTP, teleport.HOTP:
		webSession, err = clt.CreateUserWithOTP(token, password, otpToken)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return webSession, nil
}

func (s *sessionCache) CreateNewU2FUser(token string, password string, u2fRegisterResponse u2f.RegisterResponse) (services.WebSession, error) {
	method, err := auth.NewSignupTokenAuth(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := auth.NewTunClient("web.create-u2f-user", s.authServers, "tokenAuth", method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()
	sess, err := clt.CreateUserWithU2FToken(token, password, u2fRegisterResponse)
	return sess, trace.Wrap(err)
}

func (s *sessionCache) ValidateTrustedCluster(validateRequest *auth.ValidateTrustedClusterRequest) (*auth.ValidateTrustedClusterResponse, error) {
	method, err := auth.NewValidateTrustedClusterAuth(validateRequest.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := auth.NewTunClient("web.validate-trusted-cluster", s.authServers, "tokenAuth", method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	validateResponse, err := clt.ValidateTrustedCluster(validateRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return validateResponse, nil
}

func (s *sessionCache) InvalidateSession(ctx *SessionContext) error {
	defer ctx.Close()
	if err := s.resetContext(ctx.GetUser(), ctx.GetWebSession().GetName()); err != nil {
		return trace.Wrap(err)
	}
	clt, err := ctx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}
	err = clt.DeleteWebSession(ctx.GetUser(), ctx.GetWebSession().GetName())
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
	log.Debugf("ValidateSession(%s, %s)", user, sid)
	method, err := auth.NewWebSessionAuth(user, []byte(sid))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Note: do not close this auth API client now. It will exist inside of "session context"
	clt, err := auth.NewTunClient("web.session-user", s.authServers, user, method)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := clt.GetWebSessionInfo(user, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c := &SessionContext{
		clt:    clt,
		user:   user,
		sess:   sess,
		parent: s,
	}
	c.Entry = log.WithFields(log.Fields{
		"user": user,
		"sess": sess.GetShortName(),
	})

	ttl := utils.ToTTL(clockwork.NewRealClock(), sess.GetBearerTokenExpiryTime())
	out, err := s.insertContext(user, sid, c, ttl)
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
