/*
Copyright 2023 Gravitational, Inc.

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

package gateway

import (
	"crypto/tls"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
)

// RouteToDatabase returns tlsca.RouteToDatabase based on the config of the gateway.
//
// The tlsca.RouteToDatabase.Database field is skipped, as it's an optional field and gateways can
// change their Config.TargetSubresourceName at any moment.
func (g *Gateway) RouteToDatabase() tlsca.RouteToDatabase {
	return g.cfg.RouteToDatabase()
}

func (g *Gateway) makeLocalProxyForDB(listener net.Listener) error {
	tlsCert, err := keys.LoadX509KeyPair(g.cfg.CertPath, g.cfg.KeyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := checkCertSubject(tlsCert, g.RouteToDatabase()); err != nil {
		return trace.Wrap(err,
			"database certificate check failed, try restarting the database connection")
	}

	localProxyConfig := alpnproxy.LocalProxyConfig{
		InsecureSkipVerify:      g.cfg.Insecure,
		RemoteProxyAddr:         g.cfg.WebProxyAddr,
		Listener:                listener,
		ParentContext:           g.closeContext,
		Certs:                   []tls.Certificate{tlsCert},
		Clock:                   g.cfg.Clock,
		ALPNConnUpgradeRequired: g.cfg.TLSRoutingConnUpgradeRequired,
	}

	if g.cfg.OnExpiredCert != nil {
		localProxyConfig.Middleware = &dbMiddleware{
			log:           g.cfg.Log,
			dbRoute:       g.cfg.RouteToDatabase(),
			onExpiredCert: g.onExpiredCert,
		}
	}

	localProxy, err := alpnproxy.NewLocalProxy(localProxyConfig,
		alpnproxy.WithDatabaseProtocol(g.cfg.Protocol),
		alpnproxy.WithClusterCAsIfConnUpgrade(g.closeContext, g.cfg.RootClusterCACertPoolFunc),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	g.localProxy = localProxy
	g.onNewCert = g.setDBCert
	return nil
}

func (g *Gateway) setDBCert(newCert tls.Certificate) error {
	if err := checkCertSubject(newCert, g.RouteToDatabase()); err != nil {
		return trace.Wrap(err,
			"database certificate check failed, try restarting the database connection")
	}

	g.localProxy.SetCerts([]tls.Certificate{newCert})
	return nil
}
