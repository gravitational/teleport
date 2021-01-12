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
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/tstranex/u2f"
)

// SessionContext is a context associated with users'
// web session, it stores connected client that persists
// between requests for example to avoid connecting
// to the auth server on every page hit
type SessionContext struct {
	sync.Mutex
	logrus.FieldLogger
	sess      services.WebSession
	user      string
	clt       auth.ClientI
	remoteClt map[string]auth.ClientI
	parent    *sessionCache
	closers   []io.Closer
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

func (c *SessionContext) addRemoteClient(siteName string, remoteClient auth.ClientI) {
	c.Lock()
	defer c.Unlock()

	c.remoteClt[siteName] = remoteClient
}

func (c *SessionContext) getRemoteClient(siteName string) (auth.ClientI, bool) {
	c.Lock()
	defer c.Unlock()

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
	closers := c.TransferClosers()
	for _, closer := range closers {
		c.Debugf("Closing %v.", closer)
		closer.Close()
	}
	if c.clt != nil {
		return trace.Wrap(c.clt.Close())
	}
	return nil
}

// newSessionCache returns new instance of the session cache
func newSessionCache(config *sessionCache) (*sessionCache, error) {
	clusterName, err := config.proxyClient.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.clock == nil {
		config.clock = clockwork.NewRealClock()
	}
	m, err := ttlmap.New(1024, ttlmap.CallOnExpire(closeContext), ttlmap.Clock(config.clock))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cache := &sessionCache{
		clusterName:  clusterName.GetClusterName(),
		proxyClient:  config.proxyClient,
		contexts:     m,
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
	log logrus.FieldLogger
	sync.Mutex
	proxyClient auth.ClientI
	contexts    *ttlmap.TTLMap
	authServers []utils.NetAddr
	closer      *utils.CloseBroadcaster
	clusterName string
	clock       clockwork.Clock

	// cipherSuites is the list of supported TLS cipher suites.
	cipherSuites []uint16
}

// Close closes all allocated resources and stops goroutines
func (s *sessionCache) Close() error {
	s.log.Info("Closing session cache.")
	return s.closer.Close()
}

// closeContext is called when session context expires from
// cache and will clean up connections
func closeContext(key string, val interface{}) {
	go func() {
		log.Infof("Closing context %v.", key)
		ctx, ok := val.(*SessionContext)
		if !ok {
			log.Warnf("Invalid value type %T.", val)
			return
		}
		if err := ctx.Close(); err != nil {
			log.Warnf("Failed to close context: %v.", err)
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
		log.Infof("Removed %v expired sessions.", expired)
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

func (s *sessionCache) GetU2FSignRequest(user, pass string) (*u2f.SignRequest, error) {
	return s.proxyClient.GetU2FSignRequest(user, []byte(pass))
}

func (s *sessionCache) AuthWithU2FSignResponse(user string, response *u2f.SignResponse) (services.WebSession, error) {
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

func (s *sessionCache) GetUserInviteU2FRegisterRequest(token string) (*u2f.RegisterRequest, error) {
	return s.proxyClient.GetSignupU2FRegisterRequest(token)
}

func (s *sessionCache) ValidateTrustedCluster(validateRequest *auth.ValidateTrustedClusterRequest) (*auth.ValidateTrustedClusterResponse, error) {
	return s.proxyClient.ValidateTrustedCluster(validateRequest)
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

	sess, err := s.proxyClient.AuthenticateWebUser(auth.AuthenticateUserRequest{
		Username: user,
		Session: &auth.SessionCreds{
			ID: sid,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
	tlsCert, err := tls.X509KeyPair(sess.GetTLSCert(), sess.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS cert and key")
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.RootCAs = certPool
	tlsConfig.ServerName = auth.EncodeClusterName(s.clusterName)
	tlsConfig.Time = s.clock.Now

	userClient, err := auth.NewClient(apiclient.Config{
		Addrs: utils.NetAddrsToStrings(s.authServers),
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
		FieldLogger: log.WithFields(logrus.Fields{
			"user": user,
			"sess": sess.GetShortName(),
		}),
	}

	ttl := utils.ToTTL(s.clock, sess.GetBearerTokenExpiryTime())
	out, err := s.insertContext(user, sid, c, ttl)
	if err != nil {
		// this means that someone has just inserted the context, so
		// close our extra context and return
		if trace.IsAlreadyExists(err) {
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
