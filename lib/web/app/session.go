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
	"github.com/sirupsen/logrus"
)

// session holds a request forwarder and web session for this request.
type session struct {
	// fwd can rewrite and forward requests to the target application.
	fwd *forward.Forwarder
	// ws represents the services.WebSession this requests belongs to.
	ws types.WebSession
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
	application, server, err := Match(ctx, accessPoint, MatchPublicAddr(identity.RouteToApp.PublicAddr))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a rewriting transport that will be used to forward requests.
	transport, err := newTransport(&transportConfig{
		proxyClient:  h.c.ProxyClient,
		accessPoint:  h.c.AccessPoint,
		cipherSuites: h.c.CipherSuites,
		identity:     identity,
		server:       server,
		app:          application,
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
		forward.WebsocketDial(transport.dialer),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &session{
		fwd: fwd,
		ws:  ws,
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
