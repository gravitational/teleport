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
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	pingv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ping/v1"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.Component("ping"))

type FindResponse struct {
	Proxy            *ProxySettings
	ServerVersion    string
	MinClientVersion string
	ClusterName      string
	Edition          string
	Fips             bool
}

type ProxySettings struct {
	SSHProxySettings  *SSHProxySettings
	TLSRoutingEnabled bool
}

type SSHProxySettings struct {
	ListenAddr       string
	TunnelListenAddr string
	WebListenAddr    string
	PublicAddr       string
	SSHPublicAddr    string
	TunnelPublicAddr string
}

type Finder struct{}

func NewFinder() *Finder {
	return &Finder{}
}

func (f *Finder) Find(proxyServer string) (*FindResponse, error) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	grpcConn, err := proxyinsecureclient.NewConnection(
		context.TODO(),
		proxyinsecureclient.ConnectionConfig{
			ProxyServer: proxyServer,
			Clock:       clockwork.NewRealClock(),
			Log:         logger,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pingClient := pingv1pb.NewPingServiceClient(grpcConn)
	res, err := pingClient.Find(ctx, &pingv1pb.FindRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &FindResponse{
		Proxy: &ProxySettings{
			SSHProxySettings: &SSHProxySettings{
				ListenAddr:       res.Proxy.SshProxySettings.ListenAddr,
				TunnelListenAddr: res.Proxy.SshProxySettings.TunnelListenAddr,
				WebListenAddr:    res.Proxy.SshProxySettings.WebListenAddr,
				PublicAddr:       res.Proxy.SshProxySettings.PublicAddr,
				SSHPublicAddr:    res.Proxy.SshProxySettings.SshPublicAddr,
				TunnelPublicAddr: res.Proxy.SshProxySettings.SshPublicAddr,
			},
			TLSRoutingEnabled: res.Proxy.TlsRoutingEnabled,
		},
		ServerVersion:    res.ServerVersion,
		MinClientVersion: res.MinClientVersion,
		ClusterName:      res.ClusterName,
		Edition:          res.Edition,
		Fips:             res.Fips,
	}, nil
}
