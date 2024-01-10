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
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

type app struct {
	*base
}

// LocalProxyURL returns the URL of the local proxy.
func (a *app) LocalProxyURL() string {
	proxyURL := url.URL{
		Scheme: strings.ToLower(a.Protocol()),
		Host:   a.LocalAddress() + ":" + a.LocalPort(),
	}
	return proxyURL.String()
}

func makeAppGateway(cfg Config) (Gateway, error) {
	base, err := newBase(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a := &app{base}

	listener, err := a.cfg.makeListener()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(a.closeContext, a.cfg, listener),
		alpnproxy.WithALPNProtocol(alpnProtocolForApp(a.cfg.Protocol)),
		alpnproxy.WithClientCerts(a.cfg.Cert),
		alpnproxy.WithClusterCAsIfConnUpgrade(a.closeContext, a.cfg.RootClusterCACertPoolFunc),
	)
	if err != nil {
		return nil, trace.NewAggregate(err, listener.Close())
	}

	a.localProxy = lp
	return a, nil
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
	if protocol == types.ApplicationProtocolTCP {
		return alpncommon.ProtocolTCP
	}
	return alpncommon.ProtocolHTTP
}
