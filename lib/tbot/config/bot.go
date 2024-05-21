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

package config

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
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

	// SignX509SVIDs uses the impersonatedClient to call SignX509SVIDs.
	SignX509SVIDs(ctx context.Context, in *machineidv1pb.SignX509SVIDsRequest, opts ...grpc.CallOption) (*machineidv1pb.SignX509SVIDsResponse, error)

	// IsALPNConnUpgradeRequired returns a (possibly cached) test of whether ALPN
	// routing is required.
	IsALPNConnUpgradeRequired(
		ctx context.Context, addr string, insecure bool,
	) (bool, error)
}
