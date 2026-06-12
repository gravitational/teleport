// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package dns

import (
	"context"
	"net/netip"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// OSUpstreamNameserverSource provides the list of upstream DNS nameservers
// configured in the OS. The VNet DNS resolver will forward unhandles queries to
// these nameservers.
type OSUpstreamNameserverSource struct {
	ttlCache *utils.FnCache
}

// NewOSUpstreamNameserverSource returns a new *OSUpstreamNameserverSource.
func NewOSUpstreamNameserverSource() (*OSUpstreamNameserverSource, error) {
	ttlCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL: 10 * time.Second,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &OSUpstreamNameserverSource{
		ttlCache: ttlCache,
	}, nil
}

// UpstreamNameservers returns a cached view of the host OS's current default
// nameservers.
func (s *OSUpstreamNameserverSource) UpstreamNameservers(ctx context.Context) ([]string, error) {
	return utils.FnCacheGet(ctx, s.ttlCache, 0, loadUpstreamNameservers)
}

func loadUpstreamNameservers(ctx context.Context) ([]string, error) {
	return platformLoadUpstreamNameservers(ctx)
}

func withDNSPort(addr netip.Addr) string {
	return netip.AddrPortFrom(addr, 53).String()
}
