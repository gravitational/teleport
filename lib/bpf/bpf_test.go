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
	"net/http"
	"net/http/httptest"
	"os"
	os_exec "os/exec"
	"testing"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

type Suite struct{}

var _ = fmt.Printf
var _ = check.Suite(&Suite{})

func TestBPF(t *testing.T) { check.TestingT(t) }

func TestMain(m *testing.M) {
	// If the test is asking for itself to be re-executed, do that instead
	// of running tests.
	if len(os.Args) == 3 && os.Args[1] == "teleport-test-helper" {
		err := runCommand(os.Args[2])
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	code := m.Run()
	os.Exit(code)
}

func (s *Suite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}
func (s *Suite) TearDownSuite(c *check.C) {}
func (s *Suite) SetUpTest(c *check.C)     {}
func (s *Suite) TearDownTest(c *check.C)  {}

/*
func (s *Suite) TestWatch(c *check.C) {
	// This test must be run as root. Only root can create cgroups.
	if !isRoot() {
		c.Skip("Tests for package cgroup can only be run as root.")
	}

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	dir, err := ioutil.TempDir("", "cgroup-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir)

	service, err := New(&Config{
		Enabled:    true,
		CgroupPath: dir,
	})

	sessionID := uuid.New()
	serverID := uuid.New()

	// Re-exec test with a pipe that can be used to signal to continue.

	cgroupID, err := service.OpenSession(&SessionContext{
		Namespace: "default",
		SessionID: sessionID,
		ServerID:  serverID,
		Login:     "foo",
		User:      "foo@example.com",
		PID:       0,
		AuditLog:  &fakeLog{},
		Events: map[string]bool{
			"command",
			"disk",
			"network",
		},
	})
	c.Assert(err, check.IsNil)
	c.Assert(cgroupID > 0, check.Equals, true)

	// Pull events off the fakeLog here and make sure the event that was asked
	// to be executed was executed.
}
*/

func (s *Suite) TestEvents(c *check.C) {
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

func runCommand(file string) {
	err := waitForContinue()
	if err != nil {
		return trace.Wrap(err)
	}

	// Lookup and run the requested command.
	path, err := os_exec.LookPath(file)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os_exec.Command(path).Run()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func waitForContinue() error {
	contfd := os.NewFile(3, "/proc/self/fd/4")
	if cmdfd == nil {
		return trace.BadParameter("no continue file descriptor found")
	}

	// Reading from the continue file descriptor will block until it's closed. It
	// won't be closed until the parent has placed it in a cgroup.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		var r bytes.Buffer
		r.ReadFrom(contfd)
		cancel()
	}()

	select {
	case <-ctx.Done():
		return nil
	case <-time.After(5 * time.Second):
		return trace.BadParameter("timed out waiting for continue signal")
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
