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
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
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
	// ReloadCert loads the key pair from cfg.CertPath & cfg.KeyPath and
	// updates the cert of the running local proxy.
	ReloadCert() error
	// CLICommand returns a command which launches a CLI client pointed at the gateway.
	CLICommand() (*api.GatewayCLICommand, error)

	URI() uri.ResourceURI
	TargetURI() uri.ResourceURI
	TargetName() string
	Protocol() string
	TargetUser() string
	TargetSubresourceName() string
	SetTargetSubresourceName(value string)
	Log() *logrus.Entry
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
}
