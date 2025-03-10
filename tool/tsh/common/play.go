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

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// onPlay is used to interact with recorded sessions.
// It has several modes:
//
//  1. If --format is "pty" (the default), then the recorded
//     session is played back in the user's terminal.
//  2. Otherwise, `tsh play` is used to export a session from the
//     binary protobuf format into YAML or JSON.
//
// Each of these modes has two subcases:
// i) --session-id ends with ".tar" - tsh operates on a local
// file containing a previously downloaded session
//
// b) --session-id is the ID of a session - tsh operates on the
// session recording by connecting to the Teleport cluster
func onPlay(cf *CLIConf) error {
	if format := strings.ToLower(cf.Format); format == teleport.PTY {
		return playSession(cf)
	}
	if cf.PlaySpeed != "1x" {
		logger.WarnContext(cf.Context, "--speed is not applicable for formats other than pty")
	}
	return exportSession(cf)
}

var playbackSpeeds = map[string]float64{
	"0.5x": 0.5,
	"1x":   1.0,
	"2x":   2.0,
	"4x":   4.0,
	"8x":   8.0,
}

// playSession implements `tsh play` for the PTY format.
func playSession(cf *CLIConf) error {
	speed, ok := playbackSpeeds[cf.PlaySpeed]
	if !ok {
		speed = 1.0
	}

	isLocalFile := path.Ext(cf.SessionID) == ".tar"
	if isLocalFile {
		sid := sessionIDFromPath(cf.SessionID)
		if err := client.PlayFile(cf.Context, cf.SessionID, sid, speed, cf.NoWait); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := tc.Play(cf.Context, cf.SessionID, speed, cf.NoWait); err != nil {
		if trace.IsNotFound(err) {
			logger.DebugContext(cf.Context, "error playing session", "error", err)
			return trace.NotFound("Recording for session %s not found.", cf.SessionID)
		}
		return trace.Wrap(err)
	}
	return nil
}

func sessionIDFromPath(path string) string {
	fileName := filepath.Base(path)
	return strings.TrimSuffix(fileName, ".tar")
}

// exportSession implements `tsh play` for formats other than PTY
func exportSession(cf *CLIConf) error {
	format := strings.ToLower(cf.Format)
	isLocalFile := path.Ext(cf.SessionID) == ".tar"
	if isLocalFile {
		return trace.Wrap(exportFile(cf.Context, cf.SessionID, format))
	}

	switch format {
	case teleport.JSON, teleport.YAML, teleport.Text:
	default:
		// this should be unreachable since kingpin validates the format flag
		return trace.BadParameter("Invalid format %s", format)
	}

	sid, err := session.ParseID(cf.SessionID)
	if err != nil {
		return trace.BadParameter("'%v' is not a valid session ID (must be GUID)", cf.SessionID)
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterClient, err := tc.ConnectToCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	eventC, errC := clusterClient.AuthClient.StreamSessionEvents(metadata.WithSessionRecordingFormatContext(cf.Context, format), *sid, 0)

	var exporter sessionExporter
	switch format {
	case teleport.JSON:
		exporter = jsonSessionExporter{}
	case teleport.YAML:
		exporter = yamlSessionExporter{}
	case teleport.Text:
		exporter = textSessionExporter{}
	}

	exporter.WriteStart()
	defer exporter.WriteEnd()
	first := true

	for {
		select {
		case err := <-errC:
			return trace.Wrap(err)
		case event, ok := <-eventC:
			if !ok {
				return nil
			}
			// when playing from a file, id is not included, this
			// makes the outputs otherwise identical
			event.SetID("")

			if first {
				first = false
			} else {
				exporter.WriteSeparator()
			}

			if err := exporter.WriteEvent(event); err != nil {
				return trace.Wrap(err)
			}

		}
	}
}

type sessionExporter interface {
	WriteStart() error
	WriteEnd() error
	WriteSeparator() error
	WriteEvent(evt apievents.AuditEvent) error
}

type jsonSessionExporter struct{}

func (jsonSessionExporter) WriteStart() error {
	_, err := fmt.Println("[")
	return err
}

func (jsonSessionExporter) WriteEnd() error {
	_, err := fmt.Println("]")
	return err
}

func (jsonSessionExporter) WriteSeparator() error {
	_, err := fmt.Print(",\n")
	return err
}

func (jsonSessionExporter) WriteEvent(evt apievents.AuditEvent) error {
	b, err := json.MarshalIndent(evt, "    ", "    ")
	if err != nil {
		return trace.Wrap(err)
	}

	// JSON prefix does not apply to the first line, so add it manually
	os.Stdout.Write([]byte("    "))

	_, err = os.Stdout.Write(bytes.TrimSpace(b))
	return trace.Wrap(err)
}

type yamlSessionExporter struct{}

func (yamlSessionExporter) WriteStart() error { return nil }

func (yamlSessionExporter) WriteEnd() error { return nil }

func (yamlSessionExporter) WriteSeparator() error {
	_, err := fmt.Println("---")
	return err
}

func (yamlSessionExporter) WriteEvent(evt apievents.AuditEvent) error {
	b, err := yaml.Marshal(evt)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(b)
	return err
}

type textSessionExporter struct{}

func (textSessionExporter) WriteStart() error     { return nil }
func (textSessionExporter) WriteEnd() error       { return nil }
func (textSessionExporter) WriteSeparator() error { return nil }

func (textSessionExporter) WriteEvent(evt apievents.AuditEvent) error {
	printEvent, ok := evt.(*apievents.SessionPrint)
	if !ok {
		return nil
	}

	_, err := os.Stdout.Write(printEvent.Data)
	return trace.Wrap(err)
}

// exportFile converts the binary protobuf events from the file
// identified by path to text (JSON/YAML) and writes the converted
// events to standard out.
func exportFile(ctx context.Context, path string, format string) error {
	f, err := os.Open(path)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	err = events.Export(ctx, f, os.Stdout, format)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
