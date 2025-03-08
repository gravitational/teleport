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
	"time"

	"github.com/gravitational/teleport/api/types"
)

// TODO(gavin)(healthcheck): godoc nopush
type healthCheckConfig struct {
	name               string
	protocol           types.TargetHealthProtocol
	interval           time.Duration
	timeout            time.Duration
	healthyThreshold   int
	unhealthyThreshold int

	matcher types.LabelMatchers
}

// isEqual returns whether this health check config is equal to another,
// ignoring label matchers since they have no effect on an running worker.
func (h healthCheckConfig) isEqual(other healthCheckConfig) bool {
	return h.name == other.name &&
		h.protocol == other.protocol &&
		h.interval == other.interval &&
		h.timeout == other.timeout &&
		h.healthyThreshold == other.healthyThreshold &&
		h.unhealthyThreshold == other.unhealthyThreshold
}
