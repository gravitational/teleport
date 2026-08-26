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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// HealthChecker is a resource which provides health checks.
type HealthChecker interface {
	// CheckHealth checks the health of a target resource.
	CheckHealth(ctx context.Context) ([]string, error)
	// GetProtocol returns the network protocol used for checking health.
	GetProtocol() types.TargetHealthProtocol
}

// Target is a health check target.
type Target struct {
	// HealthChecker checks the resource's health.
	HealthChecker
	// GetResource gets a copy of the target resource with updated labels.
	GetResource func() types.ResourceWithLabels

	// -- test fields below --

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
	if t.HealthChecker == nil {
		return trace.BadParameter("missing health checker")
	}
	return nil
}
