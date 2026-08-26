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

package local

import "time"

// ServiceOption is a functional option for configuring the service.
// TODO(tigrato): Add support for other services besides the access list service.
type ServiceOption func(*serviceOptions)

type serviceOptions struct {
	runWhileLockedRetryInterval time.Duration
}

// WithRunWhileLockedRetryInterval sets the retry interval for the RunWhileLocked function.
func WithRunWhileLockedRetryInterval(interval time.Duration) ServiceOption {
	return func(o *serviceOptions) {
		o.runWhileLockedRetryInterval = interval
	}
}
