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
	"github.com/sirupsen/logrus"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Gateway is a interface defines all gateway functions.
type Gateway interface {
	GatewayReader
	GatewaySetter

	Serve() error
	Close() error
	ReloadCert() error
}

// GatewayReader defines getter functions for the gateway.
type GatewayReader interface {
	URI() uri.ResourceURI
	TargetURI() uri.ResourceURI
	TargetName() string
	Protocol() string
	TargetUser() string
	TargetSubresourceName() string
	Log() *logrus.Entry
	LocalAddress() string
	LocalPort() string
	LocalPortInt() int
	CLICommand() (*api.GatewayCLICommand, error)

	// Database-specific functions.
	RouteToDatabase() tlsca.RouteToDatabase

	// Kube-specific functions.
	KubeconfigPath() string
}

// GatewaySetter defines setter functions for the gateway.
type GatewaySetter interface {
	SetURI(newURI uri.ResourceURI)
	SetTargetSubresourceName(value string)
}
