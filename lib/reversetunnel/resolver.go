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

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Resolver looks up reverse tunnel addresses
type Resolver func() (*utils.NetAddr, error)

// CachingResolver wraps the provided Resolver with one that will cache the previous result
// for 3 seconds to reduce the number of resolutions in an effort to mitigate potentially
// overwhelming the Resolver source.
func CachingResolver(resolver Resolver, clock clockwork.Clock) (Resolver, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   3 * time.Second,
		Clock: clock,
	})
	if err != nil {
		return nil, err
	}
	return func() (*utils.NetAddr, error) {
		a, err := cache.Get(context.TODO(), "resolver", func() (interface{}, error) {
			return resolver()
		})
		if err != nil {
			return nil, err
		}
		return a.(*utils.NetAddr), nil
	}, nil
}

// WebClientResolver returns a Resolver which uses the web proxy to
// discover where the SSH reverse tunnel server is running.
func WebClientResolver(ctx context.Context, addrs []utils.NetAddr, insecureTLS bool) Resolver {
	return func() (*utils.NetAddr, error) {
		var errs []error
		for _, addr := range addrs {
			// In insecure mode, any certificate is accepted. In secure mode the hosts
			// CAs are used to validate the certificate on the proxy.
			tunnelAddr, err := webclient.GetTunnelAddr(
				&webclient.Config{Context: ctx, ProxyAddr: addr.String(), Insecure: insecureTLS})

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
			return addr, nil
		}
		return nil, trace.NewAggregate(errs...)
	}
}

// StaticResolver returns a Resolver which will always resolve to
// the provided address
func StaticResolver(address string) Resolver {
	addr, err := utils.ParseAddr(address)
	if err == nil {
		addr.Addr = utils.ReplaceUnspecifiedHost(addr, defaults.HTTPListenPort)
	}

	return func() (*utils.NetAddr, error) {
		return addr, err
	}
}
