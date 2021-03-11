/*
Copyright 2015-2018 Gravitational, Inc.

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

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/check.v1"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type AuditTestSuite struct {
	dataDir string
}

// bootstrap check
func TestAuditLog(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&AuditTestSuite{})

func (a *AuditTestSuite) TearDownSuite(c *check.C) {
	os.RemoveAll(a.dataDir)
}

// creates a file-based audit log and returns a proper *AuditLog pointer
// instead of the usual IAuditLog interface
func (a *AuditTestSuite) makeLog(c *check.C, dataDir string, recordSessions bool) (*AuditLog, error) {
	return a.makeLogWithClock(c, dataDir, recordSessions, nil)
}

// creates a file-based audit log and returns a proper *AuditLog pointer
// instead of the usual IAuditLog interface
func (a *AuditTestSuite) makeLogWithClock(c *check.C, dataDir string, recordSessions bool, clock clockwork.Clock) (*AuditLog, error) {
	handler, err := NewLegacyHandler(LegacyHandlerConfig{
		Handler: NewMemoryUploader(),
		Dir:     dataDir,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:        dataDir,
		RecordSessions: recordSessions,
		ServerID:       "server1",
		Clock:          clock,
		UIDGenerator:   utils.NewFakeUID(),
		UploadHandler:  handler,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return alog, nil
}

func (a *AuditTestSuite) SetUpTest(c *check.C) {
	a.dataDir = c.MkDir()
}

func (a *AuditTestSuite) TestNew(c *check.C) {
	alog, err := a.makeLog(c, a.dataDir, true)
	c.Assert(err, check.IsNil)
	// close twice:
	c.Assert(alog.Close(), check.IsNil)
	c.Assert(alog.Close(), check.IsNil)
}

// TestSessionsOnOneAuthServer tests scenario when there are two auth servers
// and session is recorded on the first one
func (a *AuditTestSuite) TestSessionsOnOneAuthServer(c *check.C) {
	fakeClock := clockwork.NewFakeClock()

	uploader := NewMemoryUploader()

	alog, err := NewAuditLog(AuditLogConfig{
		Clock:          fakeClock,
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server1",
		UploadHandler:  uploader,
	})
	c.Assert(err, check.IsNil)

	alog2, err := NewAuditLog(AuditLogConfig{
		Clock:          fakeClock,
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server2",
		UploadHandler:  uploader,
	})
	c.Assert(err, check.IsNil)

	uploadDir := c.MkDir()
	err = os.MkdirAll(filepath.Join(uploadDir, "upload", "sessions", defaults.Namespace), 0755)
	c.Assert(err, check.IsNil)
	sessionID := string(session.NewID())
	forwarder, err := NewForwarder(ForwarderConfig{
		Namespace:      defaults.Namespace,
		SessionID:      session.ID(sessionID),
		ServerID:       teleport.ComponentUpload,
		DataDir:        uploadDir,
		RecordSessions: true,
		ForwardTo:      alog,
		Clock:          fakeClock,
	})
	c.Assert(err, check.IsNil)

	// start the session and emit data stream to it
	firstMessage := []byte("hello")
	err = forwarder.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// start the session
			{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
			// type "hello" into session "100"
			{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 1,
				ChunkIndex: 0,
				Offset:     0,
				EventType:  SessionPrintEvent,
				Data:       firstMessage,
			},
			// emitting session end event should close the session
			{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 4,
				EventType:  SessionEndEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
		},
		Version: V3,
	})
	c.Assert(err, check.IsNil)
	c.Assert(forwarder.Close(), check.IsNil)

	upload(c, uploadDir, fakeClock, alog)

	// does not matter which audit server is accessed the results should be the same
	for _, a := range []*AuditLog{alog, alog2} {
		// read the session bytes
		history, err := a.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 0, true)
		c.Assert(err, check.IsNil)
		c.Assert(history, check.HasLen, 3)

		// make sure offsets were properly set (0 for the first event and 5 bytes for hello):
		c.Assert(history[1][SessionByteOffset], check.Equals, float64(0))
		c.Assert(history[1][SessionEventTimestamp], check.Equals, float64(0))

		// fetch all bytes
		buff, err := a.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 0, 5000)
		c.Assert(err, check.IsNil)
		c.Assert(string(buff), check.Equals, string(firstMessage))

		// with offset
		buff, err = a.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 2, 5000)
		c.Assert(err, check.IsNil)
		c.Assert(string(buff), check.Equals, string(firstMessage[2:]))
	}
}

func upload(c *check.C, uploadDir string, clock clockwork.Clock, auditLog IAuditLog) {
	// start uploader process
	eventsC := make(chan UploadEvent, 100)
	uploader, err := NewUploader(UploaderConfig{
		ServerID:   "upload",
		DataDir:    uploadDir,
		Clock:      clock,
		Namespace:  defaults.Namespace,
		Context:    context.TODO(),
		ScanPeriod: 100 * time.Millisecond,
		AuditLog:   auditLog,
		EventsC:    eventsC,
	})
	c.Assert(err, check.IsNil)

	// scanner should upload the events
	err = uploader.Scan()
	c.Assert(err, check.IsNil)

	select {
	case event := <-eventsC:
		c.Assert(event, check.NotNil)
		c.Assert(event.Error, check.IsNil)
	case <-time.After(time.Second):
		c.Fatalf("Timeout wating for the upload event")
	}
}

func (a *AuditTestSuite) TestSessionRecordingOff(c *check.C) {
	now := time.Now().In(time.UTC).Round(time.Second)

	// create audit log with session recording disabled
	fakeClock := clockwork.NewFakeClockAt(now)

	alog, err := NewAuditLog(AuditLogConfig{
		Clock:          fakeClock,
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server1",
		UploadHandler:  NewMemoryUploader(),
	})
	c.Assert(err, check.IsNil)

	username := "alice"
	sessionID := string(session.NewID())

	uploadDir := c.MkDir()
	err = os.MkdirAll(filepath.Join(uploadDir, "upload", "sessions", defaults.Namespace), 0755)
	c.Assert(err, check.IsNil)
	forwarder, err := NewForwarder(ForwarderConfig{
		Namespace:      defaults.Namespace,
		SessionID:      session.ID(sessionID),
		ServerID:       teleport.ComponentUpload,
		DataDir:        uploadDir,
		RecordSessions: false,
		ForwardTo:      alog,
		Clock:          fakeClock,
	})
	c.Assert(err, check.IsNil)

	// start the session and emit data stream to it
	firstMessage := []byte("hello")
	err = forwarder.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// start the session
			{
				Time:       alog.Clock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: username}),
			},
			// type "hello" into session "100"
			{
				Time:       alog.Clock.Now().UTC().UnixNano(),
				EventIndex: 1,
				ChunkIndex: 0,
				Offset:     0,
				EventType:  SessionPrintEvent,
				Data:       firstMessage,
			},
			// end the session
			{
				Time:       alog.Clock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionEndEvent,
				Data:       marshal(EventFields{EventLogin: username}),
			},
		},
		Version: V3,
	})
	c.Assert(err, check.IsNil)
	c.Assert(forwarder.Close(), check.IsNil)

	upload(c, uploadDir, fakeClock, alog)

	// get all events from the audit log, should have two session event and one upload event
	found, err := alog.SearchEvents(now.Add(-time.Hour), now.Add(time.Hour), "", 0)
	c.Assert(err, check.IsNil)
	c.Assert(found, check.HasLen, 3)
	c.Assert(found[0].GetString(EventLogin), check.Equals, username)
	c.Assert(found[1].GetString(EventLogin), check.Equals, username)

	// inspect the session log for "200", should have two events
	history, err := alog.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 0, true)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 2)

	// try getting the session stream, should get an error
	_, err = alog.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 0, 5000)
	c.Assert(err, check.NotNil)
}

func (a *AuditTestSuite) TestBasicLogging(c *check.C) {
	// create audit log, write a couple of events into it, close it
	clock := clockwork.NewFakeClock()
	alog, err := a.makeLogWithClock(c, a.dataDir, true, clock)
	c.Assert(err, check.IsNil)

	// emit regular event:
	err = alog.EmitAuditEventLegacy(Event{Name: "user.joined"}, EventFields{"apples?": "yes"})
	c.Assert(err, check.IsNil)
	logfile := alog.localLog.file.Name()
	c.Assert(alog.Close(), check.IsNil)

	// read back what's been written:
	bytes, err := ioutil.ReadFile(logfile)
	c.Assert(err, check.IsNil)
	c.Assert(string(bytes), check.Equals,
		fmt.Sprintf("{\"apples?\":\"yes\",\"event\":\"user.joined\",\"time\":\"%s\",\"uid\":\"%s\"}\n",
			clock.Now().Format(time.RFC3339), fixtures.UUID))
}

// TestLogRotation makes sure that logs are rotated
// on the day boundary and symlinks are created and updated
func (a *AuditTestSuite) TestLogRotation(c *check.C) {
	start := time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC)
	clock := clockwork.NewFakeClockAt(start)

	// create audit log, write a couple of events into it, close it
	alog, err := a.makeLogWithClock(c, a.dataDir, true, clock)
	c.Assert(err, check.IsNil)
	defer func() {
		c.Assert(alog.Close(), check.IsNil)
	}()

	for _, duration := range []time.Duration{0, time.Hour * 25} {
		// advance time and emit audit event
		now := start.Add(duration)
		clock.Advance(duration)

		// emit regular event:
		err = alog.EmitAuditEventLegacy(Event{Name: "user.joined"}, EventFields{"apples?": "yes"})
		c.Assert(err, check.IsNil)
		logfile := alog.localLog.file.Name()

		// make sure that file has the same date as the event
		dt, err := parseFileTime(filepath.Base(logfile))
		c.Assert(err, check.IsNil)
		c.Assert(dt, check.Equals, time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))

		// read back what's been written:
		bytes, err := ioutil.ReadFile(logfile)
		c.Assert(err, check.IsNil)
		contents := fmt.Sprintf("{\"apples?\":\"yes\",\"event\":\"user.joined\",\"time\":\"%s\",\"uid\":\"%s\"}\n", now.Format(time.RFC3339), fixtures.UUID)
		c.Assert(string(bytes), check.Equals, contents)

		// read back the contents using symlink
		bytes, err = ioutil.ReadFile(filepath.Join(alog.localLog.SymlinkDir, SymlinkFilename))
		c.Assert(err, check.IsNil)
		c.Assert(string(bytes), check.Equals, contents)

		found, err := alog.SearchEvents(now.Add(-time.Hour), now.Add(time.Hour), "", 0)
		c.Assert(err, check.IsNil)
		c.Assert(found, check.HasLen, 1)
	}
}

// TestForwardAndUpload tests forwarding server and upload
// server case
func (a *AuditTestSuite) TestForwardAndUpload(c *check.C) {
	fakeClock := clockwork.NewFakeClock()
	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		Clock:          fakeClock,
		ServerID:       "remote",
		UploadHandler:  NewMemoryUploader(),
	})
	c.Assert(err, check.IsNil)
	defer alog.Close()

	a.forwardAndUpload(c, fakeClock, alog)
}

// TestLegacyHandler tests playback for legacy sessions
// that are stored on disk in unpacked format
func (a *AuditTestSuite) TestLegacyHandler(c *check.C) {
	memory := NewMemoryUploader()
	wrapper, err := NewLegacyHandler(LegacyHandlerConfig{
		Handler: memory,
		Dir:     a.dataDir,
	})
	c.Assert(err, check.IsNil)

	fakeClock := clockwork.NewFakeClock()
	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		Clock:          fakeClock,
		ServerID:       "remote",
		UploadHandler:  wrapper,
	})
	c.Assert(err, check.IsNil)
	defer alog.Close()

	sid, compare := a.forwardAndUpload(c, fakeClock, alog)

	// Download the session in the old format
	ctx := context.TODO()

	tarball, err := ioutil.TempFile("", "teleport-legacy")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tarball.Name())

	err = memory.Download(ctx, sid, tarball)
	c.Assert(err, check.IsNil)

	authServers, err := getAuthServers(a.dataDir)
	c.Assert(err, check.IsNil)
	c.Assert(authServers, check.HasLen, 1)

	targetDir := filepath.Join(a.dataDir, authServers[0], SessionLogsDir, defaults.Namespace)

	_, err = tarball.Seek(0, 0)
	c.Assert(err, check.IsNil)

	err = utils.Extract(tarball, targetDir)
	c.Assert(err, check.IsNil)

	unpacked, err := wrapper.IsUnpacked(ctx, sid)
	c.Assert(err, check.IsNil)
	c.Assert(unpacked, check.Equals, true)

	// remove recording from the uploader
	// and make sure that playback for the session still
	// works
	memory.Reset()

	err = compare()
	c.Assert(err, check.IsNil)
}

// TestExternalLog tests forwarding server and upload
// server case
func (a *AuditTestSuite) TestExternalLog(c *check.C) {
	fileLog, err := NewFileLog(FileLogConfig{
		Dir: c.MkDir(),
	})
	c.Assert(err, check.IsNil)

	fakeClock := clockwork.NewFakeClock()
	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		Clock:          fakeClock,
		ServerID:       "remote",
		UploadHandler:  NewMemoryUploader(),
		ExternalLog:    fileLog,
	})
	c.Assert(err, check.IsNil)
	defer alog.Close()

	a.forwardAndUpload(c, fakeClock, alog)
}

// forwardAndUpload tests forwarding server and upload
// server case
func (a *AuditTestSuite) forwardAndUpload(c *check.C, fakeClock clockwork.Clock, alog IAuditLog) (session.ID, func() error) {
	uploadDir := c.MkDir()
	err := os.MkdirAll(filepath.Join(uploadDir, "upload", "sessions", defaults.Namespace), 0755)
	c.Assert(err, check.IsNil)

	sessionID := session.NewID()
	forwarder, err := NewForwarder(ForwarderConfig{
		Namespace:      defaults.Namespace,
		SessionID:      sessionID,
		ServerID:       "upload",
		DataDir:        uploadDir,
		RecordSessions: true,
		ForwardTo:      alog,
	})
	c.Assert(err, check.IsNil)

	// start the session and emit data stream to it and wrap it up
	firstMessage := []byte("hello")
	err = forwarder.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: string(sessionID),
		Chunks: []*SessionChunk{
			// start the seession
			{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
			// type "hello" into session "100"
			{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 1,
				ChunkIndex: 0,
				Offset:     0,
				EventType:  SessionPrintEvent,
				Data:       firstMessage,
			},
			// emitting session end event should close the session
			{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 4,
				EventType:  SessionEndEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(forwarder.Close(), check.IsNil)

	upload(c, uploadDir, fakeClock, alog)

	compare := func() error {
		history, err := alog.GetSessionEvents(defaults.Namespace, sessionID, 0, true)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(history) != 3 {
			return trace.BadParameter("expected history of 3, got %v", len(history))
		}

		// make sure offsets were properly set (0 for the first event and 5 bytes for hello):
		if history[1][SessionByteOffset].(float64) != float64(0) {
			return trace.BadParameter("expected offset of 0, got %v", history[1][SessionByteOffset])
		}
		if history[1][SessionEventTimestamp].(float64) != float64(0) {
			return trace.BadParameter("expected timestamp of 0, got %v", history[1][SessionEventTimestamp])
		}

		// fetch all bytes
		buff, err := alog.GetSessionChunk(defaults.Namespace, sessionID, 0, 5000)
		if err != nil {
			return trace.Wrap(err)
		}
		if string(buff) != string(firstMessage) {
			return trace.CompareFailed("%q != %q", string(buff), string(firstMessage))
		}

		// with offset
		buff, err = alog.GetSessionChunk(defaults.Namespace, sessionID, 2, 5000)
		if err != nil {
			return trace.Wrap(err)
		}
		if string(buff) != string(firstMessage[2:]) {
			return trace.CompareFailed("%q != %q", string(buff), string(firstMessage[2:]))
		}
		return nil
	}

	// trigger several parallel downloads, they should not fail
	iterations := 50
	resultsC := make(chan error, iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			resultsC <- compare()
		}()
	}

	timeout := time.After(time.Second)
	for i := 0; i < iterations; i++ {
		select {
		case err := <-resultsC:
			c.Assert(err, check.IsNil)
		case <-timeout:
			c.Fatalf("timeout waiting for goroutines to finish")
		}
	}

	return sessionID, compare
}

func marshal(f EventFields) []byte {
	data, err := json.Marshal(f)
	if err != nil {
		panic(err)
	}
	return data
}
