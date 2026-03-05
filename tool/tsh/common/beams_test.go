/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"os"
	"testing"
	"time"
)

// TODO(greedy52) DELETE ME
func TestBeamSpinner(t *testing.T) {
	stopCreating := startBeamSpinner(os.Stdout, "creating...")
	time.Sleep(2 * time.Second)
	stopCreating("◆ created beams-abc-123-fake-id")

	stopConnecting := startBeamSpinner(os.Stdout, "connecting...")
	time.Sleep(2 * time.Second)
	stopConnecting("↳ ready")
}
