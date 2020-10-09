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
	"strings"
	"sync"
	"time"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/ttlmap"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// HandlerConfig is the configuration for an application handler.
type HandlerConfig struct {
	// Clock is used to control time in tests.
	Clock clockwork.Clock
	// AccessPoint is a caching client.
	AccessPoint auth.AccessPoint
	// ProxyClient holds connections to leaf clusters.
	ProxyClient reversetunnel.Server
}

// CheckAndSetDefaults validates configuration.
func (c *HandlerConfig) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.AccessPoint == nil {
		return trace.BadParameter("access point missing")
	}
	if c.ProxyClient == nil {
		return trace.BadParameter("proxy client missing")
	}

	return nil
}

// Handler is an application handler.
type Handler struct {
	c *HandlerConfig

	log *logrus.Entry

	tr http.RoundTripper

	mu    sync.Mutex
	cache *ttlmap.TTLMap
}

// NewHandler returns a new application handler.
func NewHandler(c *HandlerConfig) (*Handler, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a http.RoundTripper that is used to cache and limit transport
	// connections over the reverse tunnel subsystem.
	tr, err := newTransport(c.ProxyClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Cache of request forwarders.
	cache, err := ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Handler{
		c: c,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentAppProxy,
		}),
		tr:    tr,
		cache: cache,
	}, nil
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
	requestedHost, err := utils.ParseAddr(r.Host)
	if err != nil {
		return "", false
	}

	// TODO(russjones): Benchmark time to loop over all applications and look
	// for a match.
	if utils.IsLocalhost(requestedHost.Host()) {
		return "", false
	}
	if net.ParseIP(requestedHost.Host()) != nil {
		return "", false
	}
	if r.Host == publicAddr {
		return "", false
	}

	host, _, _ := net.SplitHostPort(r.Host)

	//u, err := url.Parse(fmt.Sprintf("https://%v/web/launch/%v", publicAddr, r.Host))
	u, err := url.Parse(fmt.Sprintf("https://%v/web/launch/%v", publicAddr, host))
	if err != nil {
		h.log.Debugf("Failed to parse while handling unauthenticated request to %v: %v.", r.Host, err)
		return "", false
	}
	return u.String(), true
}

// ServeHTTP will forward the *http.Request to the application proxy service.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.serveHTTP(w, r); err != nil {
		h.log.Warnf("Failed to serve request: %v.", err)

		// Covert trace error type to HTTP and write response.
		code := trace.ErrorToCode(err)
		http.Error(w, http.StatusText(code), code)
	}
}

func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	// If the target is an application but it hits the special "x-teleport-auth"
	// endpoint, then perform redirect authentication logic.
	if r.URL.Path == "/x-teleport-auth" {
		if err := h.handleFragment(w, r); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	// Authenticate the session based off the session cookie.
	session, err := h.authenticate(r.Context(), r)
	if err != nil {
		return trace.Wrap(err)
	}

	// Fetch a cached request forwarder or create one if this is the first
	// request (or the process has been restarted).
	fwd, err := h.getForwarder(r.Context(), session)
	if err != nil {
		return trace.Wrap(err)
	}

	// Forward the request to the Teleport application proxy service.
	fwd.ServeHTTP(w, r)
	return nil
}

// authenticate will check if request carries a session cookie matching a
// session in the backend.
func (h *Handler) authenticate(ctx context.Context, r *http.Request) (services.WebSession, error) {
	// Extract the session cookie from the *http.Request.
	cookie, err := parseCookie(r)
	if err != nil {
		h.log.Warnf("Failed to parse session cookie: %v.", err)
		return nil, trace.AccessDenied("invalid session")
	}

	// Check that the session exists in the backend cache. This allows the user
	// to logout and invalidate their application session immediately. This
	// lookup should also be fast because it's in the local cache.
	session, err := h.c.AccessPoint.GetAppWebSession(ctx, services.GetAppWebSessionRequest{
		Username:   cookie.Username,
		ParentHash: cookie.ParentHash,
		SessionID:  cookie.SessionID,
	})
	if err != nil {
		h.log.Warnf("Failed to fetch application session: %v.", err)
		return nil, trace.AccessDenied("invalid session")
	}

	return session, nil
}

