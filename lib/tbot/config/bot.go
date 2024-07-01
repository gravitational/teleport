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

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
)

// provider is an interface that allows Templates to fetch information they
// need to render from the bot hosting the template rendering.
type provider interface {
	// ProxyPing returns a (possibly cached) ping response from the Teleport proxy.
	// Note that it relies on the auth server being configured with a sane proxy
	// public address.
	ProxyPing(ctx context.Context) (*webclient.PingResponse, error)

	// GetCertAuthorities returns the possibly cached CAs of the given type and
	// requests them from the server if unavailable.
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType) ([]types.CertAuthority, error)

	// Config returns the current bot config
	Config() *BotConfig

	// GetRemoteClusters uses the impersonatedClient to call GetRemoteClusters.
	GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error)

	// GetCertAuthority uses the impersonatedClient to call GetCertAuthority.
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// IsALPNConnUpgradeRequired returns a (possibly cached) test of whether ALPN
	// routing is required.
	IsALPNConnUpgradeRequired(
		ctx context.Context, addr string, insecure bool,
	) (bool, error)
}
