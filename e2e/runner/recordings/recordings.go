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

package recordings

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e2e/runner/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// defaultE2EUser is the default user that seeded session recordings are associated with.
	defaultE2EUser = "bob"
	// defaultE2ELogin is the default login that seeded session recordings are associated with.
	defaultE2ELogin = "root"
)

// recordingUserMap allows mapping specific session recordings to different users logins. The key is the session ID
// which corresponds to the username.
var recordingUserMap = map[string]string{}

// kinds represents the different session recording types that can be seeded into the E2E environment. These values
// correspond to subdirectories under recordingsDir where .tar files are stored.
var kinds = []types.SessionKind{
	types.SSHSessionKind,
	types.KubernetesSessionKind,
	types.DatabaseSessionKind,
	types.WindowsDesktopSessionKind,
}

const (
	// recordingsDir is the directory relative to e2e/ that holds the session recording files.
	recordingsDir = "testdata/recordings"
	// eventsFile is the name of the JSONL file containing session end events.
	eventsFile = "events.jsonl"
)

// Recording represents a discovered session recording.
type Recording struct {
	SessionID string
	Path      string
	Kind      types.SessionKind
}

// Discover looks for session recordings under e2eDir for each session kind.
func Discover(e2eDir string) ([]Recording, error) {
	var recordings []Recording

	for _, st := range kinds {
		srcDir := filepath.Join(e2eDir, recordingsDir, string(st))

		tars, err := filepath.Glob(filepath.Join(srcDir, "*.tar"))
		if err != nil {
			return nil, fmt.Errorf("globbing %s: %w", srcDir, err)
		}

		for _, tarPath := range tars {
			sessionID := strings.TrimSuffix(filepath.Base(tarPath), ".tar")

			recordings = append(recordings, Recording{
				SessionID: sessionID,
				Path:      tarPath,
				Kind:      st,
			})
		}
	}

	return recordings, nil
}

// PatchSessionEnd finds the session end event in the given recording, updates it with necessary fields for the E2E
// environment, and returns the marshaled event bytes.
func PatchSessionEnd(ctx context.Context, recording Recording) ([]byte, error) {
	endEvent, err := findSessionEndEvent(ctx, recording.Path)
	if err != nil {
		return nil, fmt.Errorf("finding session end event: %w", err)
	}

	if err := patchSessionEndEvent(recording.SessionID, endEvent); err != nil {
		return nil, fmt.Errorf("patching session end event: %w", err)
	}

	return utils.FastMarshal(endEvent)
}

// AdjustEventTimestamps reads events.jsonl and shifts timestamps so that sessions appear recent (within the UI's
// default "today" search window). The relative duration between start and stop is preserved. Multiple sessions are
// staggered 5 minutes apart.
func AdjustEventTimestamps(e2eDir string) ([]string, error) {
	f, err := os.Open(filepath.Join(e2eDir, recordingsDir, eventsFile))
	if err != nil {
		return nil, fmt.Errorf("reading events: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	now := time.Now().UTC()

	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}

		event, err := unmarshalSessionEnd(raw)
		if err != nil {
			return nil, err
		}

		offset := time.Hour + time.Duration(len(lines))*5*time.Minute
		newStop := now.Add(-offset)

		if err := adjustSessionTimes(event, newStop); err != nil {
			return nil, err
		}

		adjusted, err := utils.FastMarshal(event)
		if err != nil {
			return nil, fmt.Errorf("marshaling adjusted event: %w", err)
		}

		lines = append(lines, string(adjusted))
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning events: %w", err)
	}

	return lines, nil
}

// EventsPath returns the path to the events.jsonl file under e2eDir.
func EventsPath(e2eDir string) string {
	return filepath.Join(e2eDir, recordingsDir, eventsFile)
}

// adjustSessionTimes updates the start and stop times of the provided session end event to be relative to the current
// time while preserving the original session duration.
func adjustSessionTimes(event apievents.AuditEvent, newStop time.Time) error {
	switch e := event.(type) {
	case *apievents.SessionEnd:
		duration := e.EndTime.Sub(e.StartTime)
		if duration <= 0 {
			return fmt.Errorf("invalid session duration for session %s", e.GetSessionID())
		}

		e.StartTime = newStop.Add(-duration)
		e.EndTime = newStop
		e.Time = newStop

	case *apievents.DatabaseSessionEnd:
		duration := e.EndTime.Sub(e.StartTime)
		if duration <= 0 {
			return fmt.Errorf("invalid session duration for session %s", e.GetSessionID())
		}

		e.StartTime = newStop.Add(-duration)
		e.EndTime = newStop
		e.Time = newStop

	case *apievents.WindowsDesktopSessionEnd:
		duration := e.EndTime.Sub(e.StartTime)
		if duration <= 0 {
			return fmt.Errorf("invalid session duration for session %s", e.GetSessionID())
		}

		e.StartTime = newStop.Add(-duration)
		e.EndTime = newStop
		e.Time = newStop

	default:
		return fmt.Errorf("unexpected event type %T for timestamp adjustment", event)
	}

	return nil
}

// findSessionEndEvent reads the provided recording and returns the session end event.
func findSessionEndEvent(ctx context.Context, recordingPath string) (apievents.AuditEvent, error) {
	f, err := os.Open(recordingPath)
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

		switch event.GetType() {
		case events.SessionEndEvent,
			events.DatabaseSessionEndEvent,
			events.WindowsDesktopSessionEndEvent:
			return event, nil
		}
	}
}

// patchSessionEndEvent updates the provided session end event with fields necessary for the E2E environment, such as
// user and cluster name.
func patchSessionEndEvent(sessionID string, event apievents.AuditEvent) error {
	user := defaultE2EUser
	if mappedUser, ok := recordingUserMap[sessionID]; ok {
		user = mappedUser
	}

	switch e := event.(type) {
	case *apievents.SessionEnd: // SSH and Kubernetes
		e.User = user
		e.Login = defaultE2ELogin
		e.Participants = []string{user}
		e.ClusterName = teleport.ClusterName
		e.UserClusterName = teleport.ClusterName

	case *apievents.DatabaseSessionEnd:
		e.User = user
		e.ClusterName = teleport.ClusterName
		e.UserClusterName = teleport.ClusterName

	case *apievents.WindowsDesktopSessionEnd:
		e.User = user
		e.ClusterName = teleport.ClusterName
		e.UserClusterName = teleport.ClusterName

	default:
		return fmt.Errorf("unexpected event type %T", event)
	}

	return nil
}

// unmarshalSessionEnd peeks at the "event" field to determine the type, then unmarshals the raw JSON directly into the
// correct event struct.
func unmarshalSessionEnd(raw []byte) (apievents.AuditEvent, error) {
	var meta struct {
		Event string `json:"event"`
	}
	if err := utils.FastUnmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("reading event type: %w", err)
	}

	var event apievents.AuditEvent
	switch meta.Event {
	case events.SessionEndEvent:
		event = &apievents.SessionEnd{}
	case events.DatabaseSessionEndEvent:
		event = &apievents.DatabaseSessionEnd{}
	case events.WindowsDesktopSessionEndEvent:
		event = &apievents.WindowsDesktopSessionEnd{}
	default:
		return nil, fmt.Errorf("unexpected event type %q", meta.Event)
	}

	if err := utils.FastUnmarshal(raw, event); err != nil {
		return nil, fmt.Errorf("unmarshaling %s event: %w", meta.Event, err)
	}

	return event, nil
}