// getForwarder returns a request forwarder used to proxy the request to the
// application proxy component of Teleport which will then forward the
// request to the target application.
func (h *Handler) getForwarder(ctx context.Context, session services.WebSession) (*forward.Forwarder, error) {
	// If a cached forwarder exists, return it right away.
	fwd, err := h.cacheGet(session.GetName())
	if err == nil {
		return fwd, nil
	}

	// Create the forwarder.
	fwder, err := newForwarder(forwarderConfig{
		uri:       fmt.Sprintf("http://%v.%v", session.GetServerID(), session.GetClusterName()),
		sessionID: session.GetSessionID(),
		tr:        h.tr,
		log:       h.log,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fwd, err = forward.New(
		forward.RoundTripper(fwder),
		forward.Rewriter(fwder),
		forward.Logger(h.log))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Put the forwarder in the cache so the next request can use it.
	err = h.cacheSet(session.GetName(), fwd, session.Expiry().Sub(h.c.Clock.Now()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return fwd, nil
}

// cacheGet will fetch the forwarder from the cache.
func (h *Handler) cacheGet(key string) (*forward.Forwarder, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if f, ok := h.cache.Get(key); ok {
		if fwd, fok := f.(*forward.Forwarder); fok {
			return fwd, nil
		}
		return nil, trace.BadParameter("invalid type stored in cache: %T", f)
	}
	return nil, trace.NotFound("forwarder not found")
}

// cacheSet will add the forwarder to the cache.
func (h *Handler) cacheSet(key string, value *forward.Forwarder, ttl time.Duration) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.cache.Set(key, value, ttl); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// forwarderConfig is the configuration for a forwarder.
type forwarderConfig struct {
	uri       string
	sessionID string
	tr        http.RoundTripper
	log       *logrus.Entry
}

// Check will valid the configuration of a forwarder.
func (c forwarderConfig) Check() error {
	if c.uri == "" {
		return trace.BadParameter("uri missing")
	}
	if c.sessionID == "" {
		return trace.BadParameter("session ID missing")
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
	c forwarderConfig

	uri *url.URL
}

// newForwarder returns a new forwarder.
func newForwarder(c forwarderConfig) (*forwarder, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse the target address once then inject it into all requests.
	uri, err := url.Parse(c.uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &forwarder{
		c:   c,
		uri: uri,
	}, nil
}

func (f *forwarder) RoundTrip(r *http.Request) (*http.Response, error) {
	// Update the target address of the request so it's forwarded correctly.
	// Format is always serverID.clusterName.
	r.URL.Scheme = f.uri.Scheme
	r.URL.Host = f.uri.Host

	resp, err := f.c.tr.RoundTrip(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func (f *forwarder) Rewrite(r *http.Request) {
	// Pass the application session ID to the application service in a header.
	// This will be removed by the application proxy service before forwarding
	// the request to the target application.
	r.Header.Set(teleport.AppSessionIDHeader, f.c.sessionID)

	// Remove the application specific session cookie from the header. This is
	// done by first wiping out the "Cookie" header then adding back all cookies
	// except the Teleport application specific session cookie. This appears to
	// be the best way to serialize cookies.
	cookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, cookie := range cookies {
		if cookie.Name == cookieName {
			continue
		}
		r.AddCookie(cookie)
	}
}

// newTransport creates a http.RoundTripper that uses the reverse tunnel
// subsystem to build the connection. This allows re-use of the transports
// connection pooling logic instead of needing to write and maintain our own.
func newTransport(proxyClient reversetunnel.Server) (http.RoundTripper, error) {
	// Clone the default transport to pick up sensible defaults.
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, trace.BadParameter("invalid transport type %T", http.DefaultTransport)
	}
	tr := defaultTransport.Clone()

	// Increase the size of the transports connection pool. This substantially
	// improves the performance of Teleport under load as it reduces the number
	// of TLS handshakes performed.
	tr.MaxIdleConns = defaults.HTTPMaxIdleConns
	tr.MaxIdleConnsPerHost = defaults.HTTPMaxIdleConnsPerHost

	// Set IdleConnTimeout on the transport, this defines the maximum amount of
	// time before idle connections are closed. Leaving this unset will lead to
	// connections open forever and will cause memory leaks in a long running
	// process.
	tr.IdleConnTimeout = defaults.HTTPIdleTimeout

	// The address field is always formatted as serverUUID.clusterName allowing
	// the connection pool maintained by the transport to differentiate
	// connections to different application proxy hosts.
	tr.DialContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
		serverID, clusterName, err := extract(addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusterClient, err := proxyClient.GetSite(clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		conn, err := clusterClient.Dial(reversetunnel.DialParams{
			ServerID: fmt.Sprintf("%v.%v", serverID, clusterName),
			ConnType: services.AppTunnel,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}

	return tr, nil
}

// extract takes an address in the form http://serverID.clusterName:80 and
// returns serverID and clusterName.
func extract(address string) (string, string, error) {
	// Strip port suffix.
	address = strings.TrimSuffix(address, ":80")

	// Split into two parts: serverID and clusterName.
	index := strings.Index(address, ".")
	if index == -1 {
		return "", "", fmt.Errorf("")
	}

	return address[:index], address[index+1:], nil
}
