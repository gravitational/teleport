/**
 * Copyright (C) 2025 Gravitational, Inc.
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

// Provider provides a reference to a RecordingMetadata service
// implementation.
//
// The actual metadata service has to be created after the
// lib/events.ProtoStreamer, as the metadata service depends on the streamer
// for loading session recordings, however, the streamer depends on the metadata
// service, as it needs to call it when the upload finishes.
type Provider struct {
	metadata RecordingMetadata
	mu       sync.Mutex
}

// RecordingMetadata provides the configured [RecordingMetadata]. It is safe to
// call this function from any thread. The returned [RecordingMetadata] is
// guaranteed to never be nil.
func (p *Provider) RecordingMetadata() RecordingMetadata {
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
func (p *Provider) SetRecordingMetadata(s RecordingMetadata) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadata = s
}

// NewProvider creates a new [Provider] without the metadata service set.
func NewProvider() *Provider {
	sp := &Provider{}
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
