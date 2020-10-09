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
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

type session struct {
	fwd *forward.Forwarder
}

// getSession returns a request session used to proxy the request to the
// target application. Always checks if the session is valid first and if so,
// will return a cached session, otherwise will create one.
func (s *Server) getSession(ctx context.Context, appSession services.AppSession) (*session, error) {
	// If a cached forwarder exists, return it right away.
	session, err := s.cacheGet(appSession.GetName())
	if err == nil {
		return session, nil
	}

	// Create a new session with a recorder and forwarder in it.
	session, err = s.newSession(ctx, appSession)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Put the session in the cache so the next request can use it.
	// TODO(russjones): Make this smaller of now+5 mins or expiry time.
	//err = s.cacheSet(session.GetName(), session, session.Expiry().Sub(s.c.Clock.Now()))
	err = s.cacheSet(appSession.GetName(), session, 2*time.Second)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

func (s *Server) newSession(ctx context.Context, appSession services.AppSession) (*session, error) {
	// Locally lookup the application the caller is targeting.
	app, err := s.getApp(ctx, appSession.GetPublicAddr())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the forwarder.
	fwder, err := newForwarder(s.closeContext,
		&forwarderConfig{
			publicAddr: app.PublicAddr,
			uri:        app.URI,
			jwt:        appSession.GetJWT(),
			tr:         s.tr,
			log:        s.log,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fwd, err := forward.New(
		forward.RoundTripper(fwder),
		forward.Rewriter(fwder),
		forward.Logger(s.log))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &session{
		fwd: fwd,
	}, nil
}

// getApp returns an application matching the public address. If multiple
// matching applications exist, the first one is returned. Random selection
// (or round robin) does not need to occur here because they will all point
// to the same target address. Random selection (or round robin) occurs at the
// proxy to load balance requests to the application service.
func (s *Server) getApp(ctx context.Context, publicAddr string) (*services.App, error) {
	for _, a := range s.c.Server.GetApps() {
		if publicAddr == a.PublicAddr {
			return a, nil
		}
	}

	return nil, trace.NotFound("no application at %v found", publicAddr)
}

// cacheGet will fetch the forwarder from the cache.
func (s *Server) cacheGet(key string) (*session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if f, ok := s.cache.Get(key); ok {
		if fwd, fok := f.(*session); fok {
			return fwd, nil
		}
		return nil, trace.BadParameter("invalid type stored in cache: %T", f)
	}
	return nil, trace.NotFound("session not found")
}

// cacheSet will add the forwarder to the cache.
func (s *Server) cacheSet(key string, value *session, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cache.Set(key, value, ttl); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cacheExpire will close the stream writer.
func (s *Server) cacheExpire(key string, el interface{}) {
	session, ok := el.(*session)
	if !ok {
		s.log.Debugf("Invalid type stored in cache: %T.", el)
		return
	}

	s.log.Debugf("Closing expired stream %v.", key)
}

// forwarderConfig is the configuration for a forwarder.
type forwarderConfig struct {
	publicAddr string
	uri        string
	jwt        string
	tr         http.RoundTripper
	log        *logrus.Entry
}

// Check will valid the configuration of a forwarder.
func (c *forwarderConfig) Check() error {
	if c.jwt == "" {
		return trace.BadParameter("jwt missing")
	}
	if c.uri == "" {
		return trace.BadParameter("uri missing")
	}
	if c.publicAddr == "" {
		return trace.BadParameter("public addr missing")
	}
	if c.tr == nil {
		return trace.BadParameter("round tripper missing")
	}
	if c.log == nil {
		return trace.BadParameter("logger missing")
	}
	return nil
}

// forwarder will rewrite and forward the request to the target address.
type forwarder struct {
	closeContext context.Context

	c *forwarderConfig

	uri *url.URL
}

// newForwarder creates a new forwarder that can re-write and round trip a
// HTTP request.
func newForwarder(ctx context.Context, c *forwarderConfig) (*forwarder, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse the target address once then inject it into all requests.
	uri, err := url.Parse(c.uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &forwarder{
		closeContext: ctx,
		c:            c,
		uri:          uri,
	}, nil
}

// RoundTrip make the request and log the request/response pair in the audit log.
func (f *forwarder) RoundTrip(r *http.Request) (*http.Response, error) {
	// Update the target address of the request so it's forwarded correctly.
	r.URL.Scheme = f.uri.Scheme
	r.URL.Host = f.uri.Host

	resp, err := f.c.tr.RoundTrip(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// Rewrite request headers to add in JWT header and remove any Teleport
// related authentication headers.
func (f *forwarder) Rewrite(r *http.Request) {
	// Add in JWT headers.
	r.Header.Add(teleport.AppJWTHeader, f.c.jwt)
	r.Header.Add(teleport.AppCFHeader, f.c.jwt)

	// Remove the session ID header before forwarding the session to the
	// target application.
	r.Header.Del(teleport.AppSessionIDHeader)
}
