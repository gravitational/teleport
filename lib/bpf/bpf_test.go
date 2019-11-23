// +build bpf,linux

/*
Copyright 2019 Gravitational, Inc.

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

package bpf

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	os_exec "os/exec"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/pborman/uuid"
	"gopkg.in/check.v1"
)

type Suite struct{}

var _ = fmt.Printf
var _ = check.Suite(&Suite{})

func TestBPF(t *testing.T) { check.TestingT(t) }

func (s *Suite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}
func (s *Suite) TearDownSuite(c *check.C) {}
func (s *Suite) SetUpTest(c *check.C)     {}
func (s *Suite) TearDownTest(c *check.C)  {}

func (s *Suite) TestWatch(c *check.C) {
	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	if !isRoot() {
		c.Skip("Tests for package bpf can only be run as root.")
	}
	err := isHostCompatible()
	if err != nil {
		c.Skip(fmt.Sprintf("Tests for package bpf can not be run: %v.", err))
	}

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	dir, err := ioutil.TempDir("", "cgroup-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir)

	// Create BPF service.
	service, err := New(&Config{
		Enabled:    true,
		CgroupPath: dir,
	})

	// Create a fake audit log that can be used to capture the events emitted.
	auditLog := newFakeLog()

	// Create and start a program that does nothing. Since sleep will run longer
	// than we wait below, nothing should be emit to the Audit Log.
	cmd := os_exec.Command("sleep", "10")
	err = cmd.Start()
	c.Assert(err, check.IsNil)

	// Create a monitoring session for init. The events we execute should not
	// have PID 1, so nothing should be captured in the Audit Log.
	cgroupID, err := service.OpenSession(&SessionContext{
		Namespace: defaults.Namespace,
		SessionID: uuid.New(),
		ServerID:  uuid.New(),
		Login:     "foo",
		User:      "foo@example.com",
		PID:       cmd.Process.Pid,
		AuditLog:  auditLog,
		Events: map[string]bool{
			teleport.EnhancedRecordingCommand: true,
			teleport.EnhancedRecordingDisk:    true,
			teleport.EnhancedRecordingNetwork: true,
		},
	})
	c.Assert(err, check.IsNil)
	c.Assert(cgroupID > 0, check.Equals, true)

	// Execute "ls" in a loop.
	go func() {
		for {
			// Find "ls" binary.
			lsPath, err := os_exec.LookPath("ls")
			c.Assert(err, check.IsNil)

			// Run "ls".
			err = os_exec.Command(lsPath).Run()
			c.Assert(err, check.IsNil)

			// Delay.
			time.Sleep(250 * time.Millisecond)
		}
	}()

	// Keep checking that even though events are being executed, that they are
	// not emitted to the audit log because the cgroup they are in is not being
	// monitored.
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-time.Tick(250 * time.Millisecond):
			c.Assert(auditLog.events, check.HasLen, 0)
		case <-timer.C:
			return
		}
	}
}

// TestObfuscate checks if execsnoop can capture Obfuscated commands.
func (s *Suite) TestObfuscate(c *check.C) {
	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	if !isRoot() {
		c.Skip("Tests for package bpf can only be run as root.")
	}
	err := isHostCompatible()
	if err != nil {
		c.Skip(fmt.Sprintf("Tests for package bpf can not be run: %v.", err))
	}

	// Find the programs needed to run these tests on the host.
	decoderPath, err := os_exec.LookPath("base64")
	c.Assert(err, check.IsNil)
	shellPath, err := os_exec.LookPath("sh")
	c.Assert(err, check.IsNil)

	// Create a context that will be used to close and stop the BPF programs
	// at the end of the test.
	closeContext, closeFunc := context.WithCancel(context.Background())
	defer closeFunc()

	// Start execsnoop.
	execsnoop, err := startExec(closeContext, defaults.PerfBufferPageCount)
	defer execsnoop.close()
	c.Assert(err, check.IsNil)

	// Create a context that will be used to signal that an event has been recieved.
	doneContext, doneFunc := context.WithCancel(context.Background())

	// Start two goroutines. The first writes a script which will execute "ls"
	// in a loop. The second waits for an exec event to show up the reports "ls"
	// has been executed.
	go func() {
		// Create temporary file.
		file, err := ioutil.TempFile("", "test-script")
		c.Assert(err, check.IsNil)
		defer os.Remove(file.Name())

		// Write script to file.
		shellContents := fmt.Sprintf("#!%v\necho bHM= | %v --decode | %v",
			shellPath, decoderPath, shellPath)
		_, err = file.Write([]byte(shellContents))
		c.Assert(err, check.IsNil)
		err = file.Close()
		c.Assert(err, check.IsNil)

		// Make script executable.
		err = os.Chmod(file.Name(), 0700)
		c.Assert(err, check.IsNil)

		for {
			// Run script.
			err = os_exec.Command(file.Name()).Run()
			c.Assert(err, check.IsNil)

			// Delay.
			time.Sleep(250 * time.Millisecond)
		}
	}()
	go func() {
		for {
			select {
			case eventBytes := <-execsnoop.events():
				// Unmarshal the event.
				var event rawExecEvent
				err := unmarshalEvent(eventBytes, &event)
				c.Assert(err, check.IsNil)

				// Check the event is what we expect, in this case "ls".
				if convertString(unsafe.Pointer(&event.Command)) == "ls" {
					doneFunc()
				}
			}
		}

	}()

	// Wait for an event to arrive from execsnoop. If an event does not arrive
	// within 10 seconds, timeout.
	select {
	case <-doneContext.Done():
	case <-time.After(10 * time.Second):
		c.Fatalf("Timed out waiting for an event.")
	}

}

// TestScript checks if execsnoop can capture what a script executes.
func (s *Suite) TestScript(c *check.C) {
	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	if !isRoot() {
		c.Skip("Tests for package bpf can only be run as root.")
	}
	err := isHostCompatible()
	if err != nil {
		c.Skip(fmt.Sprintf("Tests for package bpf can not be run: %v.", err))
	}

	// Create a context that will be used to close and stop the BPF programs
	// at the end of the test.
	closeContext, closeFunc := context.WithCancel(context.Background())
	defer closeFunc()

	// Start execsnoop.
	execsnoop, err := startExec(closeContext, defaults.PerfBufferPageCount)
	defer execsnoop.close()
	c.Assert(err, check.IsNil)

	// Create a context that will be used to signal that an event has been recieved.
	doneContext, doneFunc := context.WithCancel(context.Background())

	// Start two goroutines. The first writes a script which will execute "ls"
	// in a loop. The second waits for an exec event to show up the reports "ls"
	// has been executed.
	go func() {
		// Create temporary file.
		file, err := ioutil.TempFile("", "test-script")
		c.Assert(err, check.IsNil)
		defer os.Remove(file.Name())

		// Write script to file.
		_, err = file.Write([]byte("#!/bin/sh\nls"))
		c.Assert(err, check.IsNil)
		err = file.Close()
		c.Assert(err, check.IsNil)

		// Make script executable.
		err = os.Chmod(file.Name(), 0700)
		c.Assert(err, check.IsNil)

		for {
			// Run script.
			err = os_exec.Command(file.Name()).Run()
			c.Assert(err, check.IsNil)
			// Delay.
			time.Sleep(250 * time.Millisecond)
		}
	}()
	go func() {
		for {
			select {
			case eventBytes := <-execsnoop.events():
				// Unmarshal the event.
				var event rawExecEvent
				err := unmarshalEvent(eventBytes, &event)
				c.Assert(err, check.IsNil)

				// Check the event is what we expect, in this case "ls".
				if convertString(unsafe.Pointer(&event.Command)) == "ls" {
					doneFunc()
				}
			}
		}

	}()

	// Wait for an event to arrive from execsnoop. If an event does not arrive
	// within 10 seconds, timeout.
	select {
	case <-doneContext.Done():
	case <-time.After(10 * time.Second):
		c.Fatalf("Timed out waiting for an event.")
	}
}

// TestPrograms tests execsnoop, opensnoop, and tcpconnect to make sure they
// run and receive events.
func (s *Suite) TestPrograms(c *check.C) {
	// This test must be run as root. Only root can create cgroups.
	if !isRoot() {
		c.Skip("Tests for package bpf can only be run as root.")
	}

	// Check that the host is capable of running BPF programs.
	err := isHostCompatible()
	if err != nil {
		c.Skip(fmt.Sprintf("Tests for package bpf can not be run: %v.", err))
	}

	// Start a debug server that tcpconnect will connect to.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	}))
	defer ts.Close()

	// Create a context that will be used to close and stop the BPF programs
	// at the end of the test.
	closeContext, closeFunc := context.WithCancel(context.Background())
	defer closeFunc()

	// Start execsnoop.
	execsnoop, err := startExec(closeContext, defaults.PerfBufferPageCount)
	defer execsnoop.close()
	c.Assert(err, check.IsNil)

	// Start opensnoop.
	opensnoop, err := startOpen(closeContext, defaults.PerfBufferPageCount)
	defer opensnoop.close()
	c.Assert(err, check.IsNil)

	// Start tcpconnect.
	tcpconnect, err := startConn(closeContext, defaults.PerfBufferPageCount)
	defer tcpconnect.close()
	c.Assert(err, check.IsNil)

	// Loop over all three programs and make sure events are received off the
	// perf buffer.
	var tests = []struct {
		inName        string
		inCommand     string
		inCommandArgs []string
		inEventCh     <-chan []byte
		inHTTP        bool
	}{
		// Run execsnoop with "ls".
		{
			inName:        "execsnoop",
			inCommand:     "ls",
			inCommandArgs: []string{},
			inEventCh:     execsnoop.events(),
			inHTTP:        false,
		},
		// Run opensnoop with "ls". This is fine because "ls" will open some
		// shared library.
		{
			inName:        "opensnoop",
			inCommand:     "ls",
			inCommandArgs: []string{},
			inEventCh:     opensnoop.events(),
			inHTTP:        false,
		},
		// Run tcpconnect with netcat.
		{
			inName:    "tcpconnect",
			inEventCh: tcpconnect.v4Events(),
			inHTTP:    true,
		},
	}
	for _, tt := range tests {
		// Create a context that will be used to signal that an event has been recieved.
		doneContext, doneFunc := context.WithCancel(context.Background())

		// Start two goroutines. The first will wait for the BPF program event to
		// arrive, and once it has, signal over the context that it's complete. The
		// second will continue to execute or a HTTP GET in a in a loop attempting to
		// trigger an event.
		go waitForEvent(doneContext, doneFunc, tt.inEventCh)
		if tt.inHTTP {
			go executeHTTP(c, doneContext, ts.URL)
		} else {
			go executeCommand(c, doneContext, tt.inCommand)
		}

		// Wait for an event to arrive from execsnoop. If an event does not arrive
		// within 10 seconds, timeout.
		select {
		case <-doneContext.Done():
		case <-time.After(10 * time.Second):
			c.Fatalf("Timed out waiting for an %v event.", tt.inName)
		}
	}
}

// waitForEvent will wait for an event to arrive over the perf buffer and
// signal when it has.
func waitForEvent(ctx context.Context, cancel context.CancelFunc, eventCh <-chan []byte) {
	for {
		select {
		case <-eventCh:
			cancel()
		case <-ctx.Done():
			return
		}
	}
}

// executeCommand will execute some command in a loop.
func executeCommand(c *check.C, doneContext context.Context, file string) {
	for {
		// Lookup and run the requested command.
		path, err := os_exec.LookPath(file)
		if err != nil {
			c.Fatalf("Failed to find execute %q: %v.", file, err)
		}
		err = os_exec.Command(path).Run()
		if err != nil {
			c.Fatalf("Failed to run command %q: %v.", file, err)
		}

		time.Sleep(250 * time.Millisecond)
	}
}

// executeHTTP will perform a HTTP GET to some endpoint in a loop.
func executeHTTP(c *check.C, doneContext context.Context, endpoint string) {
	for {
		// Perform HTTP GET to the requested endpoint.
		_, err := http.Get(endpoint)
		c.Assert(err, check.IsNil)

		time.Sleep(250 * time.Millisecond)
	}
}

// isRoot returns a boolean if the test is being run as root or not. Tests
// for this package must be run as root.
func isRoot() bool {
	if os.Geteuid() != 0 {
		return false
	}
	return true
}

// fakeLog is used in tests to obtain events emitted to the Audit Log.
type fakeLog struct {
	mu     sync.Mutex
	events []events.EventFields
}

func newFakeLog() *fakeLog {
	return &fakeLog{
		events: make([]events.EventFields, 0),
	}
}

func (a *fakeLog) EmitAuditEvent(e events.Event, f events.EventFields) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.events = append(a.events, f)
	return nil
}

func (a *fakeLog) PostSessionSlice(s events.SessionSlice) error {
	return trace.NotImplemented("not implemented")
}

func (a *fakeLog) UploadSessionRecording(r events.SessionRecording) error {
	return trace.NotImplemented("not implemented")
}

func (a *fakeLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes int, maxBytes int) ([]byte, error) {
	return nil, trace.NotFound("")
}

func (a *fakeLog) GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]events.EventFields, error) {
	return nil, trace.NotFound("")
}

func (a *fakeLog) SearchEvents(fromUTC, toUTC time.Time, query string, limit int) ([]events.EventFields, error) {
	return nil, trace.NotFound("")
}

func (a *fakeLog) SearchSessionEvents(fromUTC time.Time, toUTC time.Time, limit int) ([]events.EventFields, error) {
	return nil, trace.NotFound("")
}

func (a *fakeLog) WaitForDelivery(context.Context) error {
	return trace.NotImplemented("not implemented")
}

func (a *fakeLog) Close() error {
	return trace.NotFound("")
}
