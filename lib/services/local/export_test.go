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

// SessionDataLimiter exports sdLimiter for tests.
var SessionDataLimiter = sdLimiter

// Reset resets the limiter to its initial state.
// Exposed for testing.
func (l *globalSessionDataLimiter) Reset() {
	l.mu.Lock()
	l.scopeCount = make(map[string]int)
	l.lastReset = time.Time{}
	l.mu.Unlock()
}

const (
	WebPrefix    = webPrefix
	UsersPrefix  = usersPrefix
	ParamsPrefix = paramsPrefix
)
