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

package experiment

import (
	"os"
	"sync/atomic"
)

var enabled = atomic.Bool{}

func init() {
	enabled.Store(os.Getenv("BOT_INSTANCE_EXPERIMENT") == "1")
}

// Enabled returns true if the bot instance experiment is enabled.
func Enabled() bool {
	return enabled.Load()
}

// SetEnabled sets the bot instance experiment flag to the given value.
func SetEnabled(value bool) {
	enabled.Store(value)
}
