/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package recordingexport

import (
	"image"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/desktop/rdp/decoder"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

// RemoteFXDecoder decodes screen fragments from RemoteFX desktop recordings.
type RemoteFXDecoder struct {
	d                   *decoder.Decoder
	maxWidth, maxHeight uint16
}

// NewRemoteFXDecoder creates a decoder for RemoteFX recordings.
//
//nolint:staticcheck // SA4023. False positive, depends on build tags.
func NewRemoteFXDecoder(maxWidth, maxHeight uint32) (*RemoteFXDecoder, error) {

	d, err := decoder.New(uint16(maxWidth), uint16(maxHeight))
	if err != nil {
		return nil, trace.Wrap(err, "creating RemoteFX decoder")
	}

	return &RemoteFXDecoder{
		d:         d,
		maxWidth:  uint16(maxWidth),
		maxHeight: uint16(maxHeight),
	}, nil
}

func (r *RemoteFXDecoder) ClearScreen() {
	r.d.Resize(r.maxWidth, r.maxHeight)
}

func (r *RemoteFXDecoder) UpdateScreen(msg tdp.Message) error {
	fastPath, ok := msg.(tdp.RDPFastPathPDU)
	if !ok {
		return nil
	}

	r.d.Process(fastPath)
	return nil
}

func (r *RemoteFXDecoder) Image() image.Image {
	return r.d.Image()
}

func (r *RemoteFXDecoder) Close() error {
	if r.d != nil {
		r.d.Release()
		r.d = nil
	}
	return nil
}
