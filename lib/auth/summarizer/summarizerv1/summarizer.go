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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// Summarizer summarizes session recordings using language model inference.
type Summarizer interface {
	// Summarize summarizes a session recording with a given ID. The
	// sessionEndEvent is optional, but should be specified if possible, as it
	// lets us skip reading the session stream just to find the end event.
	Summarize(ctx context.Context, sessionID session.ID, sessionEndEvent *events.OneOf) error
}

// SummarizerWrapper is a wrapper around the SummarizerService interface. Its
// purpose is to allow substituting the wrapped service after a dependent
// service has been configured with the wrapper as the service implementation.
type SummarizerWrapper struct {
	Summarizer
}

func NewSummarizerWrapper() *SummarizerWrapper {
	return &SummarizerWrapper{
		Summarizer: &UnimplementedSummarizer{},
	}
}

type UnimplementedSummarizer struct{}

func (s *UnimplementedSummarizer) Summarize(
	ctx context.Context, sessionID session.ID, sessionEndEvent *events.OneOf,
) error {
	return requireEnterprise()
}

func requireEnterprise() error {
	return trace.AccessDenied(
		"session recording summarization is only available with an enterprise license that supports Teleport Identity Security")
}
