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

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// SessionSummarizer summarizes session recordings using language model
// inference.
type SessionSummarizer interface {
	// SummarizeSSH summarizes the SSH session recording associated with the
	// provided [events.SessionEnd] event.
	SummarizeSSH(ctx context.Context, sessionEndEvent *events.SessionEnd) error
	// SummarizeDatabase summarizes the database session recording associated
	// with the provided [events.DatabaseSessionEnd] event.
	SummarizeDatabase(ctx context.Context, sessionEndEvent *events.DatabaseSessionEnd) error
	// SummarizeWithoutEndEvent summarizes a session recording with a given ID.
	// This is used for cases where the session ID is known, but there is no end
	// event available. [SessionSummarizer.SummarizeSSH] and
	// [SessionSummarizer.SummarizeDatabase] should be used instead of this
	// method whenever possible, as they are more efficient.
	SummarizeWithoutEndEvent(ctx context.Context, sessionID session.ID) error
}
