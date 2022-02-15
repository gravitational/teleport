// Copyright 2022 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	clientapi "github.com/gravitational/teleport/api/client/proto"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

const (
	errorTunnelNotFound = "NOT_FOUND"
)

// ClientConfig configures a Client instance.
type ClientConfig struct {
	// AccessCache is the caching client connected to the proxy client.
	AccessCache auth.AccessCache
	// TLSConfig is the proxy client TLS configuration.
	TLSConfig *tls.Config
	// Log is the proxy client logger.
	Log logrus.FieldLogger
}

// checkAndSetDefaults checks and sets default values
func (c *ClientConfig) checkAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.New()
	}

	c.Log = c.Log.WithField(
		trace.Component,
		teleport.Component(teleport.ComponentProxyPeer),
	)

	if c.AccessCache == nil {
		return trace.BadParameter("missing access cache")
	}

	if c.TLSConfig == nil {
		return trace.BadParameter("missing tls config")
	}

	if len(c.TLSConfig.Certificates) == 0 {
		return trace.BadParameter("missing tls certificate")
	}

	c.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	c.TLSConfig.GetConfigForClient = getConfigForClient(c.TLSConfig, c.AccessCache, c.Log)

	return nil
}

// Client is a peer proxy service client using grpc and tls.
type Client struct {
	sync.RWMutex

	config  ClientConfig
	conns   map[string]*grpc.ClientConn
	metrics *clientMetrics
}

type connections struct{}

// NewClient creats a new peer proxy client.
func NewClient(config ClientConfig) (*Client, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	metrics, err := newClientMetrics()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		config:  config,
		conns:   make(map[string]*grpc.ClientConn),
		metrics: metrics,
	}, nil
}

// Dial dials a node through a peer proxy.
func (c *Client) Dial(
	ctx context.Context,
	proxyAddr string,
	src net.Addr,
	dst net.Addr,
	nodeID string,
	tunnelType types.TunnelType,
) (net.Conn, error) {
	grpcConn, err := c.dial(ctx, proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := clientapi.NewProxyServiceClient(grpcConn)
	stream, err := client.DialNode(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// send dial request as the first frame
	if err = stream.Send(&clientapi.Frame{
		Message: &clientapi.Frame_DialRequest{
			DialRequest: &clientapi.DialRequest{
				NodeID:     nodeID,
				TunnelType: tunnelType,
				Source: &clientapi.NetAddr{
					Addr:    src.String(),
					Network: src.Network(),
				},
				Destination: &clientapi.NetAddr{
					Addr:    dst.String(),
					Network: dst.Network(),
				},
			},
		},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	conn := newStreamConn(stream, src, dst)
	go conn.start()

	return conn, nil
}

// Close closes all existing client connections.
func (c *Client) Close() error {
	c.Lock()
	defer c.Unlock()

	var errs []error
	for _, conn := range c.conns {
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

// dial returns an existing grpc.ClientConn or initializes a new connection to
// proxyAddr otherwise.
func (c *Client) dial(ctx context.Context, proxyAddr string) (*grpc.ClientConn, error) {
	conn, err := c.getConnection(proxyAddr)
	if err == nil {
		c.config.Log.Debugf("found existing connection to proxy %+v", proxyAddr)
		return conn, nil
	} else if !trace.IsNotFound(err) {
		c.config.Log.Debugf("error checking for existing connections to proxy %+v", proxyAddr)
		return nil, trace.Wrap(err)
	}

	c.config.Log.Debugf("establishing new connection to proxy %+v", proxyAddr)

	conn, err = c.newConnection(ctx, proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

// getConnection checks and returns an existing grpc.ClientConn to
// proxyAddr from memory.
func (c *Client) getConnection(proxyAddr string) (*grpc.ClientConn, error) {
	c.RLock()
	defer c.RUnlock()

	conn, ok := c.conns[proxyAddr]
	if !ok {
		c.metrics.tunnelErrorCounter.WithLabelValues(errorTunnelNotFound).Inc()
		return nil, trace.NotFound("no existing peer proxy connection to %+v", proxyAddr)
	}

	state := conn.GetState()
	switch state {
	case connectivity.TransientFailure, connectivity.Shutdown:
		c.metrics.tunnelErrorCounter.WithLabelValues(state.String()).Inc()
		return nil, trace.NotFound("found connection in %+v state", state.String())
	}

	return conn, nil
}

// newConnection dials a new connection to proxyAddr.
func (c *Client) newConnection(ctx context.Context, proxyAddr string) (*grpc.ClientConn, error) {
	c.Lock()
	defer c.Unlock()

	transportCreds := newProxyCredentials(credentials.NewTLS(c.config.TLSConfig))
	conn, err := grpc.DialContext(
		ctx,
		proxyAddr,
		grpc.WithTransportCredentials(transportCreds),
		grpc.WithChainStreamInterceptor(metadata.StreamClientInterceptor, streamClientInterceptor(c.metrics)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    peerKeepAlive,
			Timeout: peerTimeout,
		}),
	)
	if err != nil {
		c.metrics.dialErrorCounter.Inc()
		return nil, trace.Wrap(err)
	}

	c.conns[proxyAddr] = conn
	return conn, nil
}
