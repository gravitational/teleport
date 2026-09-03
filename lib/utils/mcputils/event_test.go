// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Based on the official SDK tests:
// https://github.com/modelcontextprotocol/go-sdk/blob/b4f957ff3c279051f9bcc88aa08e897add012a95/mcp/event_test.go

package mcputils

import (
	"strings"
	"testing"

	"github.com/gravitational/teleport"
)

func TestScanEvents(t *testing.T) {
	largeEventData := strings.Repeat("a", 1<<20)
	halfMaxEventData := strings.Repeat("a", teleport.MaxHTTPResponseSize/2)
	tests := []struct {
		name    string
		input   string
		want    []Event
		wantErr string
	}{
		{
			name:  "simple event",
			input: "event: message\nid: 1\ndata: hello\n\n",
			want: []Event{
				{Name: "message", ID: "1", Data: []byte("hello")},
			},
		},
		{
			name:  "multiple data lines",
			input: "data: line 1\ndata: line 2\n\n",
			want: []Event{
				{Data: []byte("line 1\nline 2")},
			},
		},
		{
			name:  "multiple events",
			input: "data: first\n\nevent: second\ndata: second\n\n",
			want: []Event{
				{Data: []byte("first")},
				{Name: "second", Data: []byte("second")},
			},
		},
		{
			name:  "no trailing newline",
			input: "data: hello",
			want: []Event{
				{Data: []byte("hello")},
			},
		},
		{
			name:  "large event",
			input: "data: " + largeEventData + "\n\n",
			want: []Event{
				{Data: []byte(largeEventData)},
			},
		},
		{
			name:    "malformed line",
			input:   "invalid line\n\n",
			wantErr: "malformed line",
		},
		{
			name:    "event larger than response limit across lines",
			input:   "data: " + halfMaxEventData + "\ndata: " + halfMaxEventData + "\n\n",
			wantErr: "event exceeded max size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			var got []Event
			var err error
			for e, err2 := range scanEvents(r) {
				if err2 != nil {
					err = err2
					break
				}
				got = append(got, e)
			}

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("scanEvents() got nil error, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("scanEvents() error = %q, want containing %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("scanEvents() returned unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("scanEvents() got %d events, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if g, w := got[i].Name, tt.want[i].Name; g != w {
					t.Errorf("event %d: name = %q, want %q", i, g, w)
				}
				if g, w := got[i].ID, tt.want[i].ID; g != w {
					t.Errorf("event %d: id = %q, want %q", i, g, w)
				}
				if g, w := string(got[i].Data), string(tt.want[i].Data); g != w {
					t.Errorf("event %d: data = %q, want %q", i, g, w)
				}
			}
		})
	}
}
