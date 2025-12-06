/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package bot

import (
	"context"
	"sync"

	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/internal"
	svcidentity "github.com/gravitational/teleport/lib/tbot/internal/identity"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

type dynamicService struct {
	name        string
	serviceType string
	done        <-chan struct{}

	mu  sync.Mutex
	err error
}

func (d *dynamicService) error() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.err
}

type dynamicServiceDependencies struct {
	// context is the context in which dynamic services will spawn. It's usually
	// considered evil to keep a context around, but it's better to have some
	// approximately sane lifecycle for spawned services that isn't dependent on
	// whatever the caller might be.
	context           context.Context
	identityService   *svcidentity.Service
	resolver          reversetunnelclient.Resolver
	clientBuilder     *client.Builder
	proxyPinger       connection.ProxyPinger
	reloadBroadcaster *internal.ChannelBroadcaster
	statusRegistry    *readyz.Registry
}
