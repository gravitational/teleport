/*
Copyright 2017 Gravitational, Inc.

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
package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/sshutils"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"

	"golang.org/x/crypto/ssh"

	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentConnectProxy,
})

// dialWithDeadline works around the case when net.DialWithTimeout
// succeeds, but key exchange hangs. Setting deadline on connection
// prevents this case from happening
func dialWithDeadline(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	conn, err := net.DialTimeout(network, addr, config.Timeout)
	if err != nil {
		return nil, err
	}
	return sshutils.NewClientConnWithDeadline(conn, addr, config)
}

// dialALPNWithDeadline allows connecting to Teleport in single-port mode. SSH protocol is wrapped into
// TLS connection where TLS ALPN protocol is set to ProtocolReverseTunnel allowing ALPN Proxy to route the
// incoming connection to ReverseTunnel proxy service.
func dialALPNWithDeadline(network string, addr string, config *ssh.ClientConfig, insecure bool) (*ssh.Client, error) {
	dialer := &net.Dialer{
		Timeout: config.Timeout,
	}
	address, err := utils.ParseAddr(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConn, err := tls.DialWithDialer(dialer, network, addr, &tls.Config{
		NextProtos:         []string{string(alpncommon.ProtocolReverseTunnel)},
		InsecureSkipVerify: insecure,
		ServerName:         address.Host(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshutils.NewClientConnWithDeadline(tlsConn, addr, config)
}

// A Dialer is a means for a client to establish a SSH connection.
type Dialer interface {
	// Dial establishes a client connection to a SSH server.
	Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error)

	// DialTimeout acts like Dial but takes a timeout.
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
}

type directDial struct {
	// tlsRoutingEnabled indicates that proxy is running in TLSRouting mode.
	tlsRoutingEnabled bool
	// insecure is whether to skip certificate validation.
	insecure bool
}

// Dial calls ssh.Dial directly.
func (d directDial) Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	if d.tlsRoutingEnabled {
		client, err := dialALPNWithDeadline(network, addr, config, d.insecure)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return client, nil
	}
	client, err := dialWithDeadline(network, addr, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

// DialTimeout acts like Dial but takes a timeout.
func (d directDial) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if d.tlsRoutingEnabled {
		addr, err := utils.ParseAddr(address)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tlsConn, err := tls.Dial("tcp", address, &tls.Config{
			NextProtos:         []string{string(alpncommon.ProtocolReverseTunnel)},
			InsecureSkipVerify: d.insecure,
			ServerName:         addr.Host(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tlsConn, nil
	}
	conn, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

type proxyDial struct {
	// proxyHost is the HTTPS proxy address.
	proxyHost string
	// tlsRoutingEnabled indicates that proxy is running in TLSRouting mode.
	tlsRoutingEnabled bool
	// insecure is whether to skip certificate validation.
	insecure bool
}

// DialTimeout acts like Dial but takes a timeout.
func (d proxyDial) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	// Build a proxy connection first.
	ctx := context.Background()
	if timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = timeoutCtx
	}
	conn, err := dialProxy(ctx, d.proxyHost, address)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if d.tlsRoutingEnabled {
		address, err := utils.ParseAddr(address)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conn = tls.Client(conn, &tls.Config{
			NextProtos:         []string{string(alpncommon.ProtocolReverseTunnel)},
			InsecureSkipVerify: d.insecure,
			ServerName:         address.Host(),
		})
	}
	return conn, nil
}

// Dial first connects to a proxy, then uses the connection to establish a new
// SSH connection.
func (d proxyDial) Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	// Build a proxy connection first.
	pconn, err := dialProxy(context.Background(), d.proxyHost, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.Timeout > 0 {
		pconn.SetReadDeadline(time.Now().Add(config.Timeout))
	}
	if d.tlsRoutingEnabled {
		address, err := utils.ParseAddr(addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pconn = tls.Client(pconn, &tls.Config{
			NextProtos:         []string{string(alpncommon.ProtocolReverseTunnel)},
			InsecureSkipVerify: d.insecure,
			ServerName:         address.Host(),
		})
	}

	// Do the same as ssh.Dial but pass in proxy connection.
	c, chans, reqs, err := ssh.NewClientConn(pconn, addr, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.Timeout > 0 {
		pconn.SetReadDeadline(time.Time{})
	}
	return ssh.NewClient(c, chans, reqs), nil
}

type dialerOptions struct {
	// tlsRoutingEnabled indicates that proxy is running in TLSRouting mode.
	tlsRoutingEnabled bool
	// insecureSkipTLSVerify is whether to skip certificate validation.
	insecureSkipTLSVerify bool
}

// DialerOptionFunc allows setting options as functional arguments to DialerFromEnvironment
type DialerOptionFunc func(options *dialerOptions)

// WithALPNDialer creates a dialer that allows to Teleport running in single-port mode.
func WithALPNDialer() DialerOptionFunc {
	return func(options *dialerOptions) {
		options.tlsRoutingEnabled = true
	}
}

// WithInsecureSkipTLSVerify skips the certs verifications.
func WithInsecureSkipTLSVerify(insecure bool) DialerOptionFunc {
	return func(options *dialerOptions) {
		options.insecureSkipTLSVerify = insecure
	}
}

// DialerFromEnvironment returns a Dial function. If the https_proxy or http_proxy
// environment variable are set, it returns a function that will dial through
// said proxy server. If neither variable is set, it will connect to the SSH
// server directly.
func DialerFromEnvironment(addr string, opts ...DialerOptionFunc) Dialer {
	// Try and get proxy addr from the environment.
	proxyAddr := getProxyAddress(addr)

	var options dialerOptions
	for _, opt := range opts {
		opt(&options)
	}

	// If no proxy settings are in environment return regular ssh dialer,
	// otherwise return a proxy dialer.
	if proxyAddr == "" {
		log.Debugf("No proxy set in environment, returning direct dialer.")
		return directDial{
			tlsRoutingEnabled: options.tlsRoutingEnabled,
			insecure:          options.insecureSkipTLSVerify,
		}
	}
	log.Debugf("Found proxy %q in environment, returning proxy dialer.", proxyAddr)
	return proxyDial{
		proxyHost:         proxyAddr,
		tlsRoutingEnabled: options.tlsRoutingEnabled,
		insecure:          options.insecureSkipTLSVerify,
	}
}

type DirectDialerOptFunc func(dial *directDial)

func dialProxy(ctx context.Context, proxyAddr string, addr string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		log.Warnf("Unable to dial to proxy: %v: %v.", proxyAddr, err)
		return nil, trace.ConvertSystemError(err)
	}

	connectReq := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}
	err = connectReq.Write(conn)
	if err != nil {
		log.Warnf("Unable to write to proxy: %v.", err)
		return nil, trace.Wrap(err)
	}

	// Read in the response. http.ReadResponse will read in the status line, mime
	// headers, and potentially part of the response body. the body itself will
	// not be read, but kept around so it can be read later.
	br := bufio.NewReader(conn)
	// Per the above comment, we're only using ReadResponse to check the status
	// and then hand off the underlying connection to the caller.
	// resp.Body.Close() would drain conn and close it, we don't need to do it
	// here. Disabling bodyclose linter for this edge case.
	//nolint:bodyclose
	resp, err := http.ReadResponse(br, connectReq)
	if err != nil {
		conn.Close()
		log.Warnf("Unable to read response: %v.", err)
		return nil, trace.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, trace.BadParameter("unable to proxy connection: %v", resp.Status)
	}

	// Return a bufferedConn that wraps a net.Conn and a *bufio.Reader. this
	// needs to be done because http.ReadResponse will buffer part of the
	// response body in the *bufio.Reader that was passed in. reads must first
	// come from anything buffered, then from the underlying connection otherwise
	// data will be lost.
	return &bufferedConn{
		Conn:   conn,
		reader: br,
	}, nil
}

func getProxyAddress(addr string) string {
	envs := []string{
		teleport.HTTPSProxy,
		strings.ToLower(teleport.HTTPSProxy),
		teleport.HTTPProxy,
		strings.ToLower(teleport.HTTPProxy),
	}

	for _, v := range envs {
		envAddr := os.Getenv(v)
		if envAddr == "" {
			continue
		}
		proxyAddr, err := parse(envAddr)
		if err != nil {
			log.Debugf("Unable to parse environment variable %q: %q.", v, envAddr)
			continue
		}
		log.Debugf("Successfully parsed environment variable %q: %q to %q.", v, envAddr, proxyAddr)
		if !useProxy(addr) {
			log.Debugf("Matched NO_PROXY override for %q: %q, going to ignore proxy variable.", v, envAddr)
			return ""
		}
		return proxyAddr
	}

	log.Debugf("No valid environment variables found.")
	return ""
}

// bufferedConn is used when part of the data on a connection has already been
// read by a *bufio.Reader. Reads will first try and read from the
// *bufio.Reader and when everything has been read, reads will go to the
// underlying connection.
type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

// Read first reads from the *bufio.Reader any data that has already been
// buffered. Once all buffered data has been read, reads go to the net.Conn.
func (bc *bufferedConn) Read(b []byte) (n int, err error) {
	if bc.reader.Buffered() > 0 {
		return bc.reader.Read(b)
	}
	return bc.Conn.Read(b)
}

// parse will extract the host:port of the proxy to dial to. If the
// value is not prefixed by "http", then it will prepend "http" and try.
func parse(addr string) (string, error) {
	proxyurl, err := url.Parse(addr)
	if err != nil || !strings.HasPrefix(proxyurl.Scheme, "http") {
		proxyurl, err = url.Parse("http://" + addr)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	return proxyurl.Host, nil
}
