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

package update

import (
	"fmt"
	"os"
	"testing"
)

func TestUpdate(t *testing.T) {
	// Create $TELEPORT_HOME/bin if it does not exist.
	dir, err := toolsDir()
	if err != nil {
		t.Fatalf("Failed to find tools directory: %v.", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create tools directory: %v.", err)
	}

	err = update("15.3.4")
	fmt.Printf("--> err: %v.\n", err)
}
