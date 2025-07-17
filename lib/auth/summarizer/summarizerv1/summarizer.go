// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package summarizerv1

import (
	"context"
	"sync/atomic"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// Summarizer summarizes session recordings using language model inference.
type Summarizer interface {
	// Summarize summarizes a session recording with a given ID. The
	// sessionEndEvent is optional, but should be specified if possible, as an
	// optimization to skip reading the session stream in order to find the end
	// event.
	Summarize(ctx context.Context, sessionID session.ID, sessionEndEvent *events.OneOf) error
}

// SummarizerProvider provides a reference to a Summarizer service
// implementation. Here is why it is needed:
//
// The actual summarizer service has to be created after the
// lib/events.ProtoStreamer, as the summarizer lives in the enterprise plugin,
// which is configured later in the initialization process. Another factor here
// is that the summarizer depends on the streamer for loading session
// recordings, however, the streamer depends on the summarizer, as it needs to
// call it when the upload finishes.
//
// In an absence of a dependency injection container, one solution is to use a
// provider that is passed to the streamer as a dependency. It allows for late
// initialization of the summarizer service. This way the provided service can
// be replaced without ever needing to change the streamer interface by adding
// a setter method; doing so would pollute the interface and require some
// unnecessary stub implementations.
type SummarizerProvider struct {
	summarizer atomic.Pointer[Summarizer]
}

// ProvideSummarizer provides the summarizer service. It is safe to call this
// function from any thread. Allows being called on a nil provider.
func (p *SummarizerProvider) ProvideSummarizer() Summarizer {
	if p == nil {
		return nil
	}

	s := p.summarizer.Load()
	if s == nil {
		return nil
	}

	return *s
}

// SetSummarizer sets the summarizer service to be provided. It is safe to call
// this function from any thread.
func (p *SummarizerProvider) SetSummarizer(summarizer Summarizer) {
	p.summarizer.Store(&summarizer)
}

// NewSummarizerProvider creates a new SummarizerProvider without a summarizer.
func NewSummarizerProvider() *SummarizerProvider {
	return &SummarizerProvider{}
}
