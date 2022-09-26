// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reversetunnel

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
		a, err := cache.Get(ctx, "resolver", func(ctx context.Context) (interface{}, error) {
			addr, mode, err := resolver(ctx)
			if err != nil {
				return nil, err
			}

			return &data{addr: addr, mode: mode}, nil
		})
		if err != nil {
			return nil, types.ProxyListenerMode_Separate, err
		}

		d := a.(*data)
		if d.addr != nil {
			// make a copy to avoid a data race when the caching resolver is shared by goroutines.
			addrCopy := *d.addr
			return &addrCopy, d.mode, nil
		}

		return d.addr, d.mode, nil
	}, nil
}

// WebClientResolver returns a Resolver which uses the web proxy to
// discover where the SSH reverse tunnel server is running.
func WebClientResolver(addrs []utils.NetAddr, insecureTLS bool) Resolver {
	return func(ctx context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
		var errs []error
		mode := types.ProxyListenerMode_Separate
		for _, addr := range addrs {
			// In insecure mode, any certificate is accepted. In secure mode the hosts
			// CAs are used to validate the certificate on the proxy.
			resp, err := webclient.Find(&webclient.Config{Context: ctx, ProxyAddr: addr.String(), Insecure: insecureTLS})

			if err != nil {
				errs = append(errs, err)
				continue
			}

			tunnelAddr, err := resp.Proxy.TunnelAddr()
			if err != nil {
				errs = append(errs, err)
				continue
			}

			addr, err := utils.ParseAddr(tunnelAddr)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			addr.Addr = utils.ReplaceUnspecifiedHost(addr, defaults.HTTPListenPort)
			if resp.Proxy.TLSRoutingEnabled {
				mode = types.ProxyListenerMode_Multiplex
			}
			return addr, mode, nil
		}
		return nil, mode, trace.NewAggregate(errs...)
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
