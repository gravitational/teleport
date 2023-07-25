//go:build bpf && !386
// +build bpf,!386

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
	_ "embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	osexec "os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/aquasecurity/libbpfgo"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cgroup"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func TestRootWatch(t *testing.T) {
	// TODO(jakule): Find a way to run this test in CI. Disable for now to not block all BPF tests.
	t.Skip("this test always fails when running inside a CGroup/Docker")

	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	if !bpfTestEnabled() {
		t.Skip("BPF testing is disabled")
	}
	if !isRoot() {
		t.Skip("Tests for package bpf can only be run as root.")
	}

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	cgroupPath := t.TempDir()

	// Create BPF service.
	service, err := New(&servicecfg.BPFConfig{
		Enabled:    true,
		CgroupPath: cgroupPath,
	}, &servicecfg.RestrictedSessionConfig{})
	require.NoError(t, err)

	t.Cleanup(func() {
		const restarting = false
		require.NoError(t, service.Close(restarting))
	})

	// Create a fake audit log that can be used to capture the events emitted.
	emitter := &eventstest.MockEmitter{}

	// Create and start a program that does nothing. Since sleep will run longer
	// than we wait below, nothing should be emitted to the Audit Log.
	cmd := osexec.Command("sleep", "10")
	err = cmd.Start()
	require.NoError(t, err)

	// Create a monitoring session for init. The events we execute should not
	// have PID 1, so nothing should be captured in the Audit Log.
	cgroupID, err := service.OpenSession(&SessionContext{
		Namespace:      apidefaults.Namespace,
		SessionID:      uuid.New().String(),
		ServerID:       uuid.New().String(),
		ServerHostname: "ip-172-31-11-148",
		Login:          "foo",
		User:           "foo@example.com",
		PID:            cmd.Process.Pid,
		Emitter:        emitter,
		Events: map[string]bool{
			constants.EnhancedRecordingCommand: true,
			constants.EnhancedRecordingDisk:    true,
			constants.EnhancedRecordingNetwork: true,
		},
	})
	require.NoError(t, err)
	require.Greater(t, cgroupID, uint64(0))

	// Find "ls" binary.
	lsPath, err := osexec.LookPath("ls")
	require.NoError(t, err)

	// Execute "ls" a few times
	for i := 0; i < 5; i++ {
		// Run "ls".
		err = osexec.Command(lsPath).Run()
		require.NoError(t, err)

		// Delay.
		time.Sleep(25 * time.Millisecond)
	}

	// Make sure no events from "ls" were generated
	for _, e := range emitter.Events() {
		var pid uint64

		switch ev := e.(type) {
		case *apievents.SessionCommand:
			pid = ev.BPFMetadata.PID
		case *apievents.SessionDisk:
			pid = ev.BPFMetadata.PID
		case *apievents.SessionNetwork:
			pid = ev.BPFMetadata.PID
		}
		require.Equal(t, int(pid), cmd.Process.Pid)
	}
}

// TestRootObfuscate checks if execsnoop can capture Obfuscated commands.
func TestRootObfuscate(t *testing.T) {
	t.Skip("flaky test, disable now")
	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	if !bpfTestEnabled() {
		t.Skip("BPF testing is disabled")
	}
	if !isRoot() {
		t.Skip("Tests for package bpf can only be run as root.")
	}

	// Find the programs needed to run these tests on the host.
	decoderPath, err := osexec.LookPath("base64")
	require.NoError(t, err)
	shellPath, err := osexec.LookPath("sh")
	require.NoError(t, err)

	// Start execsnoop.
	execsnoop, err := startExec(8192)
	defer execsnoop.close()
	require.NoError(t, err)

	// Create obfuscated script.
	shellContents := fmt.Sprintf("#!%v\necho bHM= | %v --decode | %v",
		shellPath, decoderPath, shellPath)

	// Write script to a temporary folder.
	fileName := filepath.Join(t.TempDir(), "test-script")
	err = os.WriteFile(fileName, []byte(shellContents), 0700)
	require.NoError(t, err)

	done := make(chan struct{})
	defer close(done)

	// Start a goroutine that writes a script which will execute "ls"
	// in a loop. Then waits for an exec event to show up the reports "ls"
	// has been executed.
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				runCmd(t, reexecInCGroupCmd, fileName, execsnoop)
			case <-done:
				return
			}
		}
	}()

	// Wait for an event to arrive from execsnoop. If an event does not arrive
	// within 10 seconds, timeout.
	for {
		select {
		case eventBytes := <-execsnoop.events():
			var event rawExecEvent
			err := unmarshalEvent(eventBytes, &event)
			require.NoError(t, err)

			// Check the event is what we expect, in this case "ls".
			if ConvertString(unsafe.Pointer(&event.Command)) == "ls" {
				return
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("Timed out waiting for an event.")
		}
	}
}

