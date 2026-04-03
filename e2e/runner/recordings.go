/**
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

package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport/e2e/runner/recordings"
	"github.com/gravitational/teleport/lib/utils"
)

// seedRecordings copies session recording .tar files into Teleport's records directory and injects corresponding
// session.end audit events so that the Web UI's session recordings page shows content immediately after startup.
func seedRecordings(e2eDir, dataDir string) error {
	recordsDir := filepath.Join(dataDir, "log", "records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		return fmt.Errorf("creating records dir: %w", err)
	}

	discovered, err := recordings.Discover(e2eDir)
	if err != nil {
		return fmt.Errorf("discovering recordings: %w", err)
	}

	if len(discovered) == 0 {
		return fmt.Errorf("no .tar files found in any session type directory")
	}

	for _, recording := range discovered {
		for _, ext := range []string{".tar", ".metadata", ".thumbnail"} {
			src := filepath.Join(filepath.Dir(recording.Path), recording.SessionID+ext)
			dst := filepath.Join(recordsDir, recording.SessionID+ext)

			if err := utils.CopyFile(src, dst, 0o644); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					if ext != ".tar" {
						// metadata and thumbnails are optional
						continue
					}

					return fmt.Errorf("required recording file not found: %s", src)
				}

				return fmt.Errorf("copying %s: %w", recording.SessionID+ext, err)
			}
		}
	}

	adjustedEvents, err := recordings.AdjustEventTimestamps(e2eDir)
	if err != nil {
		return fmt.Errorf("adjusting events: %w", err)
	}

	eventsLog := filepath.Join(dataDir, "log", "events.log")
	f, err := os.OpenFile(eventsLog, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("opening active audit log: %w", err)
	}
	defer f.Close()

	for _, line := range adjustedEvents {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("writing audit event: %w", err)
		}
	}

	slog.Info("seeded session recordings", "total_recordings", len(discovered))

	return nil
}
