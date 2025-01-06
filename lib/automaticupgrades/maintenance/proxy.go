/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package maintenance

import (
	"context"

	"github.com/gravitational/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/automaticupgrades/cache"
	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
)

type proxyMaintenanceClient struct {
	client *webclient.ReusableClient
}

// Get does the HTTPS call to the Teleport Proxy sevrice to check if the update should happen now.
// If the proxy response does not contain the auto_update.agent_version field,
// this means the proxy does not support autoupdates. In this case we return trace.NotImplementedErr.
func (b *proxyMaintenanceClient) Get(ctx context.Context) (bool, error) {
	resp, err := b.client.Find()
	if err != nil {
		return false, trace.Wrap(err)
	}
	// We check if a version is advertised to know if the proxy implements RFD-184 or not.
	if resp.AutoUpdate.AgentVersion == "" {
		return false, trace.NotImplemented("proxy does not seem to implement RFD-184")
	}
	return resp.AutoUpdate.AgentAutoUpdate, nil
}

// ProxyMaintenanceTrigger checks if the maintenance should be triggered from the Teleport Proxy service /find endpoint,
// as specified in the RFD-184: https://github.com/gravitational/teleport/blob/master/rfd/0184-agent-auto-updates.md
// The Trigger returns trace.NotImplementedErr when running against a proxy that does not seem to
// expose automatic update instructions over the /find endpoint (proxy too old).
type ProxyMaintenanceTrigger struct {
	name         string
	cachedGetter func(context.Context) (bool, error)
}

// Name implements maintenance.Trigger returns the trigger name for logging
// and debugging purposes.
func (g ProxyMaintenanceTrigger) Name() string {
	return g.name
}

// Default implements maintenance.Trigger and returns what to do if the trigger can't be evaluated.
// ProxyMaintenanceTrigger should fail open, so the function returns true.
func (g ProxyMaintenanceTrigger) Default() bool {
	return false
}

// CanStart implements maintenance.Trigger.
func (g ProxyMaintenanceTrigger) CanStart(ctx context.Context, _ client.Object) (bool, error) {
	result, err := g.cachedGetter(ctx)
	return result, trace.Wrap(err)
}

// NewProxyMaintenanceTrigger builds and return a Trigger checking a public HTTP endpoint.
func NewProxyMaintenanceTrigger(name string, clt *webclient.ReusableClient) Trigger {
	maintenanceClient := &proxyMaintenanceClient{
		client: clt,
	}

	return ProxyMaintenanceTrigger{name, cache.NewTimedMemoize[bool](maintenanceClient.Get, constants.CacheDuration).Get}
}
