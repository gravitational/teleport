// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package experiment

import (
	"os"
	"sync"
)

var mu sync.Mutex

var experimentEnabled = os.Getenv("TELEPORT_WORKLOAD_IDENTITY_UX_EXPERIMENT") == "1"

// Enabled returns true if the workload identity UX experiment is
// enabled.
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
