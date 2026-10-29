/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package safetransport

import (
	"cmp"
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"code.dny.dev/ssrf"
	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpproxy"

	"github.com/gravitational/teleport/lib/defaults"
)

// Dial timeouts match http.DefaultTransport.
const (
	dialTimeout = 30 * time.Second
	keepAlive   = 30 * time.Second
)

// defaultDialer refuses all private network addresses.
var defaultDialer = Dialer()

// New returns a round tripper that refuses connections to private network addresses.
//
// Direct requests have the address checked at dial time.
//
// Proxied requests have the address checked at resolution, as the address dialed
// is that of the proxy, rather than the upstream, so a dial time check would be checking the wrong address.
// This is inherently racy, in that the address can resolve publicly at validation, but privately at the proxy.
// This is mitigated by only accepting HTTPS, which would result in TLS handshake failure, should it occur.
func New() (http.RoundTripper, error) {
	direct, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	direct.Proxy = nil
	direct.DialContext = defaultDialer.DialContext

	proxied, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
	proxy := func(req *http.Request) (*url.URL, error) {
		return proxyFunc(req.URL)
	}
	proxied.Proxy = proxy

	return &transport{direct: direct, proxied: proxied, proxy: proxy}, nil
}

// Dialer returns a dialer that refuses connections to all private network addresses,
// with the exception of any optionally provided allowed prefixes.
func Dialer(allowedPrefixes ...netip.Prefix) *net.Dialer {
	var v4, v6 []netip.Prefix
	for _, prefix := range allowedPrefixes {
		if prefix.Addr().Unmap().Is4() {
			v4 = append(v4, prefix)
		} else {
			v6 = append(v6, prefix)
		}
	}

	return &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: keepAlive,
		Control: ssrf.New(
			ssrf.WithAnyPort(),
			ssrf.WithAllowedV4Prefixes(v4...),
			ssrf.WithAllowedV6Prefixes(v6...),
		).Safe,
	}
}

// transport sends a request over the direct path or over a proxy when one is configured.
type transport struct {
	direct  http.RoundTripper
	proxied http.RoundTripper
	proxy   func(*http.Request) (*url.URL, error)
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.EqualFold(req.URL.Scheme, "https") {
		return nil, trace.BadParameter("requests must use HTTPS")
	}

	proxyURL, err := t.proxy(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if proxyURL == nil {
		return t.direct.RoundTrip(req)
	}

	if err := ValidateURL(req.Context(), req.URL); err != nil {
		return nil, trace.Wrap(err)
	}

	return t.proxied.RoundTrip(req)
}

func (t *transport) CloseIdleConnections() {
	for _, rt := range []http.RoundTripper{t.proxied, t.direct} {
		if closer, ok := rt.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
	}
}

// ValidateURL resolves the host of the URL and returns an error if it is not an
// HTTPS URL or if it resolves to a prohibited address using the default dialer.
func ValidateURL(ctx context.Context, u *url.URL) error {
	if u == nil {
		return trace.BadParameter("url is required")
	}

	if !strings.EqualFold(u.Scheme, "https") {
		return trace.BadParameter("%q is not an HTTPS URL", u.Redacted())
	}

	host := u.Hostname()
	if host == "" {
		return trace.BadParameter("%q has no hostname", u.Redacted())
	}

	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(addresses) == 0 {
		return trace.BadParameter("%q resolves to no addresses", u.Redacted())
	}

	port := cmp.Or(u.Port(), "443")
	for _, address := range addresses {
		addr := address.Unmap()

		network := "tcp6"
		if addr.Is4() {
			network = "tcp4"
		}

		if err := defaultDialer.Control(network, net.JoinHostPort(addr.String(), port), nil); err != nil {
			return trace.BadParameter("%q resolves to a prohibited network address", u.Redacted())
		}
	}

	return nil
}
