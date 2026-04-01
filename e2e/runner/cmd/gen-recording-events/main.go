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

// gen-recording-events scans e2e/testdata/recordings/{ssh,k8s,desktop,db}/*.tar, reads each recording using teleport's
// ProtoReader, extracts the session end event, and writes all of them to e2e/testdata/recordings/events.jsonl.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gravitational/teleport/e2e/runner/recordings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	e2eDir := ".."
	ctx := context.Background()

	outPath := recordings.EventsPath(e2eDir)
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outPath, err)
	}
	defer out.Close()

	discovered, err := recordings.Discover(e2eDir)
	if err != nil {
		return fmt.Errorf("discovering recordings: %w", err)
	}

	if len(discovered) == 0 {
		return fmt.Errorf("no .tar files found in any session type directory")
	}

	for _, recording := range discovered {
		line, err := recordings.PatchSessionEnd(ctx, recording)
		if err != nil {
			return fmt.Errorf("patching session end event for %s: %w", recording.SessionID, err)
		}

		if _, err := fmt.Fprintln(out, string(line)); err != nil {
			return fmt.Errorf("writing event: %w", err)
		}
	}

	fmt.Printf("generated %s with %d event(s)\n", outPath, len(discovered))

	return nil
}
