/*
Copyright 2015-2019 Gravitational, Inc.

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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// SessionContext is a context associated with users'
// web session, it stores connected client that persists
// between requests for example to avoid connecting
// to the auth server on every page hit
type SessionContext struct {
	log    logrus.FieldLogger
	user   string
	clt    auth.ClientI
	parent *sessionCache
	// session refers the web session created for the user.
	session services.WebSession

	mu        sync.Mutex
	remoteClt map[string]auth.ClientI
	closers   []io.Closer
}

// String returns the text representation of this context
func (c *SessionContext) String() string {
	return fmt.Sprintf("session(user=%v,id=%v,ttl=%v)",
		c.user,
		c.session.GetName(),
		c.session.GetExpiryTime(),
	)
}

// AddClosers adds the specified closers to this context
func (c *SessionContext) AddClosers(closers ...io.Closer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closers = append(c.closers, closers...)
}

// RemoevCloser removes the specified closer from this context
func (c *SessionContext) RemoveCloser(closer io.Closer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, cls := range c.closers {
		if cls == closer {
			c.closers = append(c.closers[:i], c.closers[i+1:]...)
			return
		}
	}
}

// Invalidate invalidates this context by removing the underlying session
// and closing all underlying closers
func (c *SessionContext) Invalidate() error {
	return c.parent.invalidateSession(c)
}

func (c *SessionContext) validateBearerToken(token string) error {
	_, err := readBearerToken(ctx, c.parent.accessPoint, services.GetTokenRequest{
		User:  c.user,
		Token: token,
	})
	return trace.Wrap(err)
}

func (c *SessionContext) addRemoteClient(siteName string, remoteClient auth.ClientI) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remoteClt[siteName] = remoteClient
}

func (c *SessionContext) getRemoteClient(siteName string) (auth.ClientI, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	remoteClt, ok := c.remoteClt[siteName]
	return remoteClt, ok
}

// GetClient returns the client connected to the auth server
func (c *SessionContext) GetClient() (auth.ClientI, error) {
	return c.clt, nil
}

// GetUserClient will return an auth.ClientI with the role of the user at
// the requested site. If the site is local a client with the users local role
// is returned. If the site is remote a client with the users remote role is
// returned.
func (c *SessionContext) GetUserClient(site reversetunnel.RemoteSite) (auth.ClientI, error) {
	// get the name of the current cluster
	clusterName, err := c.clt.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if we're trying to access the local cluster, pass back the local client.
	if clusterName.GetClusterName() == site.GetName() {
		return c.clt, nil
	}

	// look to see if we already have a connection to this cluster
	remoteClt, ok := c.getRemoteClient(site.GetName())
	if !ok {
		rClt, err := c.newRemoteClient(site)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// we'll save the remote client in our session context so we don't have to
		// build a new connection next time. all remote clients will be closed when
		// the session context is closed.
		c.addRemoteClient(site.GetName(), rClt)

		return rClt, nil
	}

	return remoteClt, nil
}

// newRemoteClient returns a client to a remote cluster with the role of
// the logged in user.
func (c *SessionContext) newRemoteClient(cluster reversetunnel.RemoteSite) (auth.ClientI, error) {
	clt, err := c.tryRemoteTLSClient(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

// clusterDialer returns DialContext function using cluster's dial function
func clusterDialer(remoteCluster reversetunnel.RemoteSite) auth.ContextDialer {
	return auth.ContextDialerFunc(func(in context.Context, network, _ string) (net.Conn, error) {
		return remoteCluster.DialAuthServer()
	})
}

// tryRemoteTLSClient tries creating TLS client and using it (the client may not be available
// due to older clusters), returns client if it is working properly
func (c *SessionContext) tryRemoteTLSClient(cluster reversetunnel.RemoteSite) (auth.ClientI, error) {
	clt, err := c.newRemoteTLSClient(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = clt.GetDomainName()
	if err != nil {
		return clt, trace.Wrap(err)
	}
	return clt, nil
}

// ClientTLSConfig returns client TLS authentication associated
// with the web session context
func (c *SessionContext) ClientTLSConfig(clusterName ...string) (*tls.Config, error) {
	var certPool *x509.CertPool
	if len(clusterName) == 0 {
		certAuthorities, err := c.parent.proxyClient.GetCertAuthorities(services.HostCA, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPool, err = services.CertPoolFromCertAuthorities(certAuthorities)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		certAuthority, err := c.parent.proxyClient.GetCertAuthority(services.CertAuthID{
			Type:       services.HostCA,
			DomainName: clusterName[0],
		}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPool, err = services.CertPool(certAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	tlsConfig := utils.TLSConfig(c.parent.cipherSuites)
	tlsCert, err := tls.X509KeyPair(c.session.GetTLSCert(), c.session.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS cert and key")
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.RootCAs = certPool
	tlsConfig.ServerName = auth.EncodeClusterName(c.parent.clusterName)
	return tlsConfig, nil
}

func (c *SessionContext) newRemoteTLSClient(cluster reversetunnel.RemoteSite) (auth.ClientI, error) {
	tlsConfig, err := c.ClientTLSConfig(cluster.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return auth.NewClient(apiclient.Config{Dialer: clusterDialer(cluster), TLS: tlsConfig})
}

// GetUser returns the authenticated teleport user
func (c *SessionContext) GetUser() string {
	return c.user
}

// GetWebSession returns a web session
func (c *SessionContext) GetWebSession() services.WebSession {
	return c.session
}

// ExtendWebSession creates a new web session for this user
// based on the previous session
func (c *SessionContext) ExtendWebSession(accessRequestID string) (services.WebSession, error) {
	session, err := c.clt.ExtendWebSession(c.user, c.session.GetName(), accessRequestID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return session, nil
}

// GetAgent returns agent that can be used to answer challenges
// for the web to ssh connection as well as certificate
func (c *SessionContext) GetAgent() (agent.Agent, *ssh.Certificate, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(c.session.GetPub())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	cert, ok := pub.(*ssh.Certificate)
	if !ok {
		return nil, nil, trace.BadParameter("expected certificate, got %T", pub)
	}
	if len(cert.ValidPrincipals) == 0 {
		return nil, nil, trace.BadParameter("expected at least valid principal in certificate")
	}
	privateKey, err := ssh.ParseRawPrivateKey(c.session.GetPriv())
	if err != nil {
		return nil, nil, trace.Wrap(err, "failed to parse SSH private key")
	}

	keyring := agent.NewKeyring()
	err = keyring.Add(agent.AddedKey{
		PrivateKey:  privateKey,
		Certificate: cert,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyring, cert, nil
}

// GetCertificates returns the *ssh.Certificate and *x509.Certificate
// associated with this context's session.
func (c *SessionContext) GetCertificates() (*ssh.Certificate, *x509.Certificate, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(c.session.GetPub())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	sshCert, ok := pub.(*ssh.Certificate)
	if !ok {
		return nil, nil, trace.BadParameter("not certificate")
	}
	tlsCert, err := tlsca.ParseCertificatePEM(c.session.GetTLSCert())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return sshCert, tlsCert, nil

}

// GetSessionID returns the ID of the underlying user web session.
func (c *SessionContext) GetSessionID() string {
	return c.session.GetName()
}

// Close cleans up resources associated with this context and removes it
// from the user context
func (c *SessionContext) Close() error {
	closers := c.transferClosers()
	for _, closer := range closers {
		c.log.Debugf("Closing %v.", closer)
		closer.Close()
	}
	if c.clt != nil {
		return trace.Wrap(c.clt.Close())
	}
	return nil
}

func (c *SessionContext) transferClosers() []io.Closer {
	c.mu.Lock()
	defer c.mu.Unlock()
	closers := c.closers
	c.closers = nil
	return closers
}

func (c *SessionContext) validateSession(ctx context.Context, session services.WebSession) (*SessionContext, error) {
	return c.parent.ValidateSession(ctx, session.GetUser(), session.GetName())
}

func (c *SessionContext) getToken() services.Token {
	return services.Token{
		Token:   c.session.GetBearerToken(),
		Expires: c.session.GetBearerTokenExpiryTime(),
	}
}

// expired returns whether this context has expired.
// The context is considered expired when its bearer token TTL
// is in the past
func (c *SessionContext) expired(ctx context.Context) bool {
	_, err := readSession(ctx, c.parent.accessPoint, services.GetWebSessionRequest{
		User:      c.user,
		SessionID: c.session.GetName(),
	})
	if err == nil {
		return false
	}
	c.log.WithError(err).Warn("Failed to query web session.")
	return true
}

// newSessionCache returns new instance of the session cache
func newSessionCache(config *sessionCache) (*sessionCache, error) {
	clusterName, err := proxyClient.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cache := &sessionCache{
		clusterName:  clusterName.GetClusterName(),
		proxyClient:  config.proxyClient,
		accessPoint:  config.accessPoint,
		contexts:     make(map[string]*SessionContext),
		authServers:  config.authServers,
		closer:       utils.NewCloseBroadcaster(),
		cipherSuites: config.cipherSuites,
		log:          newPackageLogger(),
		clock:        config.clock,
	}
	// periodically close expired and unused sessions
	go cache.expireSessions()
	return cache, nil
}

// sessionCache handles web session authentication,
// and holds in-memory contexts associated with each session
type sessionCache struct {
	log         logrus.FieldLogger
	proxyClient auth.ClientI
	authServers []utils.NetAddr
	accessPoint auth.ReadAccessPoint
	closer      *utils.CloseBroadcaster
	clusterName string
	// cipherSuites is the list of supported TLS cipher suites.
	cipherSuites []uint16
	clock        clockwork.Clock

	mu sync.Mutex
	// contexts maps user and session ID to an active web session
	contexts map[string]*SessionContext
}

// Close closes all allocated resources and stops goroutines
func (s *sessionCache) Close() error {
	s.log.Info("Closing session cache.")
	return s.closer.Close()
}

func (s *sessionCache) expireSessions() {
	ticker := s.clock.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
			s.clearExpiredSessions(context.TODO())
		case <-s.closer.C:
			return
		}
	}
}

func (s *sessionCache) clearExpiredSessions(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, c := range s.contexts {
		if !c.expired(ctx) {
			continue
		}
		delete(s.contexts, id)
		c.Close()
		s.log.WithField("ctx", c.String()).Info("Context expired.")
	}
}

// AuthWithOTP authenticates the specified user with the given password and OTP token.
// Returns a new web session if successful.
func (s *sessionCache) AuthWithOTP(user, pass, otpToken string) (services.WebSession, error) {
	return s.proxyClient.AuthenticateWebUser(auth.AuthenticateUserRequest{
		Username: user,
		OTP: &auth.OTPCreds{
			Password: []byte(pass),
			Token:    otpToken,
		},
	})
}

// AuthWithoutOTP authenticates the specified user with the given password.
// Returns a new web session if successful.
func (s *sessionCache) AuthWithoutOTP(user, pass string) (services.WebSession, error) {
	return s.proxyClient.AuthenticateWebUser(auth.AuthenticateUserRequest{
		Username: user,
		Pass: &auth.PassCreds{
			Password: []byte(pass),
		},
	})
}

func (s *sessionCache) GetU2FSignRequest(user, pass string) (*u2f.AuthenticateChallenge, error) {
	return s.proxyClient.GetU2FSignRequest(user, []byte(pass))
}

func (s *sessionCache) AuthWithU2FSignResponse(user string, response *u2f.AuthenticateChallengeResponse) (services.WebSession, error) {
	return s.proxyClient.AuthenticateWebUser(auth.AuthenticateUserRequest{
		Username: user,
		U2F: &auth.U2FSignResponseCreds{
			SignResponse: *response,
		},
	})
}

// GetCertificateWithoutOTP returns a new user certificate for the specified request.
func (s *sessionCache) GetCertificateWithoutOTP(c client.CreateSSHCertReq) (*auth.SSHLoginResponse, error) {
	return s.proxyClient.AuthenticateSSHUser(auth.AuthenticateSSHRequest{
		AuthenticateUserRequest: auth.AuthenticateUserRequest{
			Username: c.User,
			Pass: &auth.PassCreds{
				Password: []byte(c.Password),
			},
		},
		PublicKey:         c.PubKey,
		CompatibilityMode: c.Compatibility,
		TTL:               c.TTL,
		RouteToCluster:    c.RouteToCluster,
		KubernetesCluster: c.KubernetesCluster,
	})
}

// GetCertificateWithOTP returns a new user certificate for the specified request.
// The request is used with the given OTP token.
func (s *sessionCache) GetCertificateWithOTP(c client.CreateSSHCertReq) (*auth.SSHLoginResponse, error) {
	return s.proxyClient.AuthenticateSSHUser(auth.AuthenticateSSHRequest{
		AuthenticateUserRequest: auth.AuthenticateUserRequest{
			Username: c.User,
			OTP: &auth.OTPCreds{
				Password: []byte(c.Password),
				Token:    c.OTPToken,
			},
		},
		PublicKey:         c.PubKey,
		CompatibilityMode: c.Compatibility,
		TTL:               c.TTL,
		RouteToCluster:    c.RouteToCluster,
		KubernetesCluster: c.KubernetesCluster,
	})
}

func (s *sessionCache) GetCertificateWithU2F(c client.CreateSSHCertWithU2FReq) (*auth.SSHLoginResponse, error) {
	return s.proxyClient.AuthenticateSSHUser(auth.AuthenticateSSHRequest{
		AuthenticateUserRequest: auth.AuthenticateUserRequest{
			Username: c.User,
			U2F: &auth.U2FSignResponseCreds{
				SignResponse: c.U2FSignResponse,
			},
		},
		PublicKey:         c.PubKey,
		CompatibilityMode: c.Compatibility,
		TTL:               c.TTL,
		RouteToCluster:    c.RouteToCluster,
		KubernetesCluster: c.KubernetesCluster,
	})
}

// Ping gets basic info about the auth server.
func (s *sessionCache) Ping(ctx context.Context) (proto.PingResponse, error) {
	return s.proxyClient.Ping(ctx)
}

func (s *sessionCache) GetUserInviteU2FRegisterRequest(token string) (*u2f.RegisterChallenge, error) {
	return s.proxyClient.GetSignupU2FRegisterRequest(token)
}

func (s *sessionCache) ValidateTrustedCluster(validateRequest *auth.ValidateTrustedClusterRequest) (*auth.ValidateTrustedClusterResponse, error) {
	return s.proxyClient.ValidateTrustedCluster(validateRequest)
}

// NewSession creates a new session for the specified user/session ID
func (s *sessionCache) NewSession(user, sessionID string) (*SessionContext, error) {
	return s.newSessionContext(user, sessionID)
}

// ValidateSession validates whether the session for the given user and session ID is valid.
// It will be added it to the internal session cache if necessary
func (s *sessionCache) ValidateSession(ctx context.Context, user, sessionID string) (*SessionContext, error) {
	sessionCtx, err := s.getContext(user, sessionID)
	if err == nil {
		return sessionCtx, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	session, err := readSession(ctx, s.accessPoint, services.GetWebSessionRequest{
		User:      user,
		SessionID: sessionID,
	})
	if err != nil {
		// Session must be a valid web sesssion - otherwise bail out with an error
		return nil, trace.Wrap(err)
	}
	return s.newSessionContextFromSession(user, session)
}

func (s *sessionCache) invalidateSession(ctx *SessionContext) error {
	defer ctx.Close()
	if err := s.resetContext(ctx.GetUser(), ctx.session.GetName()); err != nil {
		return trace.Wrap(err)
	}
	clt, err := ctx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}
	err = clt.DeleteWebSession(ctx.GetUser(), ctx.GetWebSession().GetName())
	return trace.Wrap(err)
}

func (s *sessionCache) getContext(user, sessionID string) (*SessionContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, ok := s.contexts[user+sessionID]
	if ok {
		return ctx, nil
	}
	return nil, trace.NotFound("no session context for user %v and session %v",
		user, sessionID)
}

func (s *sessionCache) insertContext(user, sessionID string, ctx *SessionContext) (exists bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.contexts[user]; exists {
		return true
	}
	s.contexts[user+sessionID] = ctx
	return false
}

func (s *sessionCache) resetContext(user, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := user + sessionID
	ctx, ok := s.contexts[id]
	if !ok {
		return nil
	}
	delete(s.contexts, id)
	if err := ctx.Close(); err != nil {
		s.log.WithFields(logrus.Fields{
			logrus.ErrorKey: err,
			"ctx":           ctx.String(),
		}).Debug("Failed to close session context.")
	}
	return nil
}

func (s *sessionCache) newSessionContext(user, sessionID string) (*SessionContext, error) {
	session, err := s.proxyClient.AuthenticateWebUser(auth.AuthenticateUserRequest{
		Username: user,
		Session: &auth.SessionCreds{
			ID: sessionID,
		},
	})
	if err != nil {
		// This will fail if the session has expired and was removed
		return nil, trace.Wrap(err)
	}
	return s.newSessionContextFromSession(user, session)
}

func (s *sessionCache) newSessionContextFromSession(user string, session services.WebSession) (*SessionContext, error) {
	tlsConfig, err := s.tlsConfig(session.GetTLSCert(), session.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userClient, err := auth.NewTLSClient(auth.ClientConfig{
		Addrs: s.authServers,
		TLS:   tlsConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx := &SessionContext{
		clt:       userClient,
		remoteClt: make(map[string]auth.ClientI),
		user:      user,
		session:   session,
		parent:    s,
		log: s.log.WithFields(logrus.Fields{
			"user":    user,
			"session": session.GetShortName(),
		}),
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if exists := s.insertContext(user, session.GetName(), ctx); exists {
		// this means that someone has just inserted the context, so
		// close our extra context and return
		ctx.Close()
	}
	return ctx, nil
}

func (s *sessionCache) clientTLSConfig(cert, privKey []byte, clusterName ...string) (*tls.Config, error) {
	var certPool *x509.CertPool
	if len(clusterName) == 0 {
		certAuthorities, err := s.proxyClient.GetCertAuthorities(services.HostCA, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPool, err = services.CertPoolFromCertAuthorities(certAuthorities)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		certAuthority, err := s.proxyClient.GetCertAuthority(services.CertAuthID{
			Type:       services.HostCA,
			DomainName: clusterName[0],
		}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPool, err = services.CertPool(certAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	tlsConfig := utils.TLSConfig(s.cipherSuites)
	tlsCert, err := tls.X509KeyPair(cert, privKey)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS certificate and key")
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.RootCAs = certPool
	tlsConfig.ServerName = auth.EncodeClusterName(s.clusterName)
	return tlsConfig, nil
}

func (s *sessionCache) tlsConfig(cert, privKey []byte) (*tls.Config, error) {
	ca, err := s.proxyClient.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: s.clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certPool, err := services.CertPool(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig := utils.TLSConfig(s.cipherSuites)
	tlsCert, err := tls.X509KeyPair(cert, privKey)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS certificate and key")
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.RootCAs = certPool
	tlsConfig.ServerName = auth.EncodeClusterName(s.clusterName)
	return tlsConfig, nil
}

// waitForWebSession will block until the requested web session shows up in the
// cache or a timeout occurs.
func (h *Handler) waitForWebSession(ctx context.Context, req services.GetWebSessionRequest) error {
	_, err := h.cfg.AccessPoint.GetWebSession(ctx, req)
	if err == nil {
		return nil
	}
	// Establish a watch on the session.
	watcher, err := h.cfg.AccessPoint.NewWatcher(ctx, services.Watch{
		Name: teleport.ComponentWebProxy,
		Kinds: []services.WatchKind{
			{
				Kind:    services.KindWebSession,
				SubKind: services.KindWebSession,
				Name:    req.SessionID,
			},
		},
		MetricComponent: teleport.ComponentWebProxy,
	})
	defer watcher.Close()
	matchEvent := func(event services.Event) bool {
		// TODO(dmitri): remove comparison against sessionID if watcher accepts the ID (see above)
		return event.Type == backend.OpPut && event.Resource.GetName() == req.SessionID
	}
	_, err = waitForSession(ctx, watcher, eventMatcherFunc(matchEvent))
	return trace.Wrap(err)
}

func readBearerToken(ctx context.Context, accessPoint auth.ReadAccessPoint, req services.GetWebTokenRequest) (services.WebToken, error) {
	token, err := accessPoint.GetWebToken(ctx, req)
	if err == nil {
		return token, nil
	}
	return nil, trace.Wrap(err)
	// TODO(dmitri): wait for token to appear in cache if not found
}

func readSession(ctx context.Context, accessPoint auth.ReadAccessPoint, req services.GetWebSessionRequest) (services.WebSession, error) {
	session, err := accessPoint.GetWebSession(ctx, req)
	if err == nil {
		return session, nil
	}
	// Establish a watch on application session.
	watcher, err := accessPoint.NewWatcher(ctx, services.Watch{
		Name: teleport.ComponentWebProxy,
		Kinds: []services.WatchKind{
			{
				Kind:    services.KindWebSession,
				SubKind: services.KindWebSession,
				Name:    req.SessionID,
			},
		},
		MetricComponent: teleport.ComponentWebProxy,
	})
	defer watcher.Close()
	matchEvent := func(event services.Event) bool {
		return event.Resource.GetName() == req.SessionID
	}
	session, err = waitForSession(ctx, watcher, eventMatcherFunc(matchEvent))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return session, nil
}

func waitForSession(ctx context.Context, watcher services.Watcher, m eventMatcher) (services.WebSession, error) {
	timeout := time.NewTimer(defaults.WebHeadersTimeout)
	defer timeout.Stop()

	select {
	case event := <-watcher.Events():
		if event.Type != backend.OpInit {
			return nil, trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	case <-watcher.Done():
		// Watcher closed, probably due to a network error.
		return nil, trace.ConnectionProblem(watcher.Error(), "watcher is closed")
	case <-timeout.C:
		return nil, trace.LimitExceeded("timed out waiting for initialize event")
	}

	for {
		select {
		case event := <-watcher.Events():
			if event.Resource.GetKind() != services.KindWebSession {
				return nil, trace.BadParameter("unexpected event: %v.", event.Resource.GetKind())
			}
			if m.match(event) {
				return event.Resource.(services.WebSession), nil
			}
		case <-watcher.Done():
			// Watcher closed, probably due to a network error.
			return nil, trace.ConnectionProblem(watcher.Error(), "watcher is closed")
		case <-timeout.C:
			return nil, trace.LimitExceeded("timed out waiting for session")
		}
	}
}

func (r eventMatcherFunc) match(event services.Event) bool {
	return r(event)
}

type eventMatcherFunc func(services.Event) bool

type eventMatcher interface {
	match(services.Event) bool
}
