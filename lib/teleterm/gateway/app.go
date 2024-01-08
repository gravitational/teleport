// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package gateway

import (
	"context"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

func makeAppGateway(cfg Config) (Gateway, error) {
	base, err := newBase(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := base.cfg.makeListener()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(base.closeContext, base.cfg, listener),
		alpnproxy.WithALPNProtocol(alpnProtocolForApp(base.cfg.Protocol)),
		alpnproxy.WithClientCerts(base.cfg.Cert),
		alpnproxy.WithClusterCAsIfConnUpgrade(base.closeContext, base.cfg.RootClusterCACertPoolFunc),
	)
	if err != nil {
		return nil, trace.NewAggregate(err, listener.Close())
	}

	base.localProxy = lp
	return base, nil
}

func makeBasicLocalProxyConfig(ctx context.Context, cfg *Config, listener net.Listener) alpnproxy.LocalProxyConfig {
	return alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:         cfg.WebProxyAddr,
		InsecureSkipVerify:      cfg.Insecure,
		ParentContext:           ctx,
		Listener:                listener,
		ALPNConnUpgradeRequired: cfg.TLSRoutingConnUpgradeRequired,
	}
}

func alpnProtocolForApp(protocol string) alpncommon.Protocol {
	if protocol == types.ProtocolTCP {
		return alpncommon.ProtocolTCP
	}
	return alpncommon.ProtocolHTTP
}
