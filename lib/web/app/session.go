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
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"

	"github.com/gravitational/oxy/forward"
	oxyutils "github.com/gravitational/oxy/utils"
	"github.com/sirupsen/logrus"
)

// session holds a request forwarder and web session for this request.
type session struct {
	// fwd can rewrite and forward requests to the target application.
	fwd *forward.Forwarder
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

	// Match healthy and PublicAddr servers. Having a list of only healthy
	// servers helps the transport fail before the request is forwarded to a
	// server (in cases where there are no healthy servers). This process might
	// take an additional time to execute, but since it is cached, only a few
	// requests need to perform it.
	servers, err := Match(ctx, accessPoint, MatchAll(
		MatchPublicAddr(identity.RouteToApp.PublicAddr),
		// NOTE: Try to leave this matcher as the last one to dial only the
		// application servers that match the requested application.
		MatchHealthy(h.c.ProxyClient, identity),
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(servers) == 0 {
		return nil, trace.NotFound("failed to match applications")
	}

	// Create a rewriting transport that will be used to forward requests.
	transport, err := newTransport(&transportConfig{
		log:          h.log,
		proxyClient:  h.c.ProxyClient,
		accessPoint:  h.c.AccessPoint,
		cipherSuites: h.c.CipherSuites,
		identity:     identity,
		servers:      servers,
		ws:           ws,
		clusterName:  h.clusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fwd, err := forward.New(
		forward.FlushInterval(100*time.Millisecond),
		forward.RoundTripper(transport),
		forward.Logger(h.log),
		forward.PassHostHeader(true),
		forward.WebsocketDial(transport.DialWebsocket),
		forward.ErrorHandler(oxyutils.ErrorHandlerFunc(h.handleForwardError)),
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
