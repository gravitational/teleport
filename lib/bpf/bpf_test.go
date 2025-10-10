//go:build bpf && !386

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package bpf

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cgroup"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

const (
	// reexecInCGroupCmd is a cmd used to re-exec the test binary and call arbitrary program.
	reexecInCGroupCmd = "reexecCgroup"
	// networkIPv4InCgroupCmd is a cmd used to re-exec the test binary and make HTTP call using an IPv4 address.
	networkIPv4InCgroupCmd = "networkCgroupIPv4"
	// networkIPv6InCgroupCmd is a cmd used to re-exec the test binary and make HTTP call using an IPv6 address.
	networkIPv6InCgroupCmd = "networkCgroupIPv6"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)

	// Check if the re-exec was requested.
	if len(os.Args) == 3 {
		var err error

		switch os.Args[1] {
		case reexecInCGroupCmd:
			// Get the command to run passed as the 3rd argument.
			cmd := os.Args[2]

			err = waitAndRun(cmd)
		case networkIPv4InCgroupCmd:
			endpoint := os.Args[2]
			err = getEndpoint(endpoint, false)
		case networkIPv6InCgroupCmd:
			endpoint := os.Args[2]
			err = getEndpoint(endpoint, true)
		default:
			os.Exit(2)
		}

		if err != nil {
			fmt.Printf("rexec failed: %v\n", err)
			// Something went wrong, exit with error.
			os.Exit(1)
		}

		// The rexec was handled and nothing bad happened.
		os.Exit(0)
	}

	os.Exit(m.Run())
}

// waitAndRun wait for continue signal to be generated an executes the
// passed command and waits until returns.
func waitAndRun(cmd string) error {
	if err := waitForContinue(); err != nil {
		return err
	}

	return osexec.Command(cmd).Run()
}

// getEndpoint wait for continue signal to be generated then creates an
// HTTP GET request on provided endpoint.
func getEndpoint(endpoint string, ipv6 bool) error {
	if err := waitForContinue(); err != nil {
		return err
	}
	forceIPVersion := "-4"
	if ipv6 {
		forceIPVersion = "-6"
	}

	return osexec.Command("curl", forceIPVersion, endpoint).Run()
}

// waitForContinue opens FD 3 and waits the signal from parent process that
// the cgroup is being observed and the even can be generated.
func waitForContinue() error {
	waitFD := os.NewFile(3, "/proc/self/fd/3")
	defer waitFD.Close()

	buff := make([]byte, 1)
	if _, err := waitFD.Read(buff); err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	return nil
}

