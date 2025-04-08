/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package app

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

// session holds a request forwarder and web session for this request.
type session struct {
	// fwd can rewrite and forward requests to the target application.
	fwd *reverseproxy.Forwarder
	// ws represents the services.WebSession this requests belongs to.
	ws types.WebSession
	// transport allows to dial an application server.
	tr *transport
}

// newSession creates a new session.
func (h *Handler) newSession(ctx context.Context, ws types.WebSession) (*session, error) {
	// Extract the identity of the user.
	certificate, err := tlsca.ParseCertificatePEM(ws.GetTLSCert())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(certificate.Subject, certificate.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Query the cluster this application is running in to find the public
	// address and cluster name pair which will be encoded into the certificate.
	clusterClient, err := h.c.ProxyClient.GetSite(identity.RouteToApp.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessPoint, err := clusterClient.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := Match(
		ctx,
		accessPoint,
		appServerMatcher(h.c.ProxyClient, identity.RouteToApp.PublicAddr, identity.RouteToApp.ClusterName),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(servers) == 0 {
		return nil, trace.NotFound("failed to match applications")
	}

	// Create a rewriting transport that will be used to forward requests.
	transport, err := newTransport(&transportConfig{
		log:                   h.log,
		clock:                 h.c.Clock,
		proxyClient:           h.c.ProxyClient,
		accessPoint:           h.c.AccessPoint,
		cipherSuites:          h.c.CipherSuites,
		identity:              identity,
		servers:               servers,
		ws:                    ws,
		clusterName:           h.clusterName,
		integrationAppHandler: h.c.IntegrationAppHandler,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Don't trust any "X-Forward-*" headers the client sends, instead set our own.
	delegate := reverseproxy.NewHeaderRewriter()
	delegate.TrustForwardHeader = false
	hr := common.NewHeaderRewriter(delegate)

	// Create a forwarder that will be used to forward requests.
	fwd, err := reverseproxy.New(
		reverseproxy.WithPassHostHeader(),
		reverseproxy.WithFlushInterval(100*time.Millisecond),
		reverseproxy.WithRoundTripper(transport),
		reverseproxy.WithLogger(h.log),
		reverseproxy.WithErrorHandler(h.handleForwardError),
		reverseproxy.WithRewriter(hr),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &session{
		fwd: fwd,
		ws:  ws,
		tr:  transport,
	}, nil
}

// appServerMatcher returns a Matcher function used to find which AppServer can
// handle the application requests.
func appServerMatcher(proxyClient reversetunnelclient.Tunnel, publicAddr string, clusterName string) Matcher {
	// Match healthy and PublicAddr servers. Having a list of only healthy
	// servers helps the transport fail before the request is forwarded to a
	// server (in cases where there are no healthy servers). This process might
	// take an additional time to execute, but since it is cached, only a few
	// requests need to perform it.
	return MatchAll(
		MatchPublicAddr(publicAddr),
		// NOTE: Try to leave this matcher as the last one to dial only the
		// application servers that match the requested application.
		MatchHealthy(proxyClient, clusterName),
	)
}

// sessionCache holds a cache of sessions that are used to forward requests.
type sessionCache struct {
	mu    sync.Mutex
	cache *ttlmap.TTLMap

	closeContext context.Context

	log *logrus.Entry
}

// newSessionCache creates a new session cache.
func newSessionCache(ctx context.Context, log *logrus.Entry) (*sessionCache, error) {
	var err error

	s := &sessionCache{
		closeContext: ctx,
		log:          log,
	}

	// Cache of request forwarders. Set an expire function that can be used to
	// close any open resources.
	s.cache, err = ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go s.expireSessions()

	return s, nil
}

// cacheGet will fetch the forwarder from the cache.
func (s *sessionCache) get(key string) (*session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if f, ok := s.cache.Get(key); ok {
		if fwd, fok := f.(*session); fok {
			return fwd, nil
		}
		return nil, trace.BadParameter("invalid type stored in cache: %T", f)
	}
	return nil, trace.NotFound("forwarder not found")
}

// cacheSet will add the forwarder to the cache.
func (s *sessionCache) set(key string, value *session, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cache.Set(key, value, ttl); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// remove immediately removes a single session from the cache.
func (s *sessionCache) remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, _ = s.cache.Remove(key)
}

// expireSessions ticks every second trying to close expired sessions.
func (s *sessionCache) expireSessions() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.expireSession()
		case <-s.closeContext.Done():
			return
		}
	}
}

// expiredSession tries to expire sessions in the cache.
func (s *sessionCache) expireSession() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.RemoveExpired(10)
}
