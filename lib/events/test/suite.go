/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/session"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

var _ = fmt.Printf

// HandlerSuite is a conformance test suite to verify external UploadHandlers
// behavior.
type HandlerSuite struct {
	Handler events.UploadHandler
}

func (s *HandlerSuite) UploadDownload(c *check.C) {
	val := "hello, how is it going? this is the uploaded file"
	id := session.NewID()
	_, err := s.Handler.Upload(context.TODO(), id, bytes.NewBuffer([]byte(val)))
	c.Assert(err, check.IsNil)

	dir := c.MkDir()
	f, err := os.Create(filepath.Join(dir, string(id)))
	c.Assert(err, check.IsNil)
	defer f.Close()

	err = s.Handler.Download(context.TODO(), id, f)
	c.Assert(err, check.IsNil)

	_, err = f.Seek(0, 0)
	c.Assert(err, check.IsNil)

	data, err := ioutil.ReadAll(f)
	c.Assert(err, check.IsNil)
	c.Assert(string(data), check.Equals, val)
}

// CancelUpload makes sure a file upload can be canceled and the file is
// not uploaded to storage.
func (s *HandlerSuite) CancelUpload(c *check.C) {
	uploadCh := make(chan error)
	sessionID := session.NewID()

	// Create a temporary 100 MB file with junk in it.
	tmpfile, err := ioutil.TempFile("", "teleport-large-file")
	c.Assert(err, check.IsNil)
	defer os.Remove(tmpfile.Name())
	err = tmpfile.Truncate(100000000)
	c.Assert(err, check.IsNil)
	err = tmpfile.Close()
	c.Assert(err, check.IsNil)

	// Get a handle to the large file created before.
	reader, err := os.Open(tmpfile.Name())
	c.Assert(err, check.IsNil)
	defer reader.Close()

	// Start uploading the file in another goroutine and cancel it right away.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_, err := s.Handler.Upload(ctx, sessionID, reader)
		uploadCh <- err

	}()
	cancel()

	// Wait for upload to return an error.
	select {
	case err := <-uploadCh:
		c.Assert(err, check.NotNil)
	case <-time.After(60 * time.Second):
		c.Fatalf("Timed out waiting for upload to cancel.")
	}

	// Make sure the file was not uploaded.
	err = s.Handler.Download(context.Background(), sessionID, &writeAtDiscarder{})
	c.Assert(err, check.NotNil)
}

func (s *HandlerSuite) DownloadNotFound(c *check.C) {
	id := session.NewID()

	dir := c.MkDir()
	f, err := os.Create(filepath.Join(dir, string(id)))
	c.Assert(err, check.IsNil)
	defer f.Close()

	err = s.Handler.Download(context.TODO(), id, f)
	fixtures.ExpectNotFound(c, err)
}

// EventsSuite is a conformance test suite to verify external event backends
type EventsSuite struct {
	Log        events.IAuditLog
	Clock      clockwork.Clock
	QueryDelay time.Duration
}

// SessionEventsCRUD covers session events
func (s *EventsSuite) SessionEventsCRUD(c *check.C) {
	// Bob has logged in
	err := s.Log.EmitAuditEvent(events.UserLocalLogin, events.EventFields{
		events.LoginMethod:        events.LoginMethodSAML,
		events.AuthAttemptSuccess: true,
		events.EventUser:          "bob",
		events.EventTime:          s.Clock.Now().UTC(),
	})

	// For eventually consistent queries
	if s.QueryDelay != 0 {
		time.Sleep(s.QueryDelay)
	}

	history, err := s.Log.SearchEvents(s.Clock.Now().Add(-1*time.Hour), s.Clock.Now().Add(time.Hour), "", 100)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 1)

	// start the session and emit data stream to it and wrap it up
	sessionID := session.NewID()
	err = s.Log.PostSessionSlice(events.SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: string(sessionID),
		Chunks: []*events.SessionChunk{
			// start the seession
			&events.SessionChunk{
				Time:       s.Clock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  events.SessionStartEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "bob"}),
			},
			// emitting session end event should close the session
			&events.SessionChunk{
				Time:       s.Clock.Now().Add(time.Hour).UTC().UnixNano(),
				EventIndex: 4,
				EventType:  events.SessionEndEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "bob"}),
			},
		},
		Version: events.V2,
	})
	c.Assert(err, check.IsNil)

	// read the session event
	history, err = s.Log.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 0, false)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 2)
	c.Assert(history[0].GetString(events.EventType), check.Equals, events.SessionStartEvent)
	c.Assert(history[1].GetString(events.EventType), check.Equals, events.SessionEndEvent)

	history, err = s.Log.SearchSessionEvents(s.Clock.Now().Add(-1*time.Hour), s.Clock.Now().Add(2*time.Hour), 100)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 2)

	history, err = s.Log.SearchSessionEvents(s.Clock.Now().Add(-1*time.Hour), s.Clock.Now().Add(time.Hour-time.Second), 100)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 1)
}

func marshal(f events.EventFields) []byte {
	data, err := json.Marshal(f)
	if err != nil {
		panic(err)
	}
	return data
}

type writeAtDiscarder struct {
}

func (w *writeAtDiscarder) WriteAt(p []byte, off int64) (n int, err error) {
	return len(p), nil
}
