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
	"crypto/tls"
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

	middleware := &appMiddleware{
		log: a.cfg.Log,
		onExpiredCert: func(ctx context.Context) (tls.Certificate, error) {
			cert, err := a.cfg.OnExpiredCert(ctx, a)
			return cert, trace.Wrap(err)
		},
	}

	localProxyConfig := alpnproxy.LocalProxyConfig{
		InsecureSkipVerify:      a.cfg.Insecure,
		RemoteProxyAddr:         a.cfg.WebProxyAddr,
		Listener:                listener,
		ParentContext:           a.closeContext,
		Clock:                   a.cfg.Clock,
		ALPNConnUpgradeRequired: a.cfg.TLSRoutingConnUpgradeRequired,
	}

	lp, err := alpnproxy.NewLocalProxy(
		localProxyConfig,
		alpnproxy.WithALPNProtocol(alpnProtocolForApp(a.cfg.Protocol)),
		alpnproxy.WithClientCerts(a.cfg.Cert),
		alpnproxy.WithClusterCAsIfConnUpgrade(a.closeContext, a.cfg.RootClusterCACertPoolFunc),
		alpnproxy.WithMiddleware(middleware),
	)
	if err != nil {
		return nil, trace.NewAggregate(err, listener.Close())
	}

	a.localProxy = lp
	return a, nil
}

func alpnProtocolForApp(protocol string) alpncommon.Protocol {
	if protocol == types.ApplicationProtocolTCP {
		return alpncommon.ProtocolTCP
	}
	return alpncommon.ProtocolHTTP
}
