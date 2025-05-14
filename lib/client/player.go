/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package client

import (
	"context"
	"os"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// playFromFileStreamer implements [player.Streamer] for
// streaming from a local file.
type playFromFileStreamer struct {
	filename string
}

func (p *playFromFileStreamer) StreamSessionEvents(
	ctx context.Context,
	sessionID session.ID,
	startIndex int64,
) (chan apievents.AuditEvent, chan error) {
	evts := make(chan apievents.AuditEvent)
	errs := make(chan error, 1)

	go func() {
		f, err := os.Open(p.filename)
		if err != nil {
			errs <- trace.ConvertSystemError(err)
			return
		}
		defer f.Close()

		pr, err := events.NewProtoReader(f, nil)
		if err != nil {
			errs <- trace.Wrap(err)
			return
		}

		for i := int64(0); ; i++ {
			evt, err := pr.Read(ctx)
			if err != nil {
				errs <- trace.Wrap(err)
				return
			}

			if i >= startIndex {
				select {
				case evts <- evt:
				case <-ctx.Done():
					errs <- trace.Wrap(ctx.Err())
					return
				}
			}
		}
	}()

	return evts, errs
}
