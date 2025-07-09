// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package boundkeypairexperiment

import (
	"os"
	"strconv"
	"sync"
)

var mu sync.Mutex

var experimentEnabled, _ = strconv.ParseBool(os.Getenv("TELEPORT_UNSTABLE_BOUND_KEYPAIR_JOINING_EXPERIMENT"))

// Enabled returns true if the bound keypair joining experiment is enabled.
func Enabled() bool {
	mu.Lock()
	defer mu.Unlock()

	return experimentEnabled
}

// SetEnabled sets the experiment enabled flag.
func SetEnabled(enabled bool) {
	mu.Lock()
	defer mu.Unlock()

	experimentEnabled = enabled
}
