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

package helpers

import (
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/defaults"
)

// SetTestTimeouts affects global timeouts inside Teleport, making connections
// work faster but consuming more CPU (useful for integration testing).
// NOTE: This function modifies global values for timeouts, etc. If your tests
// call this function, they MUST NOT BE RUN IN PARALLEL, as they may stomp on
// other tests.
func SetTestTimeouts(t time.Duration) {
	// TODO(tcsc): Remove this altogether and replace with per-test timeout
	//             config (as per #8913)

	// Space out the timeouts a little, as we don't want to trigger all tasks at the exact same time.
	apidefaults.SetTestTimeouts(time.Duration(float64(t)*1.0), time.Duration(float64(t)*1.1))

	defaults.ResyncInterval = time.Duration(float64(t) * 1.2)
	defaults.HeartbeatCheckPeriod = time.Duration(float64(t) * 1.4)
}
