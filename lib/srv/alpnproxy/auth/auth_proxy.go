/*
Copyright 2021 Gravitational, Inc.

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

package alpnproxyauth

import (
	"context"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

type sitesGetter interface {
	GetSites() ([]reversetunnel.RemoteSite, error)
}

// NewAuthProxyDialerService create new instance of AuthProxyDialerService.
func NewAuthProxyDialerService(reverseTunnelServer sitesGetter, localClusterName string, authServer string) *AuthProxyDialerService {
	return &AuthProxyDialerService{
		reverseTunnelServer: reverseTunnelServer,
		localClusterName:    localClusterName,
		authServer:          authServer,
		tracer:              tracing.DefaultProvider().Tracer("alpnAuthDialer"),
	}
}

// AuthProxyDialerService allows dialing local/remote auth service base on SNI value encoded as destination auth
// cluster name and ALPN set to teleport-auth protocol.
type AuthProxyDialerService struct {
	reverseTunnelServer sitesGetter
	localClusterName    string
	authServer          string
	tracer              oteltrace.Tracer
}

func (s *AuthProxyDialerService) HandleConnection(ctx context.Context, conn net.Conn, connInfo alpnproxy.ConnectionInfo) error {
	ctx, span := s.tracer.Start(
		ctx,
		"alpnAuthDialer/HandleConnection",
		oteltrace.WithAttributes(
			attribute.String("sni", connInfo.SNI),
			attribute.StringSlice("alpn", connInfo.ALPN),
		),
	)
	defer span.End()

	defer conn.Close()

	span.AddEvent("getting cluster name")
	clusterName, err := getClusterName(connInfo)
	if err != nil {
		return trace.Wrap(err)
	}

	authConn, err := s.dialAuthServer(ctx, clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer authConn.Close()

	span.AddEvent("proxying connection")
	if err := utils.ProxyConn(ctx, conn, authConn); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getClusterName(info alpnproxy.ConnectionInfo) (string, error) {
	if len(info.ALPN) == 0 {
		return "", trace.NotFound("missing ALPN value")
	}

	protocol := info.ALPN[0]
	if !strings.HasPrefix(protocol, common.ProtocolAuth) {
		return "", trace.BadParameter("auth routing prefix not found")
	}

	cn, err := apiutils.DecodeClusterName(protocol[len(common.ProtocolAuth):])
	return cn, trace.Wrap(err)
}

func (s *AuthProxyDialerService) dialAuthServer(ctx context.Context, clusterNameFromSNI string) (net.Conn, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"alpnAuthDialer/dialAuthServer",
		oteltrace.WithAttributes(
			attribute.String("cluster", clusterNameFromSNI),
		),
	)
	defer span.End()

	if clusterNameFromSNI == s.localClusterName {
		span.AddEvent("dialing local auth server")
		return s.dialLocalAuthServer(ctx)
	}
	if s.reverseTunnelServer != nil {
		span.AddEvent("dialing remote auth server")
		return s.dialRemoteAuthServer(ctx, clusterNameFromSNI)
	}
	return nil, trace.NotFound("auth server for %q cluster name not found", clusterNameFromSNI)
}

func (s *AuthProxyDialerService) dialLocalAuthServer(ctx context.Context) (net.Conn, error) {
	if s.authServer == "" {
		return nil, trace.NotFound("empty auth servers list")
	}

	d := &net.Dialer{
		Timeout: defaults.DefaultDialTimeout,
	}
	conn, err := d.DialContext(ctx, "tcp", s.authServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

func (s *AuthProxyDialerService) dialRemoteAuthServer(ctx context.Context, clusterName string) (net.Conn, error) {
	sites, err := s.reverseTunnelServer.GetSites()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, s := range sites {
		if s.GetName() != clusterName {
			continue
		}
		conn, err := s.DialAuthServer()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}
	return nil, trace.NotFound("cluster name %q not found", clusterName)
}
