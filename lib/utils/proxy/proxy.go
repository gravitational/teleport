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
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/utils/sshutils"
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
func (d directDial) dialALPNWithDeadline(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	dialer := &net.Dialer{
		Timeout: config.Timeout,
	}
	address, err := utils.ParseAddr(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conf, err := d.getTLSConfig(address)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConn, err := tls.DialWithDialer(dialer, network, addr, conf)
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
	// insecure is whether to skip certificate validation.
	insecure bool
	// tlsRoutingEnabled indicates that proxy is running in TLSRouting mode.
	tlsRoutingEnabled bool
	// tlsConfig is the TLS config to use.
	tlsConfig *tls.Config
}

// getTLSConfig configures the dialers TLS config for a specified address.
func (d directDial) getTLSConfig(addr *utils.NetAddr) (*tls.Config, error) {
	if d.tlsConfig == nil {
		return nil, trace.BadParameter("TLS config was nil")
	}
	tlsConfig := d.tlsConfig.Clone()
	tlsConfig.ServerName = addr.Host()
	tlsConfig.InsecureSkipVerify = d.insecure
	return tlsConfig, nil
}

// Dial calls ssh.Dial directly.
func (d directDial) Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	if d.tlsRoutingEnabled {
		client, err := d.dialALPNWithDeadline(network, addr, config)
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
		conf, err := d.getTLSConfig(addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tlsConn, err := tls.Dial("tcp", address, conf)
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
	// insecure is whether to skip certificate validation.
	insecure bool
	// tlsRoutingEnabled indicates that proxy is running in TLSRouting mode.
	tlsRoutingEnabled bool
	// tlsConfig is the TLS config to use.
	tlsConfig *tls.Config
}

// getTLSConfig configures the dialers TLS config for a specified address.
func (d proxyDial) getTLSConfig(addr *utils.NetAddr) (*tls.Config, error) {
	if d.tlsConfig == nil {
		return nil, trace.BadParameter("TLS config was nil")
	}
	tlsConfig := d.tlsConfig.Clone()
	tlsConfig.ServerName = addr.Host()
	tlsConfig.InsecureSkipVerify = d.insecure
	return tlsConfig, nil
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
	conn, err := apiclient.DialProxy(ctx, d.proxyHost, address)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if d.tlsRoutingEnabled {
		address, err := utils.ParseAddr(address)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conf, err := d.getTLSConfig(address)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conn = tls.Client(conn, conf)
	}
	return conn, nil
}

// Dial first connects to a proxy, then uses the connection to establish a new
// SSH connection.
func (d proxyDial) Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	// Build a proxy connection first.
	pconn, err := apiclient.DialProxy(context.Background(), d.proxyHost, addr)
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
		conf, err := d.getTLSConfig(address)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pconn = tls.Client(pconn, conf)
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
	// insecureSkipTLSVerify is whether to skip certificate validation.
	insecureSkipTLSVerify bool
	// tlsRoutingEnabled indicates that proxy is running in TLSRouting mode.
	tlsRoutingEnabled bool
	// tlsConfig is the TLS config to use for TLS routing.
	tlsConfig *tls.Config
}

// DialerOptionFunc allows setting options as functional arguments to DialerFromEnvironment
type DialerOptionFunc func(options *dialerOptions)

// WithALPNDialer creates a dialer that allows to Teleport running in single-port mode.
func WithALPNDialer(tlsConfig *tls.Config) DialerOptionFunc {
	return func(options *dialerOptions) {
		options.tlsRoutingEnabled = true
		options.tlsConfig = tlsConfig
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
	proxyAddr := apiclient.GetProxyAddress(addr)

	var options dialerOptions
	for _, opt := range opts {
		opt(&options)
	}

	// If no proxy settings are in environment return regular ssh dialer,
	// otherwise return a proxy dialer.
	if proxyAddr == "" {
		log.Debugf("No proxy set in environment, returning direct dialer.")
		return directDial{
			insecure:          options.insecureSkipTLSVerify,
			tlsRoutingEnabled: options.tlsRoutingEnabled,
			tlsConfig:         options.tlsConfig,
		}
	}
	log.Debugf("Found proxy %q in environment, returning proxy dialer.", proxyAddr)
	return proxyDial{
		proxyHost:         proxyAddr,
		insecure:          options.insecureSkipTLSVerify,
		tlsRoutingEnabled: options.tlsRoutingEnabled,
		tlsConfig:         options.tlsConfig,
	}
}

type DirectDialerOptFunc func(dial *directDial)
