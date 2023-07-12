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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys"
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

	tlsCert, err := keys.LoadX509KeyPair(d.cfg.CertPath, d.cfg.KeyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkCertSubject(tlsCert, d.RouteToDatabase()); err != nil {
		return nil, trace.Wrap(err,
			"database certificate check failed, try restarting the database connection")
	}

	listener, err := d.cfg.makeListener()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localProxyConfig := alpnproxy.LocalProxyConfig{
		InsecureSkipVerify:      d.cfg.Insecure,
		RemoteProxyAddr:         d.cfg.WebProxyAddr,
		Listener:                listener,
		ParentContext:           d.closeContext,
		Certs:                   []tls.Certificate{tlsCert},
		Clock:                   d.cfg.Clock,
		ALPNConnUpgradeRequired: d.cfg.TLSRoutingConnUpgradeRequired,
	}

	if d.cfg.OnExpiredCert != nil {
		localProxyConfig.Middleware = &dbMiddleware{
			log:           d.cfg.Log,
			dbRoute:       d.cfg.RouteToDatabase(),
			onExpiredCert: d.onExpiredCert,
		}
	}

	localProxy, err := alpnproxy.NewLocalProxy(localProxyConfig,
		alpnproxy.WithDatabaseProtocol(d.cfg.Protocol),
		alpnproxy.WithClusterCAsIfConnUpgrade(d.closeContext, d.cfg.RootClusterCACertPoolFunc),
	)
	if err != nil {
		return nil, trace.NewAggregate(err, listener.Close())
	}

	d.localProxy = localProxy
	d.onNewCertFuncs = append(d.onNewCertFuncs, d.setDBCert)
	return d, nil
}

func (d *db) setDBCert(newCert tls.Certificate) error {
	if err := checkCertSubject(newCert, d.RouteToDatabase()); err != nil {
		return trace.Wrap(err,
			"database certificate check failed, try restarting the database connection")
	}

	d.localProxy.SetCerts([]tls.Certificate{newCert})
	return nil
}
