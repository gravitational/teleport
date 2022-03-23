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
	"io"
	"math/rand"
	"net"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

type sitesGetter interface {
	GetSites() ([]reversetunnel.RemoteSite, error)
}

type authGetter interface {
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
	GetAuthServers() ([]types.Server, error)
}

// NewAuthProxyDialerService create new instance of AuthProxyDialerService.
func NewAuthProxyDialerService(reverseTunnelServer sitesGetter, accessPoint authGetter) *AuthProxyDialerService {
	return &AuthProxyDialerService{
		reverseTunnelServer: reverseTunnelServer,
		accessPoint:         accessPoint,
	}
}

// AuthProxyDialerService allows dialing local/remote auth service base on SNI value encoded as destination auth
// cluster name and ALPN set to teleport-auth protocol.
type AuthProxyDialerService struct {
	reverseTunnelServer sitesGetter
	accessPoint         authGetter
}

func (s *AuthProxyDialerService) HandleConnection(ctx context.Context, conn net.Conn, connInfo alpnproxy.ConnectionInfo) error {
	defer conn.Close()
	clusterName, err := getClusterName(connInfo)
	if err != nil {
		return trace.Wrap(err)
	}
	authConn, err := s.dialAuthServer(ctx, clusterName)
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

func (s *AuthProxyDialerService) dialAuthServer(ctx context.Context, clusterNameFromSNI string) (net.Conn, error) {
	clusterName, err := s.accessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if clusterName.GetClusterName() == clusterNameFromSNI {
		return s.dialLocalAuthServer(ctx)
	}
	if s.reverseTunnelServer != nil {
		return s.dialRemoteAuthServer(ctx, clusterNameFromSNI)
	}
	return nil, trace.NotFound("auth server for %q cluster name not found", clusterNameFromSNI)
}

func (s *AuthProxyDialerService) dialLocalAuthServer(ctx context.Context) (net.Conn, error) {
	authServers, err := s.accessPoint.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(authServers) == 0 {
		return nil, trace.NotFound("empty auth servers list")
	}
	//TODO(smallinksy) Better support for HA. Add dial retry on auth network errors.
	authServerIndex := rand.Intn(len(authServers))
	conn, err := net.Dial("tcp", authServers[authServerIndex].GetAddr())
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
	for i := 0; i < 2; i++ {
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
