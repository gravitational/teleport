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

package version

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/automaticupgrades/cache"
	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
)

type Finder interface {
	Find() (*webclient.PingResponse, error)
}

type proxyVersionClient struct {
	client Finder
}

func (b *proxyVersionClient) Get(_ context.Context) (string, error) {
	resp, err := b.client.Find()
	if err != nil {
		return "", trace.Wrap(err)
	}
	// We check if a version is advertised to know if the proxy implements RFD-184 or not.
	if resp.AutoUpdate.AgentVersion == "" {
		return "", trace.NotImplemented("proxy does not seem to implement RFD-184")
	}
	return EnsureSemver(resp.AutoUpdate.AgentVersion)
}

// ProxyVersionGetter gets the target version from the Teleport Proxy Service /find endpoint, as
// specified in the RFD-184: https://github.com/gravitational/teleport/blob/master/rfd/0184-agent-auto-updates.md
// The Getter returns trace.NotImplementedErr when running against a proxy that does not seem to
// expose automatic update instructions over the /find endpoint (proxy too old).
type ProxyVersionGetter struct {
	name         string
	cachedGetter func(context.Context) (string, error)
}

// Name implements Getter
func (g ProxyVersionGetter) Name() string {
	return g.name
}

// GetVersion implements Getter
func (g ProxyVersionGetter) GetVersion(ctx context.Context) (string, error) {
	return g.cachedGetter(ctx)
}

// NewProxyVersionGetter creates a ProxyVersionGetter from a webclient.
// The answer is cached for a minute.
func NewProxyVersionGetter(name string, clt *webclient.ReusableClient) Getter {
	versionClient := &proxyVersionClient{
		client: clt,
	}

	return ProxyVersionGetter{name, cache.NewTimedMemoize[string](versionClient.Get, constants.CacheDuration).Get}
}
