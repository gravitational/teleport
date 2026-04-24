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
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// defaultE2ELogin is the default login that seeded session recordings are associated with.
	defaultE2ELogin = "root"
)

// kinds represents the different session recording types that can be seeded into the E2E environment. These values
// correspond to subdirectories under recordingsDir where .tar files are stored.
var kinds = []types.SessionKind{
	types.SSHSessionKind,
	types.KubernetesSessionKind,
	types.DatabaseSessionKind,
	types.WindowsDesktopSessionKind,
}

// recordingsDir is the directory relative to e2e/ that holds the session recording files.
const recordingsDir = "testdata/recordings"

// recording represents a discovered session recording.
type recording struct {
	SessionID string
	Path      string
	Kind      types.SessionKind
}

// recordingInstance represents a single seeded copy of a session recording:
// the source recording, the owning user, and the fresh session ID assigned to
// this copy on disk and in the audit log.
type recordingInstance struct {
	recording
	user          string
	destSessionID string
}

// seedRecordings copies session recording .tar files into Teleport's records directory and injects
// corresponding session.end audit events. Only recordings referenced in recordingOwners are seeded.
// Each owner of a recording gets its own copy with a freshly-generated session ID.
func (t *teleportInstance) seedRecordings(ctx context.Context, e2eDir, dataDir string) error {
	if len(t.recordingOwners) == 0 {
		return nil
	}

	recordsDir := filepath.Join(dataDir, "log", "records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		return fmt.Errorf("creating records dir: %w", err)
	}

	discovered, err := discoverRecordings(e2eDir)
	if err != nil {
		return fmt.Errorf("discovering recordings: %w", err)
	}

	// Index discovered recordings by session ID for lookup.
	byID := make(map[string]recording, len(discovered))
	for _, rec := range discovered {
		byID[rec.SessionID] = rec
	}

	// Sort recording IDs so that instances (and the timestamps we derive from
	// their position) are deterministic across runs.
	recIDs := make([]string, 0, len(t.recordingOwners))
	for id := range t.recordingOwners {
		recIDs = append(recIDs, id)
	}
	slices.Sort(recIDs)

	var instances []recordingInstance
	for _, recID := range recIDs {
		rec, ok := byID[recID]
		if !ok {
			return fmt.Errorf("recording %q referenced in user definition not found in testdata/recordings", recID)
		}

		for _, owner := range t.recordingOwners[recID] {
			instances = append(instances, recordingInstance{
				recording:     rec,
				user:          owner.user,
				destSessionID: owner.sessionID,
			})
		}
	}

	for _, inst := range instances {
		srcDir := filepath.Dir(inst.Path)

		for _, ext := range []string{".tar", ".metadata", ".thumbnail"} {
			src := filepath.Join(srcDir, inst.recording.SessionID+ext)
			dst := filepath.Join(recordsDir, inst.destSessionID+ext)

			if err := utils.CopyFile(src, dst, 0o644); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					if ext != ".tar" {
						continue
					}

					return fmt.Errorf("required recording file not found: %s", src)
				}

				return fmt.Errorf("copying %s: %w", inst.destSessionID+ext, err)
			}
		}
	}

	adjustedEvents, err := adjustRecordingEvents(ctx, instances)
	if err != nil {
		return fmt.Errorf("adjusting events: %w", err)
	}

	eventsLog := filepath.Join(dataDir, "log", "events.log")
	if err := waitForFile(ctx, eventsLog, 30*time.Second); err != nil {
		return fmt.Errorf("waiting for audit log: %w", err)
	}

	f, err := os.OpenFile(eventsLog, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("opening active audit log: %w", err)
	}
	defer f.Close()

	for _, line := range adjustedEvents {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("writing audit event: %w", err)
		}
	}

	slog.Info("seeded session recordings", "recordings", len(t.recordingOwners), "instances", len(instances))

	return nil
}

