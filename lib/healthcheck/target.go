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
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// supportedTargetKinds is a list of resource kinds that support health checks.
var supportedTargetKinds = []string{
	types.KindDatabase,
}

// EndpointsResolverFunc is callback func that returns endpoints for a target.
type EndpointsResolverFunc func(ctx context.Context) ([]string, error)

// OnHealthChangeFunc is a func called on each health change.
type OnHealthChangeFunc func(oldHealth, newHealth types.TargetHealth)

// Target is a health check target.
type Target struct {
	// Resource is the target resource.
	Resource types.ResourceWithLabels
	// GetUpdatedResourceFn gets a copy of the target resource with updated
	// labels.
	GetUpdatedResourceFn func() types.ResourceWithLabels
	// ResolverFn resolves the target endpoint(s).
	ResolverFn EndpointsResolverFunc
}

func (t *Target) check() error {
	if t.Resource == nil {
		return trace.BadParameter("missing target resource")
	}
	if t.GetUpdatedResourceFn == nil {
		return trace.BadParameter("missing target resource update getter")
	}
	if t.ResolverFn == nil {
		return trace.BadParameter("missing target endpoint resolver")
	}
	if !slices.Contains(supportedTargetKinds, t.Resource.GetKind()) {
		return trace.BadParameter("health check target resource kind %q is not supported", t.Resource.GetKind())
	}
	return nil
}