// TestRootScript checks if execsnoop can capture what a script executes.
func TestRootScript(t *testing.T) {
	t.Skip("flaky test, disable now")
	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	if !bpfTestEnabled() {
		t.Skip("BPF testing is disabled")
	}
	if !isRoot() {
		t.Skip("Tests for package bpf can only be run as root.")
	}

	// Write script to a temporary folder.
	fileName := filepath.Join(t.TempDir(), "test-script")
	err := os.WriteFile(fileName, []byte("#!/bin/sh\nls"), 0700)
	require.NoError(t, err)

	// Start execsnoop.
	execsnoop, err := startExec(8192)
	defer execsnoop.close()
	require.NoError(t, err)

	done := make(chan struct{})
	defer close(done)

	// Start a goroutine that writes a script which will execute "ls"
	// in a loop. Then waits for an exec event to show up the reports "ls"
	// has been executed.
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				// Run script in a cgroup.
				runCmd(t, reexecInCGroupCmd, fileName, execsnoop)
			}
		}
	}()

	// Wait for an event to arrive from execsnoop. If an event does not arrive
	// within 10 seconds, timeout.
	for {
		select {
		case eventBytes := <-execsnoop.events():
			var event rawExecEvent
			err := unmarshalEvent(eventBytes, &event)
			require.NoError(t, err)

			// Check the event is what we expect, in this case "ls".
			if ConvertString(unsafe.Pointer(&event.Command)) == "ls" {
				return
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("Timed out waiting for an event.")
			return
		}
	}
}

// TestRootPrograms tests execsnoop, opensnoop, and tcpconnect to make sure they
// run and receive events.
func TestRootPrograms(t *testing.T) {
	t.Skip("flaky test, disable now")
	// This test must be run as root. Only root can create cgroups.
	if !bpfTestEnabled() {
		t.Skip("BPF testing is disabled")
	}
	if !isRoot() {
		t.Skip("Tests for package bpf can only be run as root.")
	}

	// Start a debug server that tcpconnect will connect to.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	}))
	defer ts.Close()

	// Start execsnoop.
	execsnoop, err := startExec(8192)
	require.NoError(t, err)
	defer execsnoop.close()

	// Start opensnoop.
	opensnoop, err := startOpen(8192)
	require.NoError(t, err)
	defer opensnoop.close()

	// Start tcpconnect.
	tcpconnect, err := startConn(8192)
	require.NoError(t, err)
	defer tcpconnect.close()

	// Loop over all three programs and make sure events are received off the
	// perf buffer.
	var tests = []struct {
		inName    string
		inEventCh <-chan []byte
		genEvents func(t *testing.T, ctx context.Context)
		verifyFn  func(event []byte) bool
	}{
		// Run execsnoop with "ls".
		{
			inName:    "execsnoop",
			inEventCh: execsnoop.events(),
			genEvents: func(t *testing.T, ctx context.Context) {
				executeCommand(t, ctx, "ls", execsnoop)
			},
			verifyFn: func(event []byte) bool {
				var e rawExecEvent
				err := unmarshalEvent(event, &e)
				return err == nil && ConvertString(unsafe.Pointer(&e.Command)) == "ls"
			},
		},
		// Run opensnoop with "ls". This is fine because "ls" will open some
		// shared library.
		{
			inName:    "opensnoop",
			inEventCh: opensnoop.events(),
			genEvents: func(t *testing.T, ctx context.Context) {
				executeCommand(t, ctx, "ls", opensnoop)
			},
			verifyFn: func(event []byte) bool {
				var e rawOpenEvent
				err := unmarshalEvent(event, &e)
				return err == nil
			},
		},
		// Run tcpconnect with netcat.
		{
			inName:    "tcpconnect",
			inEventCh: tcpconnect.v4Events(),
			genEvents: func(t *testing.T, ctx context.Context) {
				executeHTTP(t, ctx, ts.URL, tcpconnect)
			},
			verifyFn: func(event []byte) bool {
				var e rawConn4Event
				err := unmarshalEvent(event, &e)
				return err == nil
			},
		},
	}
	for _, tt := range tests {
		// Create a context that will be used to signal that an event has been recieved.
		doneContext, doneFunc := context.WithCancel(context.Background())

		// Start two goroutines. The first will wait for the BPF program event to
		// arrive, and once it has, signal over the context that it's complete. The
		// second will continue to execute or an HTTP GET in a processAccessEvents attempting to
		// trigger an event.
		go waitForEvent(doneContext, doneFunc, tt.inEventCh, tt.verifyFn)

		go tt.genEvents(t, doneContext)

		// Wait for an event to arrive from execsnoop. If an event does not arrive
		// within 10 seconds, timeout.
		select {
		case <-doneContext.Done():
		case <-time.After(10 * time.Second):
			t.Fatalf("Timed out waiting for an %v event.", tt.inName)
		}
	}
}

