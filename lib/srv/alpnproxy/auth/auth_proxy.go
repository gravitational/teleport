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

package alpnproxyauth

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/defaults"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

type sitesGetter interface {
	GetSites() ([]reversetunnelclient.RemoteSite, error)
}

// NewAuthProxyDialerService create new instance of AuthProxyDialerService.
func NewAuthProxyDialerService(reverseTunnelServer sitesGetter, localClusterName string, authServers []string, proxySigner multiplexer.PROXYHeaderSigner, tracer oteltrace.Tracer) *AuthProxyDialerService {
	return &AuthProxyDialerService{
		reverseTunnelServer: reverseTunnelServer,
		localClusterName:    localClusterName,
		authServers:         authServers,
		proxySigner:         proxySigner,
		tracer:              tracer,
	}
}

// AuthProxyDialerService allows dialing local/remote auth service base on SNI value encoded as destination auth
// cluster name and ALPN set to teleport-auth protocol.
type AuthProxyDialerService struct {
	reverseTunnelServer sitesGetter
	localClusterName    string
	authServers         []string
	proxySigner         multiplexer.PROXYHeaderSigner
	tracer              oteltrace.Tracer
}

func (s *AuthProxyDialerService) HandleConnection(ctx context.Context, conn net.Conn, connInfo alpnproxy.ConnectionInfo) error {
	defer conn.Close()
	clusterName, err := getClusterName(connInfo)
	if err != nil {
		return trace.Wrap(err)
	}

	authConn, err := s.dialAuthServer(ctx, clusterName, conn.RemoteAddr(), conn.LocalAddr())
	if err != nil {
		return trace.Wrap(err)
	}
	defer authConn.Close()

	if err := s.proxyConn(ctx, conn, authConn); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getClusterName(info alpnproxy.ConnectionInfo) (string, error) {
	if len(info.ALPN) == 0 {
		return "", trace.NotFound("missing ALPN value")
	}
	protocol := info.ALPN[0]
	if !strings.HasPrefix(protocol, string(common.ProtocolAuth)) {
		return "", trace.BadParameter("auth routing prefix not found")
	}
	routeToCluster := strings.TrimPrefix(protocol, string(common.ProtocolAuth))
	cn, err := apiutils.DecodeClusterName(routeToCluster)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return cn, nil
}

func (s *AuthProxyDialerService) dialAuthServer(ctx context.Context, clusterNameFromSNI string, clientSrcAddr, clientDstAddr net.Addr) (net.Conn, error) {
	if clusterNameFromSNI == s.localClusterName {
		return s.dialLocalAuthServer(ctx, clientSrcAddr, clientDstAddr)
	}
	if s.reverseTunnelServer != nil {
		return s.dialRemoteAuthServer(ctx, clusterNameFromSNI, clientSrcAddr, clientDstAddr)
	}
	return nil, trace.NotFound("auth server for %q cluster name not found", clusterNameFromSNI)
}

func (s *AuthProxyDialerService) dialLocalAuthServer(ctx context.Context, clientSrcAddr, clientDstAddr net.Addr) (net.Conn, error) {
	ctx, span := s.tracer.Start(ctx, "authProxyDialerService/dialLocalAuthServer",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			attribute.String("src_addr", fmt.Sprintf("%v", clientSrcAddr)),
			attribute.String("dst_addr", fmt.Sprintf("%v", clientDstAddr)),
			attribute.String("cluster_name", s.localClusterName),
		))
	defer span.End()

	if len(s.authServers) == 0 {
		return nil, trace.NotFound("empty auth servers list")
	}

	addr := utils.ChooseRandomString(s.authServers)
	d := &net.Dialer{
		Timeout: defaults.DefaultIOTimeout,
	}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	span.AddEvent("dialed remote server")

	// We'll write signed PROXY header to the outgoing connection to securely
	// propagate observed client ip to the auth server.
	if s.proxySigner != nil && clientSrcAddr != nil && clientDstAddr != nil {
		b, err := s.proxySigner.SignPROXYHeader(clientSrcAddr, clientDstAddr)
		if err != nil {
			return nil, trace.Wrap(err, "could not create signed PROXY header")
		}

		_, err = conn.Write(b)
		if err != nil {
			return nil, trace.Wrap(err, "could not write PROXY line to remote connection")
		}
		span.AddEvent("wrote signed PROXY header")
	}

	return conn, nil
}

func (s *AuthProxyDialerService) dialRemoteAuthServer(ctx context.Context, clusterName string, clientSrcAddr, clientDstAddr net.Addr) (net.Conn, error) {
	_, span := s.tracer.Start(ctx, "authProxyDialerService/dialRemoteAuthServer",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			attribute.String("src_addr", fmt.Sprintf("%v", clientSrcAddr)),
			attribute.String("dst_addr", fmt.Sprintf("%v", clientDstAddr)),
			attribute.String("cluster_name", clusterName),
		))
	defer span.End()

	sites, err := s.reverseTunnelServer.GetSites()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, site := range sites {
		if site.GetName() != clusterName {
			continue
		}
		conn, err := site.DialAuthServer(reversetunnelclient.DialParams{From: clientSrcAddr, OriginalClientDstAddr: clientDstAddr})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}
	return nil, trace.NotFound("cluster name %q not found", clusterName)
}

func (s *AuthProxyDialerService) proxyConn(ctx context.Context, upstreamConn, downstreamConn net.Conn) error {
	errC := make(chan error, 2)
	go func() {
		defer upstreamConn.Close()
		defer downstreamConn.Close()
		_, err := io.Copy(downstreamConn, upstreamConn)
		errC <- trace.Wrap(err)

	}()
	go func() {
		defer upstreamConn.Close()
		defer downstreamConn.Close()
		_, err := io.Copy(upstreamConn, downstreamConn)
		errC <- trace.Wrap(err)
	}()
	var errs []error
	for range 2 {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case err := <-errC:
			if err != nil && !utils.IsOKNetworkError(err) {
				errs = append(errs, err)
			}
		}
	}
	return trace.NewAggregate(errs...)
}
