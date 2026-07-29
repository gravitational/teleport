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

package replayfixture

import (
	"context"
	"slices"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// Source serves a fixture's recordings through the two audit log calls the beam
// replay pipeline makes: a filtered search over the cluster's events, and a
// per-session stream of a chunk recording. Passing one to the replay sink runs
// the whole pipeline against an exported beam with no cluster attached.
//
// A Source is not a general audit log. It applies the event-type, beam, and
// ordering parts of a search request, but deliberately ignores the From/To
// window and the free-text Search: a fixture holds exactly one beam's export,
// already bounded when it was written, and callers routinely replay it through a
// Job with no timestamps at all. Honoring the window would make those searches
// return nothing, and would re-clip events the export padded its window to
// capture.
//
// A Source is read-only and safe for concurrent use.
type Source struct {
	beamEvents []apievents.AuditEvent
	chunks     map[string][]apievents.AuditEvent
}

// Source decodes the fixture into an audit source. It decodes every recording
// up front, so a fixture that lost an event type fails here rather than midway
// through a replay.
func (f *Fixture) Source() (*Source, error) {
	beamEvents, err := f.AuditEvents()
	if err != nil {
		return nil, trace.Wrap(err, "decoding beam events")
	}
	chunks := make(map[string][]apievents.AuditEvent, len(f.Chunks))
	for id := range f.Chunks {
		evs, err := f.ChunkEvents(id)
		if err != nil {
			return nil, trace.Wrap(err, "decoding chunk recording %q", id)
		}
		chunks[id] = evs
	}
	return &Source{beamEvents: beamEvents, chunks: chunks}, nil
}

// beamIDGetter is implemented by every event embedding UserMetadata, which
// promotes a GetBeamID method.
type beamIDGetter interface{ GetBeamID() string }

// SearchEvents returns the beam's events matching req, and the key to resume
// from when more remain. See the Source doc for which parts of req apply.
func (s *Source) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", trace.Wrap(err)
	}

	matching := make([]apievents.AuditEvent, 0, len(s.beamEvents))
	for _, ev := range s.beamEvents {
		if len(req.EventTypes) > 0 && !slices.Contains(req.EventTypes, ev.GetType()) {
			continue
		}
		if req.BeamID != "" {
			bg, ok := ev.(beamIDGetter)
			if !ok || bg.GetBeamID() != req.BeamID {
				continue
			}
		}
		matching = append(matching, ev)
	}
	// The fixture keeps the order the audit log returned events in, which is the
	// ascending order every caller asks for; only descending needs work.
	if req.Order == types.EventOrderDescending {
		slices.Reverse(matching)
	}

	offset := 0
	if req.StartKey != "" {
		parsed, err := strconv.Atoi(req.StartKey)
		if err != nil || parsed < 0 {
			return nil, "", trace.BadParameter("invalid start key %q", req.StartKey)
		}
		offset = min(parsed, len(matching))
	}
	page := matching[offset:]
	if req.Limit > 0 && len(page) > req.Limit {
		// More to come: hand back the offset the next page starts at. Only a
		// truncated page produces a key, so a caller paging until the key is
		// empty terminates.
		return page[:req.Limit], strconv.Itoa(offset + req.Limit), nil
	}
	return page, "", nil
}

// StreamSessionEvents streams the chunk recording named sessionID, skipping its
// first startIndex events. It follows the audit log's contract: the event
// channel is closed once the recording has been streamed in full, and the error
// channel receives only on failure -- so a recording missing from the export is
// reported as an error rather than as an empty recording.
func (s *Source) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	evCh := make(chan apievents.AuditEvent)
	errCh := make(chan error, 1)

	evs, ok := s.chunks[sessionID.String()]
	if !ok {
		// evCh is left open: closing it as well would let a consumer selecting
		// over both channels see a clean end of recording and miss the error.
		errCh <- trace.NotFound("fixture has no chunk recording %q", sessionID)
		return evCh, errCh
	}
	if startIndex > 0 {
		if startIndex >= int64(len(evs)) {
			evs = nil
		} else {
			evs = evs[startIndex:]
		}
	}

	go func() {
		defer close(evCh)
		for _, ev := range evs {
			select {
			case evCh <- ev:
			case <-ctx.Done():
				// Abandoned mid-recording. Report it so a consumer that is still
				// reading does not mistake the closed channel for a complete
				// recording, and stop rather than leaking this goroutine.
				errCh <- trace.Wrap(ctx.Err())
				return
			}
		}
	}()
	return evCh, errCh
}
