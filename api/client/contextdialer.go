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

package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gravitational/trace"
	mplex "github.com/libp2p/go-mplex"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/utils/sshutils"
)

// ContextDialer represents network dialer interface that uses context
type ContextDialer interface {
	// DialContext is a function that dials the specified address
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// ContextDialerFunc is a function wrapper that implements the ContextDialer interface.
type ContextDialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// DialContext is a function that dials to the specified address
func (f ContextDialerFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f(ctx, network, addr)
}

// newDirectDialer makes a new dialer to connect directly to an Auth server.
func newDirectDialer(keepAlivePeriod, dialTimeout time.Duration) ContextDialer {
	return &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: keepAlivePeriod,
	}
}

// NewDialer makes a new dialer that connects to an Auth server either directly or via an HTTP proxy, depending
// on the environment.
func NewDialer(keepAlivePeriod, dialTimeout time.Duration) ContextDialer {
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := newDirectDialer(keepAlivePeriod, dialTimeout)
		if proxyURL := proxy.GetProxyURL(addr); proxyURL != nil {
			return DialProxyWithDialer(ctx, proxyURL, addr, dialer)
		}
		return dialer.DialContext(ctx, network, addr)
	})
}

func NewDialer2(keepAlivePeriod, dialTimeout time.Duration) ContextDialer {
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := Upgrade(ctx, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, err
	})
}

// NewProxyDialer makes a dialer to connect to an Auth server through the SSH reverse tunnel on the proxy.
// The dialer will ping the web client to discover the tunnel proxy address on each dial.
func NewProxyDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration, discoveryAddr string, insecure bool) ContextDialer {
	dialer := newTunnelDialer(ssh, keepAlivePeriod, dialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, _ string) (conn net.Conn, err error) {
		tunnelAddr, err := webclient.GetTunnelAddr(
			&webclient.Config{Context: ctx, ProxyAddr: discoveryAddr, Insecure: insecure})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		conn, err = dialer.DialContext(ctx, network, tunnelAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	})
}

// newTunnelDialer makes a dialer to connect to an Auth server through the SSH reverse tunnel on the proxy.
func newTunnelDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration) ContextDialer {
	dialer := newDirectDialer(keepAlivePeriod, dialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		conn, err = dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sconn, err := sshConnect(ctx, conn, ssh, dialTimeout, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return sconn, nil
	})
}