// TestRootBPFCounter tests that BPF-to-Prometheus counter works ok
func TestRootBPFCounter(t *testing.T) {
	t.Skip("flaky test, disable now")
	// This test must be run as root. Only root can create cgroups.
	if !bpfTestEnabled() {
		t.Skip("BPF testing is disabled")
	}
	if !isRoot() {
		t.Skip("Tests for package bpf can only be run as root.")
	}

	counterTestBPF, err := embedFS.ReadFile("bytecode/counter_test.bpf.o")
	if err != nil {
		t.Skip(fmt.Sprintf("Tests for package bpf can not be run: %v.", err))
	}

	module, err := libbpfgo.NewModuleFromBuffer(counterTestBPF, "counter_test")
	require.NoError(t, err)

	// Load into the kernel
	err = module.BPFLoadObject()
	require.NoError(t, err)

	err = AttachSyscallTracepoint(module, "close")
	require.NoError(t, err)

	promCounter := prometheus.NewCounter(prometheus.CounterOpts{Name: "test"})

	counter, err := NewCounter(module, "test_counter", promCounter)
	require.NoError(t, err)

	// Make sure the counter starts with 0
	require.Zero(t, testutil.ToFloat64(promCounter))

	// close(1234) will cause the counter to get incremented.
	magicFD := 1234

	// First do it a few times as to no overflow the doorbell buffer
	gentleBumps := 10
	for i := 0; i < gentleBumps; i++ {
		syscall.Close(magicFD)
	}

	// Not ideal but no other good way to know that the counter was updated
	time.Sleep(time.Second)

	// Make sure all are accounted for
	require.Equal(t, float64(gentleBumps), testutil.ToFloat64(promCounter))

	// Next, pound the counter to hopefully overflow the doorbell.
	poundingBumps := 100000
	for i := 0; i < poundingBumps; i++ {
		syscall.Close(magicFD)
	}

	// Not ideal but no other good way to know that the counter was updated
	time.Sleep(time.Second)

	// Make sure all are accounted for
	require.Equal(t, float64(gentleBumps+poundingBumps), testutil.ToFloat64(promCounter))

	counter.Close()
}

// waitForEvent will wait for an event to arrive over the perf buffer and
// signal when it has.
func waitForEvent(ctx context.Context, cancel context.CancelFunc, eventCh <-chan []byte, verifyFn func(event []byte) bool) {
	for {
		select {
		case e := <-eventCh:
			if verifyFn(e) {
				cancel()
			}
		case <-ctx.Done():
			return
		}
	}
}

