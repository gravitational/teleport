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

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
)

// UploadDownload tests uploads and downloads
func UploadDownload(t *testing.T, handler events.MultipartHandler) {
	val := "hello, how is it going? this is the uploaded file"
	id := session.NewID()
	_, err := handler.Upload(context.TODO(), id, bytes.NewBuffer([]byte(val)))
	require.Nil(t, err)

	f, err := ioutil.TempFile("", string(id))
	require.Nil(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = handler.Download(context.TODO(), id, f)
	require.Nil(t, err)

	_, err = f.Seek(0, 0)
	require.Nil(t, err)

	data, err := ioutil.ReadAll(f)
	require.Nil(t, err)
	require.Equal(t, string(data), val)
}

// DownloadNotFound tests handling of the scenario when download is not found
func DownloadNotFound(t *testing.T, handler events.MultipartHandler) {
	id := session.NewID()

	f, err := ioutil.TempFile("", string(id))
	require.Nil(t, err)
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

// EventPagination covers event search pagination.
func (s *EventsSuite) EventPagination(c *check.C) {
	// This serves no special purpose except to make querying easier.
	baseTime := time.Date(2019, time.May, 10, 14, 43, 0, 0, time.UTC)

	names := []string{"bob", "jack", "daisy", "evan"}

	for i, name := range names {
		err := s.Log.EmitAuditEventLegacy(events.UserLocalLoginE, events.EventFields{
			events.LoginMethod:        events.LoginMethodSAML,
			events.AuthAttemptSuccess: true,
			events.EventUser:          name,
			events.EventTime:          baseTime.Add(time.Second * time.Duration(i)),
		})
		c.Assert(err, check.IsNil)
	}

	toTime := baseTime.Add(time.Hour)
	var arr []apievents.AuditEvent
	var err error
	var checkpoint string

	err = utils.RetryStaticFor(time.Minute*5, time.Second*5, func() error {
		arr, checkpoint, err = s.Log.SearchEvents(baseTime, toTime, apidefaults.Namespace, nil, 100, types.EventOrderAscending, checkpoint)
		return err
	})
	c.Assert(err, check.IsNil)
	c.Assert(arr, check.HasLen, 4)
	c.Assert(checkpoint, check.Equals, "")

	for _, name := range names {
		arr, checkpoint, err = s.Log.SearchEvents(baseTime, toTime, apidefaults.Namespace, nil, 1, types.EventOrderAscending, checkpoint)
		c.Assert(err, check.IsNil)
		c.Assert(arr, check.HasLen, 1)
		event, ok := arr[0].(*apievents.UserLogin)
		c.Assert(ok, check.Equals, true)
		c.Assert(name, check.Equals, event.User)
	}
	if checkpoint != "" {
		arr, checkpoint, err = s.Log.SearchEvents(baseTime, toTime, apidefaults.Namespace, nil, 1, types.EventOrderAscending, checkpoint)
		c.Assert(err, check.IsNil)
		c.Assert(arr, check.HasLen, 0)
	}
	c.Assert(checkpoint, check.Equals, "")

	for _, i := range []int{0, 2} {
		nameA := names[i]
		nameB := names[i+1]
		arr, checkpoint, err = s.Log.SearchEvents(baseTime, toTime, apidefaults.Namespace, nil, 2, types.EventOrderAscending, checkpoint)
		c.Assert(err, check.IsNil)
		c.Assert(arr, check.HasLen, 2)
		eventA, okA := arr[0].(*apievents.UserLogin)
		eventB, okB := arr[1].(*apievents.UserLogin)
		c.Assert(okA, check.Equals, true)
		c.Assert(okB, check.Equals, true)
		c.Assert(nameA, check.Equals, eventA.User)
		c.Assert(nameB, check.Equals, eventB.User)
	}
	if checkpoint != "" {
		arr, checkpoint, err = s.Log.SearchEvents(baseTime, toTime, apidefaults.Namespace, nil, 1, types.EventOrderAscending, checkpoint)
		c.Assert(err, check.IsNil)
		c.Assert(arr, check.HasLen, 0)
	}
	c.Assert(checkpoint, check.Equals, "")

	for i := len(names) - 1; i >= 0; i-- {
		arr, checkpoint, err = s.Log.SearchEvents(baseTime, toTime, apidefaults.Namespace, nil, 1, types.EventOrderDescending, checkpoint)
		c.Assert(err, check.IsNil)
		c.Assert(arr, check.HasLen, 1)
		event, ok := arr[0].(*apievents.UserLogin)
		c.Assert(ok, check.Equals, true)
		c.Assert(names[i], check.Equals, event.User)
	}
	if checkpoint != "" {
		arr, checkpoint, err = s.Log.SearchEvents(baseTime, toTime, apidefaults.Namespace, nil, 1, types.EventOrderDescending, checkpoint)
		c.Assert(err, check.IsNil)
		c.Assert(arr, check.HasLen, 0)
	}
	c.Assert(checkpoint, check.Equals, "")
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

	var history []apievents.AuditEvent

	err = utils.RetryStaticFor(time.Minute*5, time.Second*5, func() error {
		history, _, err = s.Log.SearchEvents(s.Clock.Now().Add(-1*time.Hour), s.Clock.Now().Add(time.Hour), apidefaults.Namespace, nil, 100, types.EventOrderAscending, "")
		return err
	})
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 1)

	// start the session and emit data stream to it and wrap it up
	sessionID := session.NewID()
	err = s.Log.PostSessionSlice(events.SessionSlice{
		Namespace: apidefaults.Namespace,
		SessionID: string(sessionID),
		Chunks: []*events.SessionChunk{
			// start the seession
			{
				Time:       s.Clock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  events.SessionStartEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "bob"}),
			},
			// emitting session end event should close the session
			{
				Time:       s.Clock.Now().Add(time.Hour).UTC().UnixNano(),
				EventIndex: 4,
				EventType:  events.SessionEndEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "bob", events.SessionParticipants: []string{"bob", "alice"}}),
			},
		},
		Version: events.V2,
	})
	c.Assert(err, check.IsNil)

	// read the session event
	historyEvents, err := s.Log.GetSessionEvents(apidefaults.Namespace, sessionID, 0, false)
	c.Assert(err, check.IsNil)
	c.Assert(historyEvents, check.HasLen, 2)
	c.Assert(historyEvents[0].GetString(events.EventType), check.Equals, events.SessionStartEvent)
	c.Assert(historyEvents[1].GetString(events.EventType), check.Equals, events.SessionEndEvent)

	history, _, err = s.Log.SearchSessionEvents(s.Clock.Now().Add(-1*time.Hour), s.Clock.Now().Add(2*time.Hour), 100, types.EventOrderAscending, "", nil)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 1)

	withParticipant := func(participant string) *types.WhereExpr {
		return &types.WhereExpr{Contains: types.WhereExpr2{
			L: &types.WhereExpr{Field: events.SessionParticipants},
			R: &types.WhereExpr{Literal: participant},
		}}
	}

	history, _, err = s.Log.SearchSessionEvents(s.Clock.Now().Add(-1*time.Hour), s.Clock.Now().Add(2*time.Hour), 100, types.EventOrderAscending, "", withParticipant("alice"))
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 1)

	history, _, err = s.Log.SearchSessionEvents(s.Clock.Now().Add(-1*time.Hour), s.Clock.Now().Add(2*time.Hour), 100, types.EventOrderAscending, "", withParticipant("cecile"))
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 0)

	history, _, err = s.Log.SearchSessionEvents(s.Clock.Now().Add(-1*time.Hour), s.Clock.Now().Add(time.Hour-time.Second), 100, types.EventOrderAscending, "", nil)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 0)
}

func marshal(f events.EventFields) []byte {
	data, err := json.Marshal(f)
	if err != nil {
		panic(err)
	}
	return data
}
