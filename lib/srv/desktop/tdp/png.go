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

package tdp

import "image/png"

// PNGEncoder returns the encoder used for PNG Frames.
// It is not safe for concurrent use.
func PNGEncoder() *png.Encoder {
	return &png.Encoder{
		CompressionLevel: png.BestSpeed,
		BufferPool:       &pool{},
	}
}

// pool implements png.EncoderBufferPool,
// allowing us to reuse encoding resources
type pool struct {
	b *png.EncoderBuffer
}

// all encoding happens in a single thread, so we don't
// need anything as sophisticated as a sync.Pool here

func (p *pool) Get() *png.EncoderBuffer   { return p.b }
func (p *pool) Put(eb *png.EncoderBuffer) { p.b = eb }
