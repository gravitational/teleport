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

package reversetunnelclient

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// Resolver looks up reverse tunnel addresses
type Resolver func(ctx context.Context) (*utils.NetAddr, types.ProxyListenerMode, error)

// CachingResolver wraps the provided Resolver with one that will cache the previous result
// for 3 seconds to reduce the number of resolutions in an effort to mitigate potentially
// overwhelming the Resolver source.
func CachingResolver(ctx context.Context, resolver Resolver, clock clockwork.Clock) (Resolver, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     3 * time.Second,
		Clock:   clock,
		Context: ctx,
	})
	if err != nil {
		return nil, err
	}

	type data struct {
		addr *utils.NetAddr
		mode types.ProxyListenerMode
	}

	return func(ctx context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
		d, err := utils.FnCacheGet(ctx, cache, "resolver", func(ctx context.Context) (data, error) {
			addr, mode, err := resolver(ctx)

			return data{addr: addr, mode: mode}, err
		})
		if err != nil {
			return nil, types.ProxyListenerMode_Separate, err
		}

		if d.addr != nil {
			// make a copy to avoid a data race when the caching resolver is shared by goroutines.
			addrCopy := *d.addr
			return &addrCopy, d.mode, nil
		}

		return nil, d.mode, nil
	}, nil
}

// WebClientResolver returns a Resolver which uses the web proxy to
// discover where the SSH reverse tunnel server is running.
func WebClientResolver(cfg *webclient.Config) Resolver {
	return func(ctx context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
		mode := types.ProxyListenerMode_Separate
		// In insecure mode, any certificate is accepted. In secure mode the hosts
		// CAs are used to validate the certificate on the proxy.
		resp, err := webclient.Find(cfg)

		if err != nil {
			return nil, mode, trace.Wrap(err)
		}

		tunnelAddr, err := resp.Proxy.TunnelAddr()
		if err != nil {
			return nil, mode, trace.Wrap(err)
		}

		addr, err := utils.ParseAddr(tunnelAddr)
		if err != nil {
			return nil, mode, trace.Wrap(err)
		}

		addr.Addr = utils.ReplaceUnspecifiedHost(addr, defaults.HTTPListenPort)
		if resp.Proxy.TLSRoutingEnabled {
			mode = types.ProxyListenerMode_Multiplex
		}
		return addr, mode, nil
	}
}

// StaticResolver returns a Resolver which will always resolve to
// the provided address
func StaticResolver(address string, mode types.ProxyListenerMode) Resolver {
	addr, err := utils.ParseAddr(address)
	if err == nil {
		addr.Addr = utils.ReplaceUnspecifiedHost(addr, defaults.HTTPListenPort)
	}

	return func(context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
		return addr, mode, err
	}
}
