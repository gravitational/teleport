// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package ping

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	pingv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ping/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/web"
)

type Service struct {
	pingv1pb.UnimplementedPingServiceServer
	cfg Config
}

type Config struct {
	ProxySettings web.ProxySettingsGetter
	ProxyClient   authclient.ClientI
}

func NewService(cfg Config) (*Service, error) {
	switch {
	case cfg.ProxySettings == nil:
		return nil, trace.BadParameter("ProxySettings is missing")
	case cfg.ProxyClient == nil:
		return nil, trace.BadParameter("ProxyClient is missing")
	}

	return &Service{cfg: cfg}, nil
}

func (s *Service) Find(ctx context.Context, req *pingv1pb.FindRequest) (*pingv1pb.FindResponse, error) {
	proxyConfig, err := s.cfg.ProxySettings.GetProxySettings(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := s.cfg.ProxyClient.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pingv1pb.FindResponse{
		Proxy: &pingv1pb.ProxySettings{
			TlsRoutingEnabled: proxyConfig.TLSRoutingEnabled,
			SshProxySettings: &pingv1pb.SSHProxySettings{
				ListenAddr:       proxyConfig.SSH.ListenAddr,
				TunnelListenAddr: proxyConfig.SSH.TunnelListenAddr,
				WebListenAddr:    proxyConfig.SSH.WebListenAddr,
				PublicAddr:       proxyConfig.SSH.PublicAddr,
				SshPublicAddr:    proxyConfig.SSH.SSHPublicAddr,
				TunnelPublicAddr: proxyConfig.SSH.TunnelPublicAddr,
			},
		},
		ServerVersion:    teleport.Version,
		MinClientVersion: teleport.MinClientSemVer().String(),
		ClusterName:      clusterName.GetClusterName(),
		Edition:          modules.GetModules().BuildType(),
		Fips:             modules.IsBoringBinary(),
	}, nil
}
