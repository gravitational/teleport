// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package asciinema

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// header, see docs: https://github.com/asciinema/asciinema/blob/develop/doc/asciicast-v2.md
type header struct {
	// Mandatory fields
	Version int `json:"version"`
	Width   int `json:"width"`
	Height  int `json:"height"`

	// Optional fields; there are more, but we have no good way of filling them.
	Timestamp int64  `json:"timestamp,omitempty"`
	Command   string `json:"command,omitempty"`
	Title     string `json:"title,omitempty"`
}

func GetSessionStream(ctx context.Context, tc *client.TeleportClient, sessionID string) ([]byte, error) {
	// connect to the auth server (site) who made the recording
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	site := proxyClient.CurrentCluster()

	var stream []byte
	sid, err := session.ParseID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("'%v' is not a valid session ID (must be GUID)", sid)
	}

	// read the stream into a buffer:
	for {
		tmp, err := site.GetSessionChunk(defaults.Namespace, *sid, len(stream), events.MaxChunkBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(tmp) == 0 {
			break
		}
		stream = append(stream, tmp...)
	}
	return stream, nil
}

func getTerminalSize(event events.EventFields) (width int, height int, err error) {
	parts := strings.Split(event.GetString("size"), ":")
	if len(parts) != 2 {
		return 0, 0, trace.BadParameter("parse error, wrong number of parts: %v", len(parts))
	}
	height, err0 := strconv.Atoi(parts[0])
	width, err1 := strconv.Atoi(parts[1])
	err = trace.NewAggregate(err0, err1)
	return
}

type Kind rune

const (
	KindOutput = 'o'
	KindInput  = 'i'
	KindMarker = 'm'
	KindResize = 'r'
)

func formatLine(timestamp float64, kind Kind, data string) ([]byte, error) {
	switch kind {
	case KindOutput, KindInput, KindMarker, KindResize:
	// all good
	default:
		return nil, trace.BadParameter("invalid event kind: %v, expected one of: o,i,m,r.", kind)
	}

	arr := []any{timestamp, string(kind), data}
	out, err := json.Marshal(arr)
	return out, trace.Wrap(err)
}

func printLine(w io.Writer, out []byte) {
	_, _ = w.Write(out)
	_, _ = w.Write([]byte("\n"))
}

// WriteAsciinema writes a session using asciicast format (v2): https://github.com/asciinema/asciinema/blob/develop/doc/asciicast-v2.md
func WriteAsciinema(w io.Writer, sessionEvents []events.EventFields, stream []byte) error {
	if len(sessionEvents) < 1 {
		return trace.BadParameter("empty session recording")
	}

	start := sessionEvents[0]

	if start.GetType() != events.SessionStartEvent {
		return trace.BadParameter("expected first event with type events.SessionStartEvent, got %q", start.GetType())
	}

	// print header
	h := header{
		Version: 2,
		Width:   0,
		Height:  0,
		Title: fmt.Sprintf("Session ID=%v, user %v, host %v (%v)",
			start.GetString("sid"),
			start.GetString("user"),
			start.GetString("server_hostname"),
			start.GetString("server_id"),
		),
		Timestamp: start.GetTimestamp().Unix(),
		Command:   strings.Join(start.GetStrings("initial_command"), " "),
	}
	var err error
	h.Height, h.Width, err = getTerminalSize(start)
	if err != nil {
		// use defaults
		h.Height = teleport.DefaultTerminalHeight
		h.Width = teleport.DefaultTerminalWidth
	}

	out, err := json.Marshal(h)
	if err != nil {
		return trace.Wrap(err)
	}
	printLine(w, out)

	startTime := start.GetTimestamp()

	// print events
	for _, event := range sessionEvents {
		// we can get ms from event.GetInt("ms"), but this gives us more precision.
		eventTime := event.GetTimestamp()
		timestamp := eventTime.Sub(startTime).Seconds()

		switch event.GetString(events.EventType) {
		// 'print' event (output)
		case events.SessionPrintEvent:
			offset := event.GetInt("offset")
			bytes := event.GetInt("bytes")
			data := stream[offset : offset+bytes]
			out, err = formatLine(timestamp, KindOutput, string(data))
			if err != nil {
				// TODO: warn about errors, but continue
				continue
			}
			printLine(w, out)

		// resize terminal event; session start is handled by the header.
		case events.ResizeEvent:
			width, height, err := getTerminalSize(event)
			if err != nil {
				continue
			}
			out, err = formatLine(timestamp, KindResize, fmt.Sprintf("%vx%v", width, height))
			if err != nil {
				// TODO: warn about errors, but continue
				continue
			}
			printLine(w, out)
		default:
			continue
		}
	}

	return nil
}
