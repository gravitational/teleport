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

package web

import (
	"testing"
	"time"
)

func FuzzHandlePlaybackAction(f *testing.F) {
	f.Add([]byte{0x3, 0x0, 0x1, 0x0})                                    // Play
	f.Add([]byte{0x3, 0x0, 0x1, 0x1})                                    // Pause
	f.Add([]byte{0x4, 0x0, 0x8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x8, 0x0}) // Seek
	f.Add([]byte{0x0, 0x0})                                              // Invalid length
	f.Add([]byte{0x0, 0xF, 0x0, 0x0})                                    // Invalid encoded size
	f.Add([]byte{0x30, 0xff, 0xff, 0x30})                                // Size uint16 overflow

	player := nopPlayer{}
	f.Fuzz(func(t *testing.T, b []byte) {
		_ = handlePlaybackAction(b, player)
	})
}

type nopPlayer struct{}

func (nopPlayer) SetPos(time.Duration) error { return nil }
func (nopPlayer) Play() error                { return nil }
func (nopPlayer) Pause() error               { return nil }