// newTLSRoutingTunnelDialer makes a reverse tunnel TLS Routing dialer to connect to an Auth server
// through the SSH reverse tunnel on the proxy.
func newTLSRoutingTunnelDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration, discoveryAddr string, insecure bool) ContextDialer {
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		tunnelAddr, err := webclient.GetTunnelAddr(
			&webclient.Config{Context: ctx, ProxyAddr: discoveryAddr, Insecure: insecure})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if env := os.Getenv("ALB_TEST"); env != "" {
			conn, err = Upgrade(ctx, tunnelAddr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			host, _, err := webclient.ParseHostPort(tunnelAddr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			tlsConn := tls.Client(conn, &tls.Config{
				NextProtos:         []string{constants.ALPNSNIProtocolReverseTunnel},
				InsecureSkipVerify: insecure,
				ServerName:         host,
			})
			if err := tlsConn.Handshake(); err != nil {
				return nil, trace.Wrap(err)
			}

			sconn, err := sshConnect(ctx, tlsConn, ssh, dialTimeout, tunnelAddr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return sconn, nil
		} else {
			dialer := &net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: keepAlivePeriod,
			}
			conn, err = dialer.DialContext(ctx, network, tunnelAddr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			host, _, err := webclient.ParseHostPort(tunnelAddr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			tlsConn := tls.Client(conn, &tls.Config{
				NextProtos:         []string{constants.ALPNSNIProtocolReverseTunnel},
				InsecureSkipVerify: insecure,
				ServerName:         host,
			})
			sconn, err := sshConnect(ctx, tlsConn, ssh, dialTimeout, tunnelAddr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return sconn, nil

		}
	})
}

func Upgrade(ctx context.Context, proxyAddr string) (net.Conn, error) {
	// TODO figure out proper ctx
	ctx = context.Background()
	// TODO detect if proxy is behind ALB
	// TODO handle insecure
	conn, err := tls.Dial("tcp", proxyAddr, &tls.Config{})
	if err != nil {
		return nil, trace.Wrap(err)

	}
	u := url.URL{
		Host:   proxyAddr,
		Scheme: "https",
		Path:   fmt.Sprintf("/webapi/connectionupgrate"),
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Add("Upgrade", "custom")
	req.Header.Add("Connection", "upgrade")

	if err = req.Write(conn); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, trace.BadParameter("failed to switch Protocols %v", resp.StatusCode)
	}

	// --- Multiplex TODO move logic to a function
	// TODO use named stream
	multiplexConn, err := mplex.NewMultiplex(conn, true, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pingStream, err := multiplexConn.NewStream(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dataStream, err := multiplexConn.NewStream(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// ----- Handle ping
	go func() {
		defer multiplexConn.Close()
		defer pingStream.Close()
		for {
			select {
			case <-time.After(time.Second * 10):
				_, err := pingStream.Write([]byte("ping"))
				if err != nil {
					logrus.Infof("-->> %v error sending ping %v", time.Now(), err)
					return
				}

			case <-ctx.Done():
				logrus.Infof("-->> %v ping stream closed", time.Now())
				return
			}
		}
	}()
	go func() {
		defer multiplexConn.Close()
		defer pingStream.Close()
		for {
			buf := make([]byte, 256)
			n, err := pingStream.Read(buf)
			if err != nil {
				logrus.Infof("-->> %v error reading pong %v", time.Now(), err)
				return
			}
			logrus.Infof("-->> %v received pong %v", time.Now(), string(buf[:n]))
		}
	}()
	return NewMultiplexConn(conn, dataStream), nil
}

// sshConnect upgrades the underling connection to ssh and connects to the Auth service.
func sshConnect(ctx context.Context, conn net.Conn, ssh ssh.ClientConfig, dialTimeout time.Duration, addr string) (net.Conn, error) {
	ssh.Timeout = dialTimeout
	sconn, err := tracessh.NewClientConnWithDeadline(ctx, conn, addr, &ssh)
	if err != nil {
		return nil, trace.NewAggregate(err, conn.Close())
	}

	// Build a net.Conn over the tunnel. Make this an exclusive connection:
	// close the net.Conn as well as the channel upon close.
	conn, _, err = sshutils.ConnectProxyTransport(sconn.Conn, &sshutils.DialReq{
		Address: constants.RemoteAuthServer,
	}, true)
	if err != nil {
		return nil, trace.NewAggregate(err, sconn.Close())
	}
	return conn, nil
}

func NewMultiplexConn(baseConn net.Conn, stream *mplex.Stream) net.Conn {
	return &multiplexConn{stream, baseConn}
}

type multiplexConn struct {
	stream   *mplex.Stream
	baseConn net.Conn
}

func (c *multiplexConn) Read(p []byte) (int, error)         { return c.stream.Read(p) }
func (c *multiplexConn) Write(p []byte) (int, error)        { return c.stream.Write(p) }
func (c *multiplexConn) LocalAddr() net.Addr                { return c.baseConn.LocalAddr() }
func (c *multiplexConn) RemoteAddr() net.Addr               { return c.baseConn.RemoteAddr() }
func (c *multiplexConn) SetDeadline(t time.Time) error      { return c.stream.SetDeadline(t) }
func (c *multiplexConn) SetReadDeadline(t time.Time) error  { return c.stream.SetReadDeadline(t) }
func (c *multiplexConn) SetWriteDeadline(t time.Time) error { return c.stream.SetWriteDeadline(t) }
func (c *multiplexConn) Close() error {
	return trace.NewAggregate(
		c.stream.Close(),
		c.baseConn.Close(),
	)
}
