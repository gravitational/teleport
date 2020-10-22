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
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
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
	ws services.WebSession
	// accessPoint is a cached connection to auth.
	accessPoint auth.AccessPoint
}

// newSession creates a new session.
func (h *Handler) newSession(ctx context.Context, ws services.WebSession) (*session, error) {
	// Extract the identity of the user.
	certificate, err := tlsca.ParseCertificatePEM(ws.GetTLSCert())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(certificate.Subject, certificate.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Find the address of the Teleport application proxy server for this public
	// address and cluster name pair.
	clusterClient, err := h.c.ProxyClient.GetSite(identity.RouteToApp.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessPoint, err := clusterClient.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	application, server, err := getApp(ctx, accessPoint, identity.RouteToApp.PublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cn, err := h.c.AccessPoint.GetClusterName()
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
		clusterName:  cn.GetClusterName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fwd, err := forward.New(
		forward.RoundTripper(transport),
		forward.Logger(h.log))
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
	s.cache, err = ttlmap.New(defaults.ClientCacheSize, ttlmap.CallOnExpire(s.expire))
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

// expire will close the stream writer.
func (s *sessionCache) expire(key string, el interface{}) {
	session, ok := el.(*session)
	if !ok {
		s.log.Debugf("Invalid type stored in cache: %T.", el)
		return
	}

	if err := session.accessPoint.Close(); err != nil {
		s.log.Debugf("Failed to close stream writer: %v.", err)
	}
}

// expireSessions ticks every second trying to close expire sessions.
func (s *sessionCache) expireSessions() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.expiredSession()
		case <-s.closeContext.Done():
			return
		}
	}
}

// expiredSession tries to expire sessions in the cache.
func (s *sessionCache) expiredSession() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.RemoveExpired(10)
}

// getApp looks for an application registered for the requested public address
// in the cluster and returns it. In the situation multiple applications match,
// a random selection is returned. This is done on purpose to support HA to
// allow multiple application proxy nodes to be run and if one is down, at
// least the application can be accessible on the other.
//
// In the future this function should be updated to keep state on application
// servers that are down and to not route requests to that server.
func getApp(ctx context.Context, accessPoint auth.AccessPoint, publicAddr string) (*services.App, services.Server, error) {
	var am []*services.App
	var sm []services.Server

	servers, err := accessPoint.GetAppServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	for _, server := range servers {
		for _, app := range server.GetApps() {
			if app.PublicAddr == publicAddr {
				am = append(am, app)
				sm = append(sm, server)
			}
		}
	}

	if len(am) == 0 {
		return nil, nil, trace.NotFound("%q not found", publicAddr)
	}
	index := rand.Intn(len(am))
	return am[index], sm[index], nil
}
