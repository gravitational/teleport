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
 * GNU Affero General Public License for more metadata.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package recordingmetadata

import (
	"context"
	"sync"

	"github.com/gravitational/teleport/lib/session"
)

// RecordingMetadataProvider provides a reference to a RecordingMetadata service
// implementation. Here is why it is needed:
//
// The actual metadata service has to be created after the
// lib/events.ProtoStreamer, as the metadata lives in the enterprise plugin,
// which is configured later in the initialization process. Another factor here
// is that the metadata depends on the streamer for loading session
// recordings, however, the streamer depends on the metadata, as it needs to
// call it when the upload finishes.
//
// In an absence of a dependency injection container, one solution is to use a
// provider that is passed to the streamer as a dependency. It allows for late
// initialization of the metadata service. This way the provided service can
// be replaced without ever needing to change the streamer interface by adding
// a setter method; doing so would pollute the interface and require some
// unnecessary stub implementations.
type RecordingMetadataProvider struct {
	metadata RecordingMetadata
	mu       sync.Mutex
}

// RecordingMetadata provides the configured [RecordingMetadata]. It is safe to
// call this function from any thread. The returned [RecordingMetadata] is
// guaranteed to never be nil.
func (p *RecordingMetadataProvider) RecordingMetadata() RecordingMetadata {
	if p == nil {
		return NoopRecordingMetadata{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.metadata == nil {
		return NoopRecordingMetadata{}
	}

	return p.metadata
}

// SetRecordingMetadata sets the metadata service to be provided. It is safe to call
// this function from any thread.
func (p *RecordingMetadataProvider) SetRecordingMetadata(s RecordingMetadata) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadata = s
}

// NewRecordingMetadataProvider creates a new [RecordingMetadataProvider]
// without a metadata.
func NewRecordingMetadataProvider() *RecordingMetadataProvider {
	sp := &RecordingMetadataProvider{}
	sp.SetRecordingMetadata(NoopRecordingMetadata{})
	return sp
}

// NoopRecordingMetadata is a no-op implementation of the [RecordingMetadata]
// interface.
type NoopRecordingMetadata struct{}

// ProcessSessionRecording is a no-op implementation of the
// [RecordingMetadata.ProcessSessionRecording] method.
func (n NoopRecordingMetadata) ProcessSessionRecording(ctx context.Context, sessionID session.ID) error {
	return nil
}
