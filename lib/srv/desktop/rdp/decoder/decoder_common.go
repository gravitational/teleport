// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package decoder

import "math"

// This file contains types and functions that are shared between the nop decoder (built without RDP decoder flag) and
// the decoder built with the RDP decoder flag.

// fitDimensions returns srcW, srcH scaled to fit within maxW x maxH while preserving aspect ratio.
// If the source already fits, it is returned unchanged (no upscaling). Mirrors RdpDecoder::fitted_dimensions.
func fitDimensions(srcW, srcH, maxW, maxH uint16) (uint16, uint16) {
	if srcW <= maxW && srcH <= maxH {
		return srcW, srcH
	}

	scaleW := float64(maxW) / float64(srcW)
	scaleH := float64(maxH) / float64(srcH)
	scale := math.Min(scaleW, scaleH)

	w := uint16(math.Round(float64(srcW) * scale))
	h := uint16(math.Round(float64(srcH) * scale))

	// Clamp: float rounding can push the non-scale-defining dimension one px past max.
	w = max(1, min(w, maxW))
	h = max(1, min(h, maxH))

	return w, h
}

// CursorState represents the state of the mouse cursor, including its visibility and position.
type CursorState struct {
	Visible bool
	X, Y    uint16
}

type decoderConfig struct {
	ioChannelID, userChannelID uint16
}

// Option is an option for configuring the RDP decoder.
type Option func(*decoderConfig)

// WithIOChannelID sets the I/O channel ID in the decoder configuration.
func WithIOChannelID(id uint16) Option {
	return func(c *decoderConfig) {
		c.ioChannelID = id
	}
}

// WithUserChannelID sets the user channel ID in the decoder configuration.
func WithUserChannelID(id uint16) Option {
	return func(c *decoderConfig) {
		c.userChannelID = id
	}
}
