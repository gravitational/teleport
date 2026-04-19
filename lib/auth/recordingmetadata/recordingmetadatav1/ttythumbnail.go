/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/hinshun/vt10x"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/terminal"
)

// ttyThumbnailGenerator is a thumbnail generator that produces thumbnails for a terminal session (i.e. SSH, kubernetes)
// by maintaining an internal virtual terminal that it updates as it processes events from the session recording.
type ttyThumbnailGenerator struct {
	vt       vt10x.Terminal
	logger   *slog.Logger
	disabled bool
}

func newTTYThumbnailGenerator(logger *slog.Logger) *ttyThumbnailGenerator {
	if logger == nil {
		logger = slog.Default()
	}

	return &ttyThumbnailGenerator{
		vt:     vt10x.New(),
		logger: logger,
	}
}

func (t *ttyThumbnailGenerator) handleEvent(event apievents.AuditEvent) error {
	if t.disabled {
		return nil
	}

	switch e := event.(type) {
	case *apievents.SessionStart:
		return t.handleSessionStart(e)

	case *apievents.Resize:
		return t.handleResize(e)

	case *apievents.SessionPrint:
		return t.handleSessionPrint(e)
	}

	return nil
}

func (t *ttyThumbnailGenerator) handleSessionStart(evt *apievents.SessionStart) error {
	return t.handleTerminalResize(evt.TerminalSize)
}

func (t *ttyThumbnailGenerator) handleResize(evt *apievents.Resize) error {
	return t.handleTerminalResize(evt.TerminalSize)
}

func (t *ttyThumbnailGenerator) handleSessionPrint(evt *apievents.SessionPrint) error {
	defer func() {
		if recoverValue := recover(); recoverValue != nil {
			t.disabled = true
			t.logger.Warn("Disabling TTY thumbnail generation after vt10x panic",
				"panic", fmt.Sprint(recoverValue))
		}
	}()

	if _, err := t.vt.Write(evt.Data); err != nil {
		return trace.Errorf("writing data to terminal: %w", err)
	}

	return nil
}

func (t *ttyThumbnailGenerator) handleTerminalResize(terminalSize string) error {
	if t.disabled {
		return nil
	}

	size, err := session.UnmarshalTerminalParams(terminalSize)
	if err != nil {
		return trace.Wrap(err, "parsing terminal size %q", terminalSize)
	}

	t.vt.Resize(size.W, size.H)

	return nil
}

func (t *ttyThumbnailGenerator) produceThumbnail() *pb.SessionRecordingThumbnail {
	if t.disabled {
		return nil
	}

	cols, rows := t.vt.Size()
	cursor := t.vt.Cursor()

	return &pb.SessionRecordingThumbnail{
		Svg:           terminal.VtToSvg(t.vt),
		Cols:          int32(cols),
		Rows:          int32(rows),
		CursorX:       int32(cursor.X),
		CursorY:       int32(cursor.Y),
		CursorVisible: t.vt.CursorVisible(),
	}
}
