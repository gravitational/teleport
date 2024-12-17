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

package proxy

import (
	"context"
	"net"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	apiutils "github.com/gravitational/teleport/api/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentConnectProxy)

// A Dialer is a means for a client to establish a SSH connection.
type Dialer interface {
	// Dial establishes a client connection to a SSH server.
	Dial(ctx context.Context, network string, addr string, config *ssh.ClientConfig) (*tracessh.Client, error)

	// DialTimeout acts like Dial but takes a timeout.
	DialTimeout(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error)
}

type directDial struct {
	// alpnDialer is the dialer used for TLS routing.
	alpnDialer apiclient.ContextDialer
	// proxyHeaderGetter is used if present to get signed PROXY headers to propagate client's IP.
	// Used by proxy's web server to make calls on behalf of connected clients.
	proxyHeaderGetter apiclient.PROXYHeaderGetter
}

// Dial returns traced SSH client connection
func (d directDial) Dial(ctx context.Context, network string, addr string, config *ssh.ClientConfig) (*tracessh.Client, error) {
	conn, err := d.DialTimeout(ctx, network, addr, config.Timeout)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Works around the case when net.DialWithTimeout succeeds, but key exchange hangs.
	// Setting deadline on connection prevents this case from happening
	return tracessh.NewClientConnWithDeadline(ctx, conn, addr, config)
}

// DialTimeout acts like Dial but takes a timeout.
func (d directDial) DialTimeout(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	if d.alpnDialer != nil {
		conn, err := d.alpnDialer.DialContext(ctx, network, address)
		return conn, trace.Wrap(err)
	}

	dialer := apiclient.NewPROXYHeaderDialer(&net.Dialer{
		Timeout: timeout,
	}, d.proxyHeaderGetter)

	conn, err := dialer.DialContext(ctx, network, address)
	return conn, trace.Wrap(err)
}

type proxyDial struct {
	// proxyHost is the HTTPS proxy address.
	proxyURL *url.URL
	// insecure is whether to skip certificate validation.
	insecure bool
	// proxyHeaderGetter is used if present to get signed PROXY headers to propagate client's IP.
	// Used by proxy's web server to make calls on behalf of connected clients.
	proxyHeaderGetter apiclient.PROXYHeaderGetter
	// alpnDialer is the dialer used for TLS routing.
	alpnDialer apiclient.ContextDialer
}

// DialTimeout acts like Dial but takes a timeout.
func (d proxyDial) DialTimeout(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	// Build a proxy connection first.
	if timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = timeoutCtx
	}

	// ALPN dialer handles proxy URL internally.
	if d.alpnDialer != nil {
		tlsConn, err := d.alpnDialer.DialContext(ctx, network, address)
		return tlsConn, trace.Wrap(err)
	}

	conn, err := apiclient.DialProxy(ctx, d.proxyURL, address, apiclient.WithInsecureSkipVerify(d.insecure),
		apiclient.WithPROXYHeaderGetter(d.proxyHeaderGetter))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

// Dial first connects to a proxy, then uses the connection to establish a new
// SSH connection.
func (d proxyDial) Dial(ctx context.Context, network string, addr string, config *ssh.ClientConfig) (*tracessh.Client, error) {
	// Build a proxy connection first.
	pconn, err := d.DialTimeout(ctx, network, addr, config.Timeout)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if config.Timeout > 0 {
		if err := pconn.SetReadDeadline(time.Now().Add(config.Timeout)); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Do the same as ssh.Dial but pass in proxy connection.
	c, chans, reqs, err := tracessh.NewClientConn(ctx, pconn, addr, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.Timeout > 0 {
		if err := pconn.SetReadDeadline(time.Time{}); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return tracessh.NewClient(c, chans, reqs), nil
}

type dialerOptions struct {
	// insecureSkipTLSVerify is whether to skip certificate validation.
	insecureSkipTLSVerify bool
	// alpnDialer is the dialer used for TLS routing.
	alpnDialer apiclient.ContextDialer

	proxyHeaderGetter apiclient.PROXYHeaderGetter
}

// DialerOptionFunc allows setting options as functional arguments to DialerFromEnvironment
type DialerOptionFunc func(options *dialerOptions)

// WithALPNDialer creates a dialer that allows to Teleport running in single-port mode.
func WithALPNDialer(alpnDialerConfig apiclient.ALPNDialerConfig) DialerOptionFunc {
	return func(options *dialerOptions) {
		options.alpnDialer = apiclient.NewALPNDialer(alpnDialerConfig)
	}
}

// WithInsecureSkipTLSVerify skips the certs verifications.
func WithInsecureSkipTLSVerify(insecure bool) DialerOptionFunc {
	return func(options *dialerOptions) {
		options.insecureSkipTLSVerify = insecure
	}
}

// WithPROXYHeaderGetter adds PROXY headers getter, which is used to propagate client's real IP
func WithPROXYHeaderGetter(proxyHeaderGetter apiclient.PROXYHeaderGetter) DialerOptionFunc {
	return func(options *dialerOptions) {
		options.proxyHeaderGetter = proxyHeaderGetter
	}
}

// DialerFromEnvironment returns a Dial function. If the https_proxy or http_proxy
// environment variable are set, it returns a function that will dial through
// said proxy server. If neither variable is set, it will connect to the SSH
// server directly.
func DialerFromEnvironment(addr string, opts ...DialerOptionFunc) Dialer {
	// Try and get proxy addr from the environment.
	proxyURL := apiutils.GetProxyURL(addr)

	var options dialerOptions
	for _, opt := range opts {
		opt(&options)
	}

	// If no proxy settings are in environment return regular ssh dialer,
	// otherwise return a proxy dialer.
	if proxyURL == nil {
		log.DebugContext(context.Background(), "No proxy set in environment, returning direct dialer")
		return directDial{
			alpnDialer:        options.alpnDialer,
			proxyHeaderGetter: options.proxyHeaderGetter,
		}
	}
	log.DebugContext(context.Background(), "Found proxy in environment, returning proxy dialer",
		"proxy_url", logutils.StringerAttr(proxyURL),
	)
	return proxyDial{
		proxyURL:          proxyURL,
		insecure:          options.insecureSkipTLSVerify,
		alpnDialer:        options.alpnDialer,
		proxyHeaderGetter: options.proxyHeaderGetter,
	}
}

type DirectDialerOptFunc func(dial *directDial)
