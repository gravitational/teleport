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

// Package app connections to applications over a reverse tunnel and forwards
// HTTP requests to them.
package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/ttlmap"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

// HandlerConfig is the configuration for an application handler.
type HandlerConfig struct {
	// Clock is used to control time in tests.
	Clock clockwork.Clock
	// AuthClient is a direct client to auth.
	AuthClient auth.ClientI
	// AccessPoint is caching client to auth.
	AccessPoint auth.AccessPoint
	// ProxyClient holds connections to leaf clusters.
	ProxyClient reversetunnel.Server
	// CipherSuites is the list of TLS cipher suites that have been configured
	// for this process.
	CipherSuites []uint16
}

// CheckAndSetDefaults validates configuration.
func (c *HandlerConfig) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.AuthClient == nil {
		return trace.BadParameter("auth client missing")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("access point missing")
	}
	if c.ProxyClient == nil {
		return trace.BadParameter("proxy client missing")
	}
	if len(c.CipherSuites) == 0 {
		return trace.BadParameter("ciphersuites missing")
	}

	return nil
}

// Handler is an application handler.
type Handler struct {
	c *HandlerConfig

	log *logrus.Entry

	mu    sync.Mutex
	cache *ttlmap.TTLMap

	router *httprouter.Router
}

// NewHandler returns a new application handler.
func NewHandler(c *HandlerConfig) (*Handler, error) {
	err := c.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h := &Handler{
		c: c,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentAppProxy,
		}),
	}

	// Cache of request forwarders.
	h.cache, err = ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h.router = httprouter.New()
	h.router.GET("/x-teleport-auth", makeRouterHandler(h.handleFragment))
	h.router.POST("/x-teleport-auth", makeRouterHandler(h.handleFragment))
	h.router.GET("/x-teleport-logout", h.withRouterAuth(h.handleLogout))
	h.router.NotFound = h.withAuth(h.handleForward)

	return h, nil
}

// ForwardToApp checks if the request is bound for the application handler.
// Used by "ServeHTTP" within the "web" package to make a decision if the
// request should be processed by the UI or forwarded to an application.
func (h *Handler) IsAuthenticatedApp(r *http.Request) bool {
	// The only unauthenticated endpoint supported is the special
	// "x-teleport-auth" endpoint. If the request is coming to this special
	// endpoint, it should be processed by application handlers.
	if r.URL.Path == "/x-teleport-auth" {
		return true
	}

	// Check if an application specific cookie exists. If it exists, forward the
	// request to an application handler otherwise allow the UI to handle it.
	_, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return true
}

// IsUnauthenticatedApp checks if the client is attempting to connect to a
// host that is different than the public address of the proxy. If it is, it
// redirects back to the application launcher in the Web UI.
func (h *Handler) IsUnauthenticatedApp(r *http.Request, publicAddr string) (string, bool) {
	addr, err := utils.ParseAddr(r.Host)
	if err != nil {
		return "", false
	}

	// The following requests can not be for an application:
	//
	//  * The request is for localhost or loopback.
	//  * The request is for an IP address.
	//  * The request is for the public address of the proxy.
	if utils.IsLocalhost(addr.Host()) {
		return "", false
	}
	if net.ParseIP(addr.Host()) != nil {
		return "", false
	}
	if r.Host == publicAddr {
		return "", false
	}

	// At this point, it is assumed the caller is requesting an application and
	// not the proxy, redirect the caller to the application launcher.
	u := url.URL{
		Scheme: "https",
		Host:   publicAddr,
		Path:   fmt.Sprintf("/web/launch/%v", addr.Host()),
	}
	return u.String(), true
}

// ServeHTTP will forward the *http.Request to the application proxy service.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func (h *Handler) handleForward(w http.ResponseWriter, r *http.Request, ws services.WebSession) error {
	// Fetch a cached request forwarder or create one if this is the first
	// request (or the process has been restarted).
	session, err := h.getSession(r.Context(), ws)
	if err != nil {
		return trace.Wrap(err)
	}

	// Forward the request to the Teleport application proxy service.
	session.fwd.ServeHTTP(w, r)
	return nil
}

// authenticate will check if request carries a session cookie matching a
// session in the backend.
func (h *Handler) authenticate(ctx context.Context, r *http.Request) (services.WebSession, error) {
	cookieValue, err := extractCookie(r)
	if err != nil {
		h.log.Warnf("Failed to extract session cookie: %v.", err)
		return nil, trace.AccessDenied("invalid session")
	}

	// Check that the session exists in the backend cache. This allows the user
	// to logout and invalidate their application session immediately. This
	// lookup should also be fast because it's in the local cache.
	session, err := h.c.AccessPoint.GetAppSession(ctx, services.GetAppSessionRequest{
		SessionID: cookieValue,
	})
	if err != nil {
		h.log.Warnf("Failed to fetch application session: %v.", err)
		return nil, trace.AccessDenied("invalid session")
	}

	return session, nil
}

// cacheGet will fetch the forwarder from the cache.
func (h *Handler) cacheGet(key string) (*session, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if s, ok := h.cache.Get(key); ok {
		if sess, sok := s.(*session); sok {
			return sess, nil
		}
		return nil, trace.BadParameter("invalid type stored in cache: %T", s)
	}
	return nil, trace.NotFound("forwarder not found")
}

// cacheSet will add the forwarder to the cache.
func (h *Handler) cacheSet(key string, value *session, ttl time.Duration) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.cache.Set(key, value, ttl); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// extractCookie extracts the cookie from the *http.Request.
func extractCookie(r *http.Request) (string, error) {
	rawCookie, err := r.Cookie(cookieName)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if rawCookie != nil && rawCookie.Value == "" {
		return "", trace.BadParameter("cookie missing")
	}

	return rawCookie.Value, nil
}

type routerFunc func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error
type routerAuthFunc func(w http.ResponseWriter, r *http.Request, p httprouter.Params, ws services.WebSession) error

type handlerAuthFunc func(w http.ResponseWriter, r *http.Request, ws services.WebSession) error
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (h *Handler) withRouterAuth(handler routerAuthFunc) httprouter.Handle {
	return makeRouterHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		ws, err := h.authenticate(r.Context(), r)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := handler(w, r, p, ws); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
}

func (h *Handler) withAuth(handler handlerAuthFunc) http.HandlerFunc {
	return makeHandler(func(w http.ResponseWriter, r *http.Request) error {
		ws, err := h.authenticate(r.Context(), r)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := handler(w, r, ws); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
}

func makeRouterHandler(handler routerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if err := handler(w, r, p); err != nil {
			writeError(w, err)
			return
		}
	}
}

func makeHandler(handler handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			writeError(w, err)
			return
		}
	}
}

func writeError(w http.ResponseWriter, err error) {
	code := trace.ErrorToCode(err)
	http.Error(w, http.StatusText(code), code)
}

const (
	// cookieName is the name of the application session cookie.
	cookieName = "grv_app_session"
)
