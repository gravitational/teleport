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

package summarizer

import (
	"context"
	"sync"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// SessionSummarizerProvider provides a reference to a Summarizer service
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
type SessionSummarizerProvider struct {
	summarizer SessionSummarizer
	mu         sync.Mutex
}

// SessionSummarizer provides the configured [SessionSummarizer]. It is safe to
// call this function from any thread. The returned [SessionSummarizer] is
// guaranteed to never be nil.
func (p *SessionSummarizerProvider) SessionSummarizer() SessionSummarizer {
	if p == nil {
		return NoopSummarizer{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.summarizer == nil {
		return NoopSummarizer{}
	}

	return p.summarizer
}

// SetSummarizer sets the summarizer service to be provided. It is safe to call
// this function from any thread.
func (p *SessionSummarizerProvider) SetSummarizer(s SessionSummarizer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.summarizer = s
}

// NewSessionSummarizerProvider creates a new [SessionSummarizerProvider]
// without a summarizer.
func NewSessionSummarizerProvider() *SessionSummarizerProvider {
	sp := &SessionSummarizerProvider{}
	sp.SetSummarizer(NoopSummarizer{})
	return sp
}

// NoopSummarizer is a no-op implementation of the [SessionSummarizer]
// interface.
type NoopSummarizer struct{}

func (n NoopSummarizer) SummarizeSSH(ctx context.Context, sessionEndEvent *events.SessionEnd) error {
	return nil
}

func (n NoopSummarizer) SummarizeDatabase(ctx context.Context, sessionEndEvent *events.DatabaseSessionEnd) error {
	return nil
}

func (NoopSummarizer) SummarizeWithoutEndEvent(ctx context.Context, sessionID session.ID) error {
	return nil
}