func TestRootWatch(t *testing.T) {
	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	checkBPF(t)

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	cgroupPath := t.TempDir()

	// Create BPF service.
	service, err := New(&servicecfg.BPFConfig{
		Enabled:    true,
		CgroupPath: cgroupPath,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		const restarting = false
		require.NoError(t, service.Close(restarting))
	})

	// Create a fake audit log that can be used to capture the events emitted.
	emitter := &eventstest.MockRecorderEmitter{}

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
	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	checkBPF(t)

	// Find the programs needed to run these tests on the host.
	decoderPath, err := osexec.LookPath("base64")
	require.NoError(t, err)
	shellPath, err := osexec.LookPath("sh")
	require.NoError(t, err)

	// Start execsnoop.
	execsnoop, err := startExec()
	t.Cleanup(execsnoop.close)
	require.NoError(t, err)

	// Create obfuscated script.
	shellContents := fmt.Sprintf("#!%v\necho bHM= | %v --decode | %v",
		shellPath, decoderPath, shellPath)

	// Write script to a temporary folder.
	fileName := filepath.Join(t.TempDir(), "test-script")
	err = os.WriteFile(fileName, []byte(shellContents), 0o700)
	require.NoError(t, err)

	done := make(chan struct{})
	t.Cleanup(func() {
		done <- struct{}{}
		<-done
	})

	// Start a goroutine that writes a script which will execute "ls"
	// in a loop. Then waits for an exec event to show up the reports "ls"
	// has been executed.
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		defer close(done)

		for {
			select {
			case <-ticker.C:
				err := runCmd(t, reexecInCGroupCmd, fileName, execsnoop)
				assert.NoError(t, err)
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
			var event commandDataT
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
	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	checkBPF(t)

	// Write script to a temporary folder.
	fileName := filepath.Join(t.TempDir(), "test-script")
	err := os.WriteFile(fileName, []byte("#!/bin/sh\nls"), 0o700)
	require.NoError(t, err)

	// Start execsnoop.
	execsnoop, err := startExec()
	t.Cleanup(execsnoop.close)
	require.NoError(t, err)

	done := make(chan struct{})
	t.Cleanup(func() {
		done <- struct{}{}
		<-done
	})

	// Start a goroutine that writes a script which will execute "ls"
	// in a loop. Then waits for an exec event to show up the reports "ls"
	// has been executed.
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		defer close(done)
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				// Run script in a cgroup.
				err := runCmd(t, reexecInCGroupCmd, fileName, execsnoop)
				assert.NoError(t, err)
			}
		}
	}()

	// Wait for an event to arrive from execsnoop. If an event does not arrive
	// within 10 seconds, timeout.
	for {
		select {
		case eventBytes := <-execsnoop.events():
			var event commandDataT
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
	// This test must be run as root. Only root can create cgroups.
	checkBPF(t)

	// Start execsnoop.
	execsnoop, err := startExec()
	require.NoError(t, err)
	defer execsnoop.close()

	// Start opensnoop.
	opensnoop, err := startOpen()
	require.NoError(t, err)
	defer opensnoop.close()

	// Start tcpconnect.
	tcpconnect, err := startConn()
	require.NoError(t, err)
	defer tcpconnect.close()

	// Loop over all three programs and make sure events are received off the
	// perf buffer.
	tests := []struct {
		inName    string
		inEventCh <-chan []byte
		genEvents func(t *testing.T)
		verifyFn  func(event []byte) bool
	}{
		// Run execsnoop with "ls".
		{
			inName:    "execsnoop",
			inEventCh: execsnoop.events(),
			genEvents: func(t *testing.T) {
				executeCommand(t, "ls", execsnoop)
			},
			verifyFn: func(event []byte) bool {
				var e commandDataT
				err := unmarshalEvent(event, &e)
				return err == nil && ConvertString(unsafe.Pointer(&e.Command)) == "ls"
			},
		},
		// Run opensnoop with "ls". This is fine because "ls" will open some
		// shared library.
		{
			inName:    "opensnoop",
			inEventCh: opensnoop.events(),
			genEvents: func(t *testing.T) {
				executeCommand(t, "ls", opensnoop)
			},
			verifyFn: func(event []byte) bool {
				var e diskDataT
				err := unmarshalEvent(event, &e)
				return err == nil && ConvertString(unsafe.Pointer(&e.Command)) == "ls"
			},
		},
		// Run tcpconnect with curl forcing IPv4.
		{
			inName:    "tcpconnect ipv4",
			inEventCh: tcpconnect.v4Events(),
			genEvents: func(t *testing.T) {
				executeHTTP(t, "http://google.com", false, tcpconnect)
			},
			verifyFn: func(event []byte) bool {
				var e networkIpv4DataT
				err := unmarshalEvent(event, &e)

				return err == nil && ConvertString(unsafe.Pointer(&e.Command)) == "curl"
			},
		},
		// Run tcpconnect with curl forcing IPv6.
		{
			inName:    "tcpconnect ipv6",
			inEventCh: tcpconnect.v6Events(),
			genEvents: func(t *testing.T) {
				executeHTTP(t, "http://google.com", true, tcpconnect)
			},
			verifyFn: func(event []byte) bool {
				var e networkIpv6DataT
				err := unmarshalEvent(event, &e)

				return err == nil && ConvertString(unsafe.Pointer(&e.Command)) == "curl"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.inName, func(t *testing.T) {
			// Create a context that will be used to signal that an event has been received.
			doneContext, doneFunc := context.WithCancel(context.Background())
			t.Cleanup(doneFunc)

			// Start two goroutines. The first will wait for the BPF program event to
			// arrive, and once it has, signal over the context that it's complete. The
			// second will continue to execute or an HTTP GET in a processAccessEvents attempting to
			// trigger an event.
			go waitForEvent(doneContext, doneFunc, tt.inEventCh, tt.verifyFn)

			go tt.genEvents(t)

			// Wait for an event to arrive from execsnoop. If an event does not arrive
			// within 10 seconds, timeout.
			select {
			case <-doneContext.Done():
			case <-time.After(10 * time.Second):
				t.Fatalf("Timed out waiting for an %v event.", tt.inName)
			}
		})
	}
}

// TestRootLargeCommands given commands with higher amount of characters
// (length), ensure the command events are generated correctly.
func TestRootLargeCommands(t *testing.T) {
	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	checkBPF(t)

	for name, test := range map[string]struct {
		cmd               string
		expectPartialPath bool
	}{
		"large command": {
			cmd: "/random" + strings.Repeat("random", 128/len("random")),
		},
		"command exceed max size": {
			cmd:               "/random" + strings.Repeat("random", ArgvMax/len("random")),
			expectPartialPath: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			// Start execsnoop.
			execsnoop, err := startExec()
			defer execsnoop.close()
			require.NoError(t, err)

			// Since we're using a random command, we should expect its
			// execution will fail.
			err = runCmd(t, reexecInCGroupCmd, test.cmd, execsnoop)
			require.Error(t, err)

			for {
				select {
				case eventBytes := <-execsnoop.events():
					var event commandDataT
					err := unmarshalEvent(eventBytes, &event)
					require.NoError(t, err)

					// Since we're executing the command using the test binary,
					// the arguments return on a single event, and the path of
					// or command will come on the argv part.
					argv := ConvertString(unsafe.Pointer(&event.Argv))
					if event.Type == eventArg {
						if test.expectPartialPath {
							require.Len(t, argv, ArgvMax)
							require.True(t, strings.HasPrefix(test.cmd, argv), "expected command to have same content until cap")
							return
						} else {
							require.Equal(t, test.cmd, argv)
							return
						}
					}
				case <-time.After(10 * time.Second):
					t.Fatalf("Timed out waiting for an event.")
					return
				}
			}
		})
	}
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
		return 0, trace.Wrap(err, "failed to mount cgroup")
	}
	t.Cleanup(func() {
		const skipUnmount = false
		require.NoError(t, cgroupSrv.Close(skipUnmount))
	})

	sessionID := uuid.New().String()
	// Put the cmd in a new cgroup.
	cgroupID, err := createCgroup(t, cgroupSrv, sessionID)
	if err != nil {
		return 0, trace.Wrap(err, "failed to create cgroup")
	}

	// Place requested PID into cgroup.
	err = cgroupSrv.Place(sessionID, pid)
	if err != nil {
		return 0, trace.Wrap(err, "failed to place pid %d into cgroup", pid)
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
func executeCommand(t *testing.T, file string, traceCgroup cgroupRegister) {
	t.Helper()

	fullPath, err := osexec.LookPath(file)
	require.NoError(t, err, "Failed to find executable %q", file)

	err = runCmd(t, reexecInCGroupCmd, fullPath, traceCgroup)
	require.NoError(t, err)
}

func runCmd(t *testing.T, reexecCmd string, arg string, traceCgroup cgroupRegister) error {
	t.Helper()

	// Create a pipe to communicate with the child process after re-exec.
	readP, writeP, err := os.Pipe()
	if err != nil {
		return trace.Wrap(err, "failed to create pipe")
	}

	t.Cleanup(func() {
		readP.Close()
		writeP.Close()
	})

	// Re-exec the test binary. We can then move the binary to a new cgroup.
	cmd := osexec.Command(os.Args[0], reexecCmd, arg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = append(cmd.ExtraFiles, readP)

	// Start the re-exec
	err = cmd.Start()
	if err != nil {
		return trace.Wrap(err, "failed to start command")
	}

	cgroupID, err := moveIntoCgroup(t, cmd.Process.Pid)
	if err != nil {
		return trace.Wrap(err, "failed to move pid %d into cgroup", cmd.Process.Pid)
	}

	// Register the process in the BPF module
	err = traceCgroup.startSession(cgroupID)
	if err != nil {
		return trace.Wrap(err, "failed to register cgroup in BPF module")
	}

	// Send one byte to continue the subprocess execution.
	_, err = writeP.Write([]byte{1})
	if err != nil {
		return trace.Wrap(err, "failed to write to pipe")
	}
	// Wait for the command to exit. Otherwise, we cannot clean up the cgroup.
	waitErr := trace.Wrap(cmd.Wait())

	// Remove the registered cgroup from the BPF module. Do not call it after
	// BPF module is deregistered.
	err = traceCgroup.endSession(cgroupID)
	if err != nil {
		return trace.NewAggregate(waitErr, trace.Wrap(err, "failed to deregister cgroup in BPF module"))
	}

	return waitErr
}

// executeHTTP will perform an HTTP GET to some endpoint in a subprocess
// that is placed into the traceCgroup cgroup so it can be tracked.
func executeHTTP(t *testing.T, endpoint string, ipv6 bool, traceCgroup cgroupRegister) {
	t.Helper()

	cmd := networkIPv4InCgroupCmd
	if ipv6 {
		cmd = networkIPv6InCgroupCmd
	}

	err := runCmd(t, cmd, endpoint, traceCgroup)
	require.NoError(t, err)
}

// checkBPF skips the test if BPF tests are not enabled or the test is not run
// as root.
func checkBPF(t *testing.T) {
	t.Helper()

	if !bpfTestEnabled() {
		t.Skip("BPF testing is disabled. Set TELEPORT_BPF_TEST environment variable to enable.")
	}
	if os.Geteuid() != 0 {
		t.Skip("Tests for package bpf can only be run as root.")
	}
}

// bpfTestEnabled returns true if BPF tests should run. Tests can be enabled by
// setting TELEPORT_BPF_TEST environment variable to any value.
func bpfTestEnabled() bool {
	return os.Getenv("TELEPORT_BPF_TEST") != ""
}
