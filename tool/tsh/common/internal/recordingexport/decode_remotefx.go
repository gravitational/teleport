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

package recordingexport

import (
	"image"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate"
)

// RemoteFXDecoder decodes screen fragments from RemoteFX desktop recordings by driving the shared rdpstate screen
// reconstruction, which handles both legacy TDP and modern TDPB recordings and owns the underlying RDP decoder.
type RemoteFXDecoder struct {
	state *rdpstate.RDPState
}

// NewRemoteFXDecoder creates a decoder for RemoteFX recordings.
//
//nolint:staticcheck // SA4023. False positive, depends on build tags.
func NewRemoteFXDecoder() *RemoteFXDecoder {
	return &RemoteFXDecoder{state: rdpstate.New()}
}

func (r *RemoteFXDecoder) UpdateScreen(evt *apievents.DesktopRecording) (bool, error) {
	r.state.ResetUpdatedRegions()
	if err := r.state.HandleMessage(evt); err != nil {
		return false, trace.Wrap(err)
	}

	return len(r.state.UpdatedRegions()) > 0, nil
}

func (r *RemoteFXDecoder) Image() image.Image {
	if img := r.state.Image(); img != nil {
		return img
	}
	return nil
}

func (r *RemoteFXDecoder) Close() error {
	r.state.Release()
	return nil
}