// discoverRecordings looks for session recordings under e2eDir for each session kind.
func discoverRecordings(e2eDir string) ([]recording, error) {
	var recordings []recording

	for _, st := range kinds {
		srcDir := filepath.Join(e2eDir, recordingsDir, string(st))

		tars, err := filepath.Glob(filepath.Join(srcDir, "*.tar"))
		if err != nil {
			return nil, fmt.Errorf("globbing %s: %w", srcDir, err)
		}

		for _, tarPath := range tars {
			sessionID := strings.TrimSuffix(filepath.Base(tarPath), ".tar")

			recordings = append(recordings, recording{
				SessionID: sessionID,
				Path:      tarPath,
				Kind:      st,
			})
		}
	}

	return recordings, nil
}

// adjustRecordingEvents reads, patches, and time-adjusts the session end event from each recording
// instance so that sessions appear recent (within the UI's default "today" search window).
func adjustRecordingEvents(ctx context.Context, instances []recordingInstance) ([]string, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var lines []string
	for _, inst := range instances {
		offset := time.Duration(len(lines)+1) * time.Second
		event, err := readAndPatchEvent(ctx, inst, startOfDay.Add(offset))
		if err != nil {
			return nil, fmt.Errorf("processing recording %s (user %s): %w", inst.destSessionID, inst.user, err)
		}

		marshaled, err := utils.FastMarshal(event)
		if err != nil {
			return nil, fmt.Errorf("marshaling event for %s: %w", inst.destSessionID, err)
		}

		lines = append(lines, string(marshaled))
	}

	return lines, nil
}

// readAndPatchEvent reads the session end event from a recording's .tar file, updates user/cluster/session
// fields for the E2E environment, and shifts timestamps so the session appears to end at newStop.
func readAndPatchEvent(ctx context.Context, inst recordingInstance, newStop time.Time) (apievents.AuditEvent, error) {
	f, err := os.Open(inst.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := events.NewProtoReader(f, nil)
	defer reader.Close()

	for {
		event, err := reader.Read(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("no session end event found")
			}
			return nil, err
		}

		switch e := event.(type) {
		case *apievents.SessionEnd:
			duration := e.EndTime.Sub(e.StartTime)
			if duration <= 0 {
				return nil, fmt.Errorf("invalid session duration for session %s", e.GetSessionID())
			}
			e.SessionMetadata.SessionID = inst.destSessionID
			e.User = inst.user
			e.Login = defaultE2ELogin
			e.Participants = []string{inst.user}
			e.ClusterName = clusterName
			e.UserClusterName = clusterName
			e.StartTime = newStop.Add(-duration)
			e.EndTime = newStop
			e.Time = newStop

			return e, nil

		case *apievents.DatabaseSessionEnd:
			duration := e.EndTime.Sub(e.StartTime)
			if duration <= 0 {
				return nil, fmt.Errorf("invalid session duration for session %s", e.GetSessionID())
			}
			e.SessionMetadata.SessionID = inst.destSessionID
			e.User = inst.user
			e.ClusterName = clusterName
			e.UserClusterName = clusterName
			e.StartTime = newStop.Add(-duration)
			e.EndTime = newStop
			e.Time = newStop

			return e, nil

		case *apievents.WindowsDesktopSessionEnd:
			duration := e.EndTime.Sub(e.StartTime)
			if duration <= 0 {
				return nil, fmt.Errorf("invalid session duration for session %s", e.GetSessionID())
			}
			e.SessionMetadata.SessionID = inst.destSessionID
			e.User = inst.user
			e.ClusterName = clusterName
			e.UserClusterName = clusterName
			e.StartTime = newStop.Add(-duration)
			e.EndTime = newStop
			e.Time = newStop

			return e, nil
		}
	}
}

// waitForFile polls until the given path exists or the timeout expires.
func waitForFile(ctx context.Context, path string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %s", path)
		case <-ticker.C:
		}
	}
}
