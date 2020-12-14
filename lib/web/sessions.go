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
	clock  clockwork.Clock

	mu sync.Mutex
	// sess refers the web session created for the user.
	// The session is updated here each time it is refreshed by the client.
	sess      services.WebSession
	remoteClt map[string]auth.ClientI
	closers   []io.Closer
}

// String returns the text representation of this context
func (c *SessionContext) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fmt.Sprintf("session(user=%v,sid=%v,bearer=%v)", c.user, c.sess.GetName(), c.sess.GetBearerToken())
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
	for i := range c.closers {
		if c.closers[i] == closer {
			c.closers = append(c.closers[:i], c.closers[i+1:]...)
			return
		}
	}
}

// Invalidate invalidates this context by removing the underlying session
// and closing all underlying closers
func (c *SessionContext) Invalidate() error {
	return c.parent.InvalidateSession(c)
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
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cn, err := clt.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if we're trying to access the local cluster, pass back the local client.
	if cn.GetClusterName() == site.GetName() {
		return clt, nil
	}

	// look to see if we already have a connection to this cluster
	remoteClt, ok := c.getRemoteClient(site.GetName())
	if !ok {
		rClt, rConn, err := c.newRemoteClient(site)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// add a closer for the underlying connection
		if rConn != nil {
			c.AddClosers(rConn)
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
func (c *SessionContext) newRemoteClient(cluster reversetunnel.RemoteSite) (auth.ClientI, net.Conn, error) {
	clt, err := c.tryRemoteTLSClient(cluster)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return clt, nil, nil
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
	tlsCert, err := tls.X509KeyPair(c.sess.GetTLSCert(), c.sess.GetPriv())
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
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sess
}

// ExtendWebSession creates a new web session for this user
// based on the previous session
func (c *SessionContext) ExtendWebSession(accessRequestID string) (services.WebSession, error) {
	sess, err := c.clt.ExtendWebSession(c.user, c.sess.GetName(), accessRequestID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

// GetAgent returns agent that can be used to answer challenges
// for the web to ssh connection as well as certificate
func (c *SessionContext) GetAgent() (agent.Agent, *ssh.Certificate, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(c.sess.GetPub())
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
	privateKey, err := ssh.ParseRawPrivateKey(c.sess.GetPriv())
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
// associated with this session.
func (c *SessionContext) GetCertificates() (*ssh.Certificate, *x509.Certificate, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(c.sess.GetPub())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	sshcert, ok := pub.(*ssh.Certificate)
	if !ok {
		return nil, nil, trace.BadParameter("not certificate")
	}
	tlscert, err := tlsca.ParseCertificatePEM(c.sess.GetTLSCert())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return sshcert, tlscert, nil
}

// Close cleans up connections associated with requests
func (c *SessionContext) Close() error {
	c.mu.Lock()
	closers := c.closers
	c.closers = nil
	c.mu.Unlock()
	var errors []error
	for _, closer := range closers {
		c.log.Debugf("Closing %v.", closer)
		if err := closer.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if c.clt != nil {
		if err := c.clt.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// expired returns whether this context has expired.
// The context is considered expired when its bearer token TTL
// is in the past
func (c *SessionContext) expired(ctx context.Context) bool {
	session, err := c.readSession(ctx)
	if err != nil {
		c.log.WithError(err).Warn("Failed to query web session.")
		return false
	}
	return c.clock.Now().After(session.GetBearerTokenExpiryTime())
}

func (c *SessionContext) readSession(ctx context.Context) (services.WebSession, error) {
	c.mu.Lock()
	sessionID := c.sess.GetName()
	c.mu.Unlock()
	sess, err := c.parent.accessPoint.GetWebSession(ctx, services.GetWebSessionRequest{SessionID: sessionID})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

func (c *SessionContext) setSession(session services.WebSession) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sess = session
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
// and holds in memory contexts associated with each session
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
	// contexts maps user/sid to an existing session context
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

func (s *sessionCache) AuthWithOTP(user, pass string, otpToken string) (services.WebSession, error) {
	return s.proxyClient.AuthenticateWebUser(auth.AuthenticateUserRequest{
		Username: user,
		OTP: &auth.OTPCreds{
			Password: []byte(pass),
			Token:    otpToken,
		},
	})
}

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
func (s *sessionCache) NewSession(user, sid string) (*SessionContext, error) {
	return s.newSessionContext(user, sid)
}

// ValidateSession returns the existing session context for the specified user/session ID.
func (s *sessionCache) ValidateSession(ctx context.Context, user, sid string) (*SessionContext, error) {
	sessionCtx, err := s.getContext(user, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := sessionCtx.readSession(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionCtx.setSession(session)
	return sessionCtx, nil
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
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, ok := s.contexts[user+sid]
	if ok {
		return ctx, nil
	}
	return nil, trace.NotFound("session context not found")
}

func (s *sessionCache) insertContext(user, sid string, ctx *SessionContext) (exists bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.contexts[user+sid]; exists {
		return true
	}
	s.contexts[user+sid] = ctx
	return false
}

func (s *sessionCache) resetContext(user, sid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := user + sid
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

func (s *sessionCache) newSessionContext(user, sid string) (*SessionContext, error) {
	sess, err := s.proxyClient.AuthenticateWebUser(auth.AuthenticateUserRequest{
		Username: user,
		Session: &auth.SessionCreds{
			ID: sid,
		},
	})
	if err != nil {
		// This will fail if the session has expired and was removed
		return nil, trace.Wrap(err)
	}
	tlsConfig, err := s.tlsConfig(sess.GetTLSCert(), sess.GetPriv())
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
	c := &SessionContext{
		clt:       userClient,
		remoteClt: make(map[string]auth.ClientI),
		user:      user,
		sess:      sess,
		parent:    s,
		log: s.log.WithFields(logrus.Fields{
			"user": user,
			"sess": sess.GetShortName(),
		}),
		clock: s.clock,
	}
	if exists := s.insertContext(user, sid, c); exists {
		// this means that someone has just inserted the context, so
		// close our extra context and return
		c.Close()
	}
	return c, nil
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
func (h *Handler) waitForWebSession(ctx context.Context, sessionID string) error {
	// Establish a watch on application session.
	watcher, err := h.cfg.AccessPoint.NewWatcher(ctx, services.Watch{
		Name: teleport.ComponentWebProxy,
		Kinds: []services.WatchKind{
			{
				Kind:    services.KindWebSession,
				SubKind: services.KindWebSession,
			},
		},
		MetricComponent: teleport.ComponentWebProxy,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	sessionProber := func() error {
		_, err = h.cfg.AccessPoint.GetWebSession(ctx, services.GetWebSessionRequest{
			SessionID: sessionID,
		})
		return trace.Wrap(err)
	}
	matcher := func(event services.Event) bool {
		return event.Type == backend.OpPut && event.Resource.GetName() == sessionID
	}
	return waitForSession(ctx, watcher, sessionProber, matcher)
}

type sessionProberFunc func() error
type eventMatcherFunc func(services.Event) bool

func waitForSession(ctx context.Context, watcher services.Watcher, sessionProber sessionProberFunc, eventMatcher eventMatcherFunc) error {
	timeout := time.NewTimer(defaults.WebHeadersTimeout)
	defer timeout.Stop()

	select {
	// Received an event, first event should always be an initialize event.
	case event := <-watcher.Events():
		if event.Type != backend.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	// Watcher closed, probably due to a network error.
	case <-watcher.Done():
		return trace.ConnectionProblem(watcher.Error(), "watcher is closed")
	// Timed out waiting for initialize event.
	case <-timeout.C:
		return trace.BadParameter("timed out waiting for initialize event")
	}

	// Check if the session exists in the backend.
	err := sessionProber()
	if err == nil {
		return nil
	}

	for {
		select {
		// If the event is the expected one, return right away.
		case event := <-watcher.Events():
			if event.Resource.GetKind() != services.KindWebSession {
				return trace.BadParameter("unexpected event: %v.", event.Resource.GetKind())
			}
			if eventMatcher(event) {
				return nil
			}
		// Watcher closed, probably due to a network error.
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed")
		// Timed out waiting for initialize event.
		case <-timeout.C:
			return trace.BadParameter("timed out waiting for session")
		}
	}
}
