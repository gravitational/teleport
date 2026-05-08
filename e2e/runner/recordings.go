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
	"encoding/json"
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

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	eventsLog := filepath.Join(dataDir, "log", "events.log")
	if err := waitForFile(ctx, eventsLog, 30*time.Second); err != nil {
		return fmt.Errorf("waiting for audit log: %w", err)
	}

	auditLog, err := os.OpenFile(eventsLog, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("opening active audit log: %w", err)
	}
	defer auditLog.Close()

	for i, inst := range instances {
		srcDir := filepath.Dir(inst.Path)

		// Copy recording files (.tar required, others optional).
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

		newStop := startOfDay.Add(time.Duration(i+1) * time.Second)
		event, originalStop, err := readAndPatchEvent(ctx, inst, newStop)
		if err != nil {
			return fmt.Errorf("processing recording %s (user %s): %w", inst.destSessionID, inst.user, err)
		}

		endEvent, err := utils.FastMarshal(event)
		if err != nil {
			return fmt.Errorf("marshaling event for %s: %w", inst.destSessionID, err)
		}

		if _, err := fmt.Fprintln(auditLog, string(endEvent)); err != nil {
			return fmt.Errorf("writing audit event: %w", err)
		}

		if err := adjustAndCopySummary(srcDir, recordsDir, inst.recording.SessionID, inst.destSessionID, endEvent, newStop.Sub(originalStop)); err != nil {
			return fmt.Errorf("adjusting summary for %s: %w", inst.destSessionID, err)
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

// adjustAndCopySummary reads a session summary JSON sidecar, replaces its session_end_event with the
// already-patched event, shifts inference timestamps by the given duration, and writes the result to dst.
func adjustAndCopySummary(srcDir, dstDir, srcSessionID, dstSessionID string, patchedEvent []byte, shift time.Duration) error {
	src := filepath.Join(srcDir, srcSessionID+".summary.json")
	dst := filepath.Join(dstDir, dstSessionID+".summary.json")

	raw, err := os.ReadFile(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("parsing summary: %w", err)
	}

	fields["session_end_event"], err = json.Marshal(json.RawMessage(patchedEvent))
	if err != nil {
		return fmt.Errorf("marshaling patched event: %w", err)
	}

	shiftTimestamp(fields, "inference_started_at", shift)
	shiftTimestamp(fields, "inference_finished_at", shift)

	adjusted, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshaling adjusted summary: %w", err)
	}

	return os.WriteFile(dst, adjusted, 0o644)
}

// readAndPatchEvent reads the session end event from a recording's .tar file, updates user/cluster/session
// fields for the E2E environment, and shifts timestamps so the session appears to end at newStop.
// It returns the patched event and the original stop time (for computing time shifts on sidecars).
func readAndPatchEvent(ctx context.Context, inst recordingInstance, newStop time.Time) (apievents.AuditEvent, time.Time, error) {
	f, err := os.Open(inst.Path)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer f.Close()

	reader := events.NewProtoReader(f, nil)
	defer reader.Close()

	for {
		event, err := reader.Read(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, time.Time{}, fmt.Errorf("no session end event found")
			}
			return nil, time.Time{}, err
		}

		switch e := event.(type) {
		case *apievents.SessionEnd:
			originalStop := e.EndTime
			duration := e.EndTime.Sub(e.StartTime)
			if duration <= 0 {
				return nil, time.Time{}, fmt.Errorf("invalid session duration for session %s", e.GetSessionID())
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

			return e, originalStop, nil

		case *apievents.DatabaseSessionEnd:
			originalStop := e.EndTime
			duration := e.EndTime.Sub(e.StartTime)
			if duration <= 0 {
				return nil, time.Time{}, fmt.Errorf("invalid session duration for session %s", e.GetSessionID())
			}
			e.SessionMetadata.SessionID = inst.destSessionID
			e.User = inst.user
			e.Participants = []string{inst.user}
			e.ClusterName = clusterName
			e.UserClusterName = clusterName
			e.StartTime = newStop.Add(-duration)
			e.EndTime = newStop
			e.Time = newStop

			return e, originalStop, nil

		case *apievents.WindowsDesktopSessionEnd:
			originalStop := e.EndTime
			duration := e.EndTime.Sub(e.StartTime)
			if duration <= 0 {
				return nil, time.Time{}, fmt.Errorf("invalid session duration for session %s", e.GetSessionID())
			}
			e.SessionMetadata.SessionID = inst.destSessionID
			e.User = inst.user
			e.Participants = []string{inst.user}
			e.ClusterName = clusterName
			e.UserClusterName = clusterName
			e.StartTime = newStop.Add(-duration)
			e.EndTime = newStop
			e.Time = newStop

			return e, originalStop, nil
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

func shiftTimestamp(fields map[string]json.RawMessage, key string, d time.Duration) {
	raw, ok := fields[key]
	if !ok {
		return
	}

	var t time.Time
	if err := json.Unmarshal(raw, &t); err != nil {
		return
	}

	fields[key], _ = json.Marshal(t.Add(d))
}