// Moves the passed pid into a new cgroup.
func moveIntoCgroup(t *testing.T, pid int) (uint64, error) {
	t.Helper()

	cgroupPath := t.TempDir()

	cgroupSrv, err := cgroup.New(&cgroup.Config{
		MountPath: cgroupPath,
	})
	if err != nil {
		return 0, trace.Wrap(err)
	}
	t.Cleanup(func() {
		const skipUnmount = false
		require.NoError(t, cgroupSrv.Close(skipUnmount))
	})

	sessionID := uuid.New().String()
	// Put the cmd in a new cgroup.
	cgroupID, err := createCgroup(t, cgroupSrv, sessionID)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Place requested PID into cgroup.
	err = cgroupSrv.Place(sessionID, pid)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	t.Cleanup(func() {
		err := cgroupSrv.Remove(sessionID)
		require.NoError(t, err)
	})

	return cgroupID, nil
}

// createCgroup is a helper function to create Cgroup.
func createCgroup(t *testing.T, cgroup *cgroup.Service, sessionID string,
) (uint64, error) {
	t.Helper()

	err := cgroup.Create(sessionID)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	cgroupID, err := cgroup.ID(sessionID)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return cgroupID, nil
}

// executeCommand will execute some command in a loop.
func executeCommand(t *testing.T, doneContext context.Context, file string,
	traceCgroup cgroupRegister,
) {
	t.Helper()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Lookup and run the requested command.
			path, err := osexec.LookPath(file)
			if err != nil {
				t.Logf("Failed to find executable %q: %v.", file, err)
			}

			fullPath, err := osexec.LookPath(path)
			require.NoError(t, err)

			runCmd(t, reexecInCGroupCmd, fullPath, traceCgroup)
		case <-doneContext.Done():
			return
		}
	}
}

func runCmd(t *testing.T, reexecCmd string, arg string, traceCgroup cgroupRegister) {
	t.Helper()

	// Create a pipe to communicate with the child process after re-exec.
	readP, writeP, err := os.Pipe()
	require.NoError(t, err)

	t.Cleanup(func() {
		readP.Close()
		writeP.Close()
	})

	// Re-exec the test binary. We can then move the binary to a new cgroup.
	cmd := osexec.Command(os.Args[0], reexecCmd, arg)

	cmd.ExtraFiles = append(cmd.ExtraFiles, readP)

	// Start the re-exec
	err = cmd.Start()
	require.NoError(t, err)

	cgroupID, err := moveIntoCgroup(t, cmd.Process.Pid)
	require.NoError(t, err)

	// Register the process in the BPF module
	err = traceCgroup.startSession(cgroupID)
	require.NoError(t, err)

	// Send one byte to continue the subprocess execution.
	_, err = writeP.Write([]byte{1})
	require.NoError(t, err)

	// Wait for the command to exit. Otherwise, we cannot clean up the cgroup.
	require.NoError(t, cmd.Wait())

	// Remove the registered cgroup from the BPF module. Do not call it after
	// BPF module is deregistered.
	err = traceCgroup.endSession(cgroupID)
	require.NoError(t, err)
}

// executeHTTP will perform a HTTP GET to some endpoint in a loop.
func executeHTTP(t *testing.T, doneContext context.Context, endpoint string, traceCgroup cgroupRegister) {
	t.Helper()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Perform HTTP GET to the requested endpoint.
			if _, err := http.Get(endpoint); err != nil {
				t.Logf("HTTP request failed: %v.", err)
			}

			runCmd(t, networkInCgroupCmd, endpoint, traceCgroup)

		case <-doneContext.Done():
			return
		}
	}
}

// isRoot returns a boolean if the test is being run as root or not. Tests
// for this package must be run as root.
func isRoot() bool {
	return os.Geteuid() == 0
}

// bpfTestEnabled returns true if BPF tests should run. Tests can be enabled by
// setting TELEPORT_BPF_TEST environment variable to any value.
func bpfTestEnabled() bool {
	return os.Getenv("TELEPORT_BPF_TEST") != ""
}
