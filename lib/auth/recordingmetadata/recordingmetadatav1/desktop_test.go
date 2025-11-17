/**
 * Teleport
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
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package recordingmetadatav1

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

func TestDesktopThumbnails(t *testing.T) {
	ctx := t.Context()

	streamer := &playFromFileStreamer{filename: "desktop.tar"}

	uploadHandler := newMockUploadHandler()

	service, err := NewRecordingMetadataService(RecordingMetadataServiceConfig{
		Streamer:      streamer,
		UploadHandler: uploadHandler,
	})
	require.NoError(t, err)

	err = service.ProcessSessionRecording(ctx, "test-session", 2*time.Minute)

	require.NoError(t, err)
}

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

		pr := events.NewProtoReader(f, nil)

		for i := int64(0); ; i++ {
			evt, err := pr.Read(ctx)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errs <- trace.Wrap(err)
				}
				close(evts)
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
