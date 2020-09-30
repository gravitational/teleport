/*
Copyright 2018-2020 Gravitational, Inc.

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
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/session"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"gopkg.in/check.v1"
)

// UploadDownload tests uploads and downloads
func UploadDownload(t *testing.T, handler events.MultipartHandler) {
	val := "hello, how is it going? this is the uploaded file"
	id := session.NewID()
	_, err := handler.Upload(context.TODO(), id, bytes.NewBuffer([]byte(val)))
	assert.Nil(t, err)

	f, err := ioutil.TempFile("", string(id))
	assert.Nil(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = handler.Download(context.TODO(), id, f)
	assert.Nil(t, err)

	_, err = f.Seek(0, 0)
	assert.Nil(t, err)

	data, err := ioutil.ReadAll(f)
	assert.Nil(t, err)
	assert.Equal(t, string(data), val)
}

// DownloadNotFound tests handling of the scenario when download is not found
func DownloadNotFound(t *testing.T, handler events.MultipartHandler) {
	id := session.NewID()

	f, err := ioutil.TempFile("", string(id))
	assert.Nil(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = handler.Download(context.TODO(), id, f)
	fixtures.AssertNotFound(t, err)
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
	err := s.Log.EmitAuditEventLegacy(events.UserLocalLoginE, events.EventFields{
		events.LoginMethod:        events.LoginMethodSAML,
		events.AuthAttemptSuccess: true,
		events.EventUser:          "bob",
		events.EventTime:          s.Clock.Now().UTC(),
	})
	c.Assert(err, check.IsNil)

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
	history, err = s.Log.GetSessionEvents(defaults.Namespace, sessionID, 0, false)
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
