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

package common

import (
	"context"
	"fmt"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
)

// localProxyApp is a generic app that can start local proxies.
type localProxyApp struct {
	tc                 *client.TeleportClient
	appName            string
	insecure           bool
	localALPNProxyPort string

	// localALPNProxy created with StartLocalProxies and closed with Close.
	localALPNProxy *alpnproxy.LocalProxy
}

// newLocalProxyApp creates a new generic app.
func newLocalProxyApp(tc *client.TeleportClient, appName string, localALPNProxyPort string, insecure bool) (*localProxyApp, error) {
	return &localProxyApp{
		tc:                 tc,
		appName:            appName,
		localALPNProxyPort: localALPNProxyPort,
		insecure:           insecure,
	}, nil
}

// StartLocalProxies sets up local proxies for serving app clients.
func (a *localProxyApp) StartLocalProxies(ctx context.Context, opts ...alpnproxy.LocalProxyConfigOpt) error {
	if err := a.startLocalALPNProxy(ctx, opts...); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Close makes all necessary close calls.
func (a *localProxyApp) Close() error {
	var errs []error
	if a.localALPNProxy != nil {
		errs = append(errs, a.localALPNProxy.Close())
	}
	return trace.NewAggregate(errs...)
}

// startLocalALPNProxy starts the local ALPN proxy.
func (a *localProxyApp) startLocalALPNProxy(ctx context.Context, opts ...alpnproxy.LocalProxyConfigOpt) error {
	appCert, err := loadAppCertificateWithAppLogin(ctx, a.tc, a.appName)
	if err != nil {
		return trace.Wrap(err)
	}

	listenAddr := "localhost:0"
	if a.localALPNProxyPort != "" {
		listenAddr = fmt.Sprintf("127.0.0.1:%s", a.localALPNProxyPort)
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	a.localALPNProxy, err = alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(ctx, a.tc, listener, a.insecure),
		append(opts,
			alpnproxy.WithClientCerts(appCert),
			alpnproxy.WithClusterCAsIfConnUpgrade(ctx, a.tc.RootClusterCACertPool),
		)...,
	)
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}

	fmt.Printf("Proxying connections to %s on %v\n", a.appName, a.localALPNProxy.GetAddr())
	if a.localALPNProxyPort == "" {
		fmt.Println("To avoid port randomization, you can choose the listening port using the --port flag.")
	}

	go func() {
		if err = a.localALPNProxy.Start(ctx); err != nil {
			log.WithError(err).Errorf("Failed to start local ALPN proxy.")
		}
	}()
	return nil
}
