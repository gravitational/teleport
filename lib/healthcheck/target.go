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

package healthcheck

import (
	"context"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// EndpointsResolverFunc is callback func that returns endpoints for a target.
type EndpointsResolverFunc func(ctx context.Context) ([]string, error)

// OnHealthChangeFunc is a func called on each health change.
type OnHealthChangeFunc func(oldHealth, newHealth types.TargetHealth)

// Target is a health check target.
type Target struct {
	// GetResource gets a copy of the target resource with updated labels.
	GetResource func() types.ResourceWithLabels
	// ResolverFn resolves the target endpoint(s).
	ResolverFn EndpointsResolverFunc

	// -- test fields below --

	// dialFn used to mock dialing in tests
	dialFn dialFunc
	// onHealthCheck is called after each health check.
	onHealthCheck func(lastResultErr error)
	// onConfigUpdate is called after each config update.
	onConfigUpdate func()
	// onClose is called after the target's worker closes.
	onClose func()
}

func (t *Target) checkAndSetDefaults() error {
	if t.GetResource == nil {
		return trace.BadParameter("missing target resource getter")
	}
	if t.ResolverFn == nil {
		return trace.BadParameter("missing target endpoint resolver")
	}
	if t.dialFn == nil {
		t.dialFn = defaultDialer().DialContext
	}
	return nil
}

func defaultDialer() *net.Dialer {
	return &net.Dialer{}
}
