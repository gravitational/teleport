/*
Copyright 2022 Gravitational, Inc.

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

package config

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// provider is an interface that allows Templates to fetch information they
// need to render from the bot hosting the template rendering.
type provider interface {
	// AuthPing pings the auth server and returns the (possibly cached) response.
	AuthPing(ctx context.Context) (*proto.PingResponse, error)

	// ProxyPing returns a (possibly cached) ping response from the Teleport proxy.
	// Note that it relies on the auth server being configured with a sane proxy
	// public address.
	ProxyPing(ctx context.Context) (*webclient.PingResponse, error)

	// GetCertAuthorities returns the possibly cached CAs of the given type and
	// requests them from the server if unavailable.
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType) ([]types.CertAuthority, error)

	// Config returns the current bot config
	Config() *BotConfig

	// GenerateHostCert uses the impersonatedClient to call GenerateHostCert.
	GenerateHostCert(ctx context.Context, key []byte, hostID, nodeName string, principals []string, clusterName string, role types.SystemRole, ttl time.Duration) ([]byte, error)

	// GetRemoteClusters uses the impersonatedClient to call GetRemoteClusters.
	GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error)

	// GetCertAuthority uses the impersonatedClient to call GetCertAuthority.
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
}
