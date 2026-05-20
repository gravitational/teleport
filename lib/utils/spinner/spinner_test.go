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

package spinner

import (
	"bytes"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpinner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var buf bytes.Buffer
		s := New(&buf, "creating...")
		t.Cleanup(s.Stop)

		time.Sleep(s.model.FPS * time.Duration(len(s.model.Frames)*2))
		s.Stop()

		// Every frame should be preceded by a carriage return after two full cycles.
		output := buf.String()
		for _, frame := range s.model.Frames {
			assert.Contains(t, output, "\r"+s.style.Render(frame)+" creating...")
		}
	})
}
