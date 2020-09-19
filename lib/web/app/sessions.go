/*
Copyright 2020 Gravitational, Inc.

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

package app

import (
	"context"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"

	"github.com/sirupsen/logrus"
)

type session struct {
	clock clockwork.Clock

	cache *sessionCache

	// cacheKey is the encoded session cookie. It is used as a key for the cache.
	cacheKey string

	url *url.URL

	//app      services.Server
	//checker  services.AccessChecker
	identity *tlsca.Identity

	fwd  *forward.Forwarder
	conn net.Conn

	jwt string
}

type sessionCacheConfig struct {
	Clock       clockwork.Clock
	AuthClient  auth.ClientI
	ProxyClient reversetunnel.Server
}

func (c *sessionCacheConfig) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.AuthClient == nil {
		return trace.BadParameter("auth client missing")
	}
	if c.ProxyClient == nil {
		return trace.BadParameter("proxy client missing")
	}
	return nil
}

type sessionCache struct {
	c   *sessionCacheConfig
	log *logrus.Entry

	mu    sync.Mutex
	cache *ttlmap.TTLMap
}

func newSessionCache(config *sessionCacheConfig) (*sessionCache, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cache, err := ttlmap.New(defaults.ClientCacheSize, ttlmap.CallOnExpire(closeSession))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &sessionCache{
		c: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentAppProxyCache,
		}),
		cache: cache,
	}, nil
}

func (s *sessionCache) get(ctx context.Context, cookieValue string) (*session, error) {
	// Always look for the existence of a session directly in the backend. This
	// is to ensure that a user can for logout of all sessions by logging out of the Web UI.
	cookie, err := decodeCookie(cookieValue)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appSession, err := s.c.AuthClient.GetAppSession(ctx, services.GetAppSessionRequest{
		Username:   cookie.Username,
		ParentHash: cookie.ParentHash,
		SessionID:  cookie.SessionID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the session exists in the backend, check if this proxy has locally
	// cached metadata about the session. If it does, return it, otherwise
	// build it and return it.
	session, err := s.cacheGet(cookieValue)
	if err == nil {
		return session, nil
	}
	if !trace.IsNotFound(err) {
		s.log.Debugf("Failed to find session in cache: %v.", err)
	}

	// Construct session metadata and put it in the cache.
	sess, err := s.newSession(ctx, cookieValue, appSession)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ttl := appSession.GetExpiryTime().Sub(s.c.Clock.Now())
	if err := s.cacheSet(cookieValue, sess, ttl); err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

// GetApp looks for an application registered for the requested public address
// in the cluster and returns it. In the situation multiple applications match,
// a random selection is returned. This is done on purpose to support HA to
// allow multiple application proxy nodes to be run and if one is down, at
// least the application can be accessible on the other.
func (s *sessionCache) GetApp(ctx context.Context, publicAddr string, clusterName string) (*services.App, services.Server, error) {
	var appMatch []*services.App
	var serverMatch []services.Server

	proxyClient, err := s.c.ProxyClient.GetSite(clusterName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	authClient, err := proxyClient.CachingAccessPoint()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	servers, err := authClient.GetApps(ctx, defaults.Namespace)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	for _, server := range servers {
		for _, a := range server.GetApps() {
			if a.PublicAddr == publicAddr {
				appMatch = append(appMatch, a)
				serverMatch = append(serverMatch, server)
			}
		}
	}

	if len(appMatch) == 0 {
		return nil, nil, trace.NotFound("%q not found in %q", publicAddr, clusterName)
	}
	index := rand.Intn(len(appMatch))
	return appMatch[index], serverMatch[index], nil
}

func (s *sessionCache) newSession(ctx context.Context, cookieValue string, sess services.WebSession) (*session, error) {
	// Get the application this session is targeting.
	app, server, err := s.GetApp(ctx, sess.GetPublicAddr(), sess.GetClusterName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(russjones): This should be parsed and always be a URI in file configuration.
	//addr, err := parseAddress(app.URI)
	//if err != nil {
	//	return nil, trace.Wrap(err)
	//}
	u, err := url.Parse(app.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract roles, traits, and identity of the user from the certificate.
	cert, err := utils.ParseCertificatePEM(sess.GetTLSCert())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles, _, err := services.ExtractFromIdentity(s.c.AuthClient, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate a signed token that can be re-used during the lifetime of this
	// session to pass authentication information to the target application.
	jwt, err := s.c.AuthClient.GenerateAppToken(ctx, services.AppTokenParams{
		Username: sess.GetUser(),
		Roles:    roles,
		AppName:  app.Name,
		Expires:  sess.GetExpiryTime(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a connection through the reverse tunnel to the target application.
	clusterClient, err := s.c.ProxyClient.GetSite(sess.GetClusterName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	//to, err := utils.ParseAddr(u.Host)
	//if err != nil {
	//	return nil, trace.Wrap(err)
	//}
	conn, err := clusterClient.Dial(reversetunnel.DialParams{
		//To:          to,
		ServerID:    strings.Join([]string{server.GetName(), sess.GetClusterName()}, "."),
		PublicAddr:  sess.GetPublicAddr(),
		Certificate: sess.GetTLSCert(),
		ConnType:    services.AppTunnel,
	})
	if err != nil {
		s.log.Warnf("Failed to establish connection to %q through reverse tunnel: %v.", sess.GetPublicAddr(), err)
		return nil, trace.BadParameter("application not available")
	}

	// Create a HTTP request forwarder that will be used to forward the actual
	// request over the reverse tunnel to the target application.
	fwdHandler := &forwardHandler{
		conn:     conn,
		jwt:      jwt,
		cache:    s,
		cacheKey: cookieValue,
	}
	fwd, err := forward.New(
		forward.RoundTripper(fwdHandler),
		forward.Rewriter(fwdHandler),
		forward.ErrorHandler(fwdHandler),
		forward.Logger(s.log))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &session{
		cache:    s,
		cacheKey: cookieValue,
		url:      u,
		identity: identity,
		conn:     conn,
		fwd:      fwd,
		jwt:      jwt,
	}, nil
}

func (s *sessionCache) cacheGet(key string) (*session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.cache.Get(key); ok {
		if se, sok := sess.(*session); sok {
			return se, nil
		}
		return nil, trace.BadParameter("invalid type stored in session cache: %T", sess)
	}
	return nil, trace.NotFound("session not found")
}

func (s *sessionCache) cacheSet(key string, value *session, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cache.Set(key, value, ttl); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *sessionCache) cacheRemove(key string) {
	sess, err := s.cacheGet(key)
	if err != nil {
		s.log.Debugf("Failed to remove item from cache: %v", err)
		return
	}
	if err := sess.conn.Close(); err != nil {
		s.log.Debugf("Failed to close connection: %v.", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.Remove(key)
}

// errorHandler is called when the forwarder is unable to forward the request.
// Removes the session from the cache to force the proxy to re-dial to the
// application.
func (s *session) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	s.cache.log.Debugf("Request to %v failed: %v, removing connection from cache.", r.URL.Path, err)
	s.cache.cacheRemove(s.cacheKey)

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

//func parseAddress(addr string) (string, error) {
//	u, err := url.Parse(addr)
//	if err != nil {
//		u, err = url.Parse("http://" + addr)
//		if err != nil {
//			return "", trace.Wrap(err)
//		}
//		return u.String(), nil
//	}
//	return u.String(), nil
//}

func closeSession(key string, val interface{}) {
	if sess, ok := val.(*session); ok {
		if err := sess.conn.Close(); err != nil {
			logrus.Debugf("Failed to close connection: %v.", err)
		}
	}
}

type forwardHandler struct {
	conn     net.Conn
	jwt      string
	cacheKey string
	cache    *sessionCache
}

func (f *forwardHandler) RoundTrip(r *http.Request) (*http.Response, error) {
	tr := &http.Transport{
		DialContext:           f.dialContext,
		ResponseHeaderTimeout: defaults.DefaultDialTimeout,
		MaxIdleConns:          defaults.HTTPMaxIdleConns,
		MaxIdleConnsPerHost:   defaults.HTTPMaxIdleConnsPerHost,
		MaxConnsPerHost:       defaults.HTTPMaxConnsPerHost,
		IdleConnTimeout:       defaults.HTTPIdleTimeout,
	}
	resp, err := tr.RoundTrip(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (f *forwardHandler) dialContext(ctx context.Context, network string, addr string) (net.Conn, error) {
	return f.conn, nil
}

func (f *forwardHandler) Rewrite(r *http.Request) {
	// Add in JWT headers.
	r.Header.Add("x-teleport-jwt-assertion", f.jwt)
	r.Header.Add("Cf-access-token", f.jwt)

	// Remove the application specific session cookie from the header. This is
	// done by first wiping out the "Cookie" header then adding back all cookies
	// except the Teleport application specific session cookie. This appears to
	// be the best way to serialize cookies.
	r.Header.Del("Cookie")
	for _, cookie := range r.Cookies() {
		if cookie.Name == cookieName {
			continue
		}
		r.AddCookie(cookie)
	}
}

// ServeHTTP is called upon errors when forwarding the request.
func (f *forwardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, err error) {
	f.cache.log.Debugf("Request to %v failed: %v, removing connection from cache.", r.URL.Path, err)
	f.cache.cacheRemove(f.cacheKey)

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
