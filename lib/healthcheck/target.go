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

	"github.com/gravitational/teleport/api/types"
)

// Target is a resource which provides health checks.
type Target interface {
	// GetResource gets the target resource.
	GetResource() types.ResourceWithLabels
	// GetAddress gets the address of the target resource.
	GetAddress() string
	// GetProtocol gets the network communication protocol for the target resource.
	GetProtocol() types.TargetHealthProtocol
	// CheckAndSetDefaults checks and sets defaults settings for the target resource.
	CheckAndSetDefaults() error
	// CheckHealth checks the health of the target resource.
	CheckHealth(ctx context.Context) error

	// -- test methods below --

	// OnHealthCheck is called after each health check.
	OnHealthCheck(lastResultErr error)
	// OnConfigUpdate is called after each config update.
	OnConfigUpdate()
	// OnClose is called after the target's worker closes.
	OnClose()
}
