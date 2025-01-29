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
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Gateway is a interface defines all gateway functions.
type Gateway interface {
	// Serve starts the underlying ALPN proxy. Blocks until closeContext is
	// canceled.
	Serve() error
	// Close terminates gateway connection.
	Close() error

	URI() uri.ResourceURI
	TargetURI() uri.ResourceURI
	TargetName() string
	Protocol() string
	TargetUser() string
	TargetSubresourceName() string
	SetTargetSubresourceName(value string)
	Log() *slog.Logger
	// LocalAddress returns the local host in the net package terms (localhost or 127.0.0.1, depending
	// on the platform).
	LocalAddress() string
	LocalPort() string
	LocalPortInt() int
}

// AsDatabase converts provided gateway to a database gateway.
func AsDatabase(g Gateway) (Database, error) {
	if db, ok := g.(Database); ok {
		return db, nil
	}
	return nil, trace.BadParameter("expecting database gateway but got %T", g)
}

// AsKube converts provided gateway to a kube gateway.
func AsKube(g Gateway) (Kube, error) {
	if kube, ok := g.(Kube); ok {
		return kube, nil
	}
	return nil, trace.BadParameter("expecting kube gateway but got %T", g)
}

// AsApp converts provided gateway to a kube gateway.
func AsApp(g Gateway) (App, error) {
	if app, ok := g.(App); ok {
		return app, nil
	}
	return nil, trace.BadParameter("expecting app gateway but got %T", g)
}

// Database defines a database gateway.
type Database interface {
	Gateway

	// RouteToDatabase returns tlsca.RouteToDatabase based on the config of the gateway.
	//
	// The tlsca.RouteToDatabase.Database field is skipped, as it's an optional field and gateways can
	// change their Config.TargetSubresourceName at any moment.
	RouteToDatabase() tlsca.RouteToDatabase
}

// Kube defines a kube gateway.
type Kube interface {
	Gateway

	// KubeconfigPath returns the path to the kubeconfig used to connect the
	// local proxy.
	KubeconfigPath() string
	// ClearCerts clears the local proxy middleware certs.
	// It will try to reissue them when a new request comes in.
	ClearCerts()
}

// App defines an app gateway.
type App interface {
	Gateway
}
