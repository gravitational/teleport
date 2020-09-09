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
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
)

type session struct {
	// cacheKey is the encoded session cookie. It is used as a key for the cache.
	cacheKey string

	app      services.Server
	checker  services.AccessChecker
	identity *tlsca.Identity
	fwd      *forward.Forwarder

	// TODO(russjones): The JWT token does not need to be stored since it's
	// embedded within the forwarder?
	//token       string
}

type sessionCacheConfig struct {
	AuthClient  auth.ClientI
	ProxyClient reversetunnel.Server
}

func (c sessionCacheConfig) Check() error {
	if c.AuthClient == nil {
		return trace.BadParameter("auth client missing")
	}
	if c.ProxyClient == nil {
		return trace.BadParameter("proxy client missing")
	}
	return nil
}

type sessionCache struct {
	c   sessionCacheConfig
	log *logrus.Entry

	cache *ttlmap.TTLMap
}

func newSessionCache(config sessionCacheConfig) (*sessionCache, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	cache, err := ttlmap.New(defaults.ClientCacheSize)
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
	// Always look for the existence of a session directly in the backend. The
	// lookup can occur in the backend cache, but lookup should occur directly
	// against the backend and not a process local cache. This is to ensure that
	// a user can for logout of all sessions by logging out of the Web UI.
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
	if sess, ok := s.cache.Get(cookieValue); ok {
		if se, sok := sess.(*session); sok {
			return se, nil
		}
	}

	// TODO(russjones): Look at session.Expiry() here.
	// Construct session metadata and put it in the cache.
	sess, err := s.newSession(ctx, cookieValue, appSession)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.cache.Set(cookieValue, sess, 10*time.Minute); err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

func (s *sessionCache) newSession(ctx context.Context, cookieValue string, sess services.WebSession) (*session, error) {
	// Get the application this session is targeting.
	app, err := s.c.AuthClient.GetApp(ctx, defaults.Namespace, sess.GetAppName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract the identity of the user from the certificate.
	cert, err := utils.ParseCertificatePEM(sess.GetTLSCert())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use roles and traits to construct and access checker.
	roles, traits, err := services.ExtractFromIdentity(s.c.AuthClient, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(roles, s.c.AuthClient, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate a signed token that can be re-used during the lifetime of this
	// session to pass authentication information to the target application.
	token, err := s.c.AuthClient.GenerateAppToken(ctx, services.AppTokenParams{
		Username: sess.GetUser(),
		Roles:    roles,
		// TODO(russjones): Expiry implies a time.Time, instead Expiry here is a
		// time.Duration. Fix it so it can be directly set like:
		// "Expiry: session.GetExpiryTime()".
		Expiry: 10 * time.Minute,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(russjones): Fill out the reversetunnel.DialParams after #4290 is merged in.
	// Get a connection through the reverse tunnel to the target application.
	clusterClient, err := s.c.ProxyClient.GetSite(sess.GetClusterName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := clusterClient.Dial(reversetunnel.DialParams{
		ServerID: strings.Join([]string{app.GetName(), sess.GetClusterName()}, "."),
		ConnType: services.AppTunnel,
	})
	if err != nil {
		s.log.Warnf("Failed to establish connection to %q through reverse tunnel: %v.", sess.GetAppName(), err)
		return nil, trace.BadParameter("application not available")
	}

	// Create a HTTP request forwarder that will be used to forward the actual
	// request over the reverse tunnel to the target application.
	fwd, err := forward.New(
		forward.RoundTripper(newCustomTransport(conn)),
		forward.Rewriter(&rewriter{signedToken: token}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &session{
		cacheKey: cookieValue,
		app:      app,
		checker:  checker,
		identity: identity,
		fwd:      fwd,
	}, nil
}

func (s *sessionCache) remove(cookieValue string) {
	s.cache.Remove(cookieValue)
}

// TODO(russjones): Strip Teleport cookies.
type rewriter struct {
	signedToken string
}

func (r *rewriter) Rewrite(req *http.Request) {
	req.Header.Add("x-teleport-jwt-assertion", r.signedToken)
	req.Header.Add("Cf-access-token", r.signedToken)

	// Wipe out any existing cookies and skip over any Teleport ones.
	req.Header.Del("Cookie")
	for _, cookie := range req.Cookies() {
		if cookie.Name == "session" {
			continue
		}
		req.AddCookie(cookie)
	}
}

type customTransport struct {
	conn net.Conn
}

func newCustomTransport(conn net.Conn) *customTransport {
	return &customTransport{
		conn: conn,
	}
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr := &http.Transport{
		Dial: func(network string, addr string) (net.Conn, error) {
			return t.conn, nil
		},
	}

	resp, err := tr.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
