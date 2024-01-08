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
	player := nopPlayer{}
	f.Fuzz(func(t *testing.T, b []byte) {
		handlePlaybackAction(b, player)
	})
}

type nopPlayer struct{}

func (nopPlayer) SetPos(time.Duration) error { return nil }
func (nopPlayer) Play() error                { return nil }
func (nopPlayer) Pause() error               { return nil }
