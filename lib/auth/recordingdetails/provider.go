/**
 * Copyright (C) 2024 Gravitational, Inc.
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

package recordingdetails

import (
	"context"
	"fmt"
	"sync"

	"github.com/gravitational/teleport/lib/session"
)

// RecordingDetailsProvider provides a reference to a RecordingDetails service
// implementation. Here is why it is needed:
//
// The actual details service has to be created after the
// lib/events.ProtoStreamer, as the details lives in the enterprise plugin,
// which is configured later in the initialization process. Another factor here
// is that the details depends on the streamer for loading session
// recordings, however, the streamer depends on the details, as it needs to
// call it when the upload finishes.
//
// In an absence of a dependency injection container, one solution is to use a
// provider that is passed to the streamer as a dependency. It allows for late
// initialization of the details service. This way the provided service can
// be replaced without ever needing to change the streamer interface by adding
// a setter method; doing so would pollute the interface and require some
// unnecessary stub implementations.
type RecordingDetailsProvider struct {
	details RecordingDetails
	mu      sync.Mutex
}

// RecordingDetails provides the configured [RecordingDetails]. It is safe to
// call this function from any thread. The returned [RecordingDetails] is
// guaranteed to never be nil.
func (p *RecordingDetailsProvider) RecordingDetails() RecordingDetails {
	if p == nil {
		fmt.Println("here1")
		return NoopRecordingDetails{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.details == nil {
		fmt.Println("here2")
		return NoopRecordingDetails{}
	}

	fmt.Println("here3")
	return p.details
}

// SetRecordingDetails sets the details service to be provided. It is safe to call
// this function from any thread.
func (p *RecordingDetailsProvider) SetRecordingDetails(s RecordingDetails) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.details = s
}

// NewRecordingDetailsProvider creates a new [RecordingDetailsProvider]
// without a details.
func NewRecordingDetailsProvider() *RecordingDetailsProvider {
	sp := &RecordingDetailsProvider{}
	sp.SetRecordingDetails(NoopRecordingDetails{})
	return sp
}

// NoopRecordingDetails is a no-op implementation of the [RecordingDetails]
// interface.
type NoopRecordingDetails struct{}

func (n NoopRecordingDetails) ProcessSessionRecording(ctx context.Context, sessionID session.ID) error {
	fmt.Println("nooop")
	return nil
}
