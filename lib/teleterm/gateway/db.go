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

package gateway

import (
	"context"
	"crypto/tls"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
)

type db struct {
	*base
}

// RouteToDatabase returns tlsca.RouteToDatabase based on the config of the gateway.
//
// The tlsca.RouteToDatabase.Database field is skipped, as it's an optional field and gateways can
// change their Config.TargetSubresourceName at any moment.
func (d *db) RouteToDatabase() tlsca.RouteToDatabase {
	return d.cfg.RouteToDatabase()
}

func makeDatabaseGateway(cfg Config) (Database, error) {
	base, err := newBase(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	d := &db{base}

	listener, err := d.cfg.makeListener()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	middleware := &dbMiddleware{
		log: d.cfg.Log,
		onExpiredCert: func(ctx context.Context) (tls.Certificate, error) {
			cert, err := d.cfg.OnExpiredCert(ctx, d)
			return cert, trace.Wrap(err)
		},
	}

	localProxyConfig := alpnproxy.LocalProxyConfig{
		InsecureSkipVerify:      d.cfg.Insecure,
		RemoteProxyAddr:         d.cfg.WebProxyAddr,
		Listener:                listener,
		ParentContext:           d.closeContext,
		Clock:                   d.cfg.Clock,
		ALPNConnUpgradeRequired: d.cfg.TLSRoutingConnUpgradeRequired,
	}

	localProxy, err := alpnproxy.NewLocalProxy(localProxyConfig,
		alpnproxy.WithDatabaseProtocol(d.cfg.Protocol),
		alpnproxy.WithClientCert(d.cfg.Cert),
		alpnproxy.WithClusterCAsIfConnUpgrade(d.closeContext, d.cfg.RootClusterCACertPoolFunc),
		alpnproxy.WithMiddleware(middleware),
	)
	if err != nil {
		return nil, trace.NewAggregate(err, listener.Close())
	}

	d.localProxy = localProxy
	return d, nil
}
