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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
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
func (a *AuditTestSuite) makeLog(c *check.C, dataDir string) (*AuditLog, error) {
	return a.makeLogWithClock(c, dataDir, nil)
}

// creates a file-based audit log and returns a proper *AuditLog pointer
// instead of the usual IAuditLog interface
func (a *AuditTestSuite) makeLogWithClock(c *check.C, dataDir string, clock clockwork.Clock) (*AuditLog, error) {
	handler, err := NewLegacyHandler(LegacyHandlerConfig{
		Handler: NewMemoryUploader(),
		Dir:     dataDir,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:       dataDir,
		ServerID:      "server1",
		Clock:         clock,
		UIDGenerator:  utils.NewFakeUID(),
		UploadHandler: handler,
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
	alog, err := a.makeLog(c, a.dataDir)
	c.Assert(err, check.IsNil)
	// close twice:
	c.Assert(alog.Close(), check.IsNil)
	c.Assert(alog.Close(), check.IsNil)
}

func (a *AuditTestSuite) TestBasicLogging(c *check.C) {
	// create audit log, write a couple of events into it, close it
	clock := clockwork.NewFakeClock()
	alog, err := a.makeLogWithClock(c, a.dataDir, clock)
	c.Assert(err, check.IsNil)

	// emit regular event:
	err = alog.EmitAuditEventLegacy(Event{Name: "user.joined"}, EventFields{"apples?": "yes"})
	c.Assert(err, check.IsNil)
	logfile := alog.localLog.file.Name()
	c.Assert(alog.Close(), check.IsNil)

	// read back what's been written:
	bytes, err := os.ReadFile(logfile)
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
	alog, err := a.makeLogWithClock(c, a.dataDir, clock)
	c.Assert(err, check.IsNil)
	defer func() {
		c.Assert(alog.Close(), check.IsNil)
	}()

	for _, duration := range []time.Duration{0, time.Hour * 25} {
		// advance time and emit audit event
		now := start.Add(duration)
		clock.Advance(duration)

		// emit regular event:
		event := &events.Resize{
			Metadata:     events.Metadata{Type: "resize", Time: now},
			TerminalSize: "10:10",
		}
		err = alog.EmitAuditEvent(context.TODO(), event)
		c.Assert(err, check.IsNil)
		logfile := alog.localLog.file.Name()

		// make sure that file has the same date as the event
		dt, err := parseFileTime(filepath.Base(logfile))
		c.Assert(err, check.IsNil)
		c.Assert(dt, check.Equals, time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))

		// read back what's been written:
		bytes, err := os.ReadFile(logfile)
		c.Assert(err, check.IsNil)
		contents, err := json.Marshal(event)
		contents = append(contents, '\n')
		c.Assert(err, check.IsNil)
		c.Assert(string(bytes), check.Equals, string(contents))

		// read back the contents using symlink
		bytes, err = os.ReadFile(filepath.Join(alog.localLog.SymlinkDir, SymlinkFilename))
		c.Assert(err, check.IsNil)
		c.Assert(string(bytes), check.Equals, string(contents))

		found, _, err := alog.SearchEvents(now.Add(-time.Hour), now.Add(time.Hour), apidefaults.Namespace, nil, 0, types.EventOrderAscending, "")
		c.Assert(err, check.IsNil)
		c.Assert(found, check.HasLen, 1)
	}
}

func (a *AuditTestSuite) TestExternalLog(c *check.C) {
	m := &mockAuditLog{
		emitter: eventstest.MockEmitter{},
	}

	fakeClock := clockwork.NewFakeClock()
	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:       a.dataDir,
		Clock:         fakeClock,
		ServerID:      "remote",
		UploadHandler: NewMemoryUploader(),
		ExternalLog:   m,
	})
	c.Assert(err, check.IsNil)
	defer alog.Close()

	evt := &events.SessionConnect{}
	c.Assert(alog.EmitAuditEvent(context.Background(), evt), check.IsNil)

	c.Assert(m.emitter.Events(), check.HasLen, 1)
	c.Assert(m.emitter.Events()[0], check.Equals, evt)
}
