//go:build bpf && !386

/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package srv

import (
	"context"
	"debug/elf"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

const (
	// the maximum number of arguments that will be emitted in a event,
	// including argv[0]
	maxArgs = 20

	// the maximum length of a path, anything longer will be truncated
	maxPathLength = 255

	longArgBase = "averylongargument"

	// number of commands that will be run in parallel during the
	// stress test test case
	stressTestRunCount = 10
)

var (
	// the maximum length of a single argument, anything longer will be truncated
	maxArgLength = bpf.ArgsCacheSize

	longArg    = strings.Repeat(longArgBase, (maxArgLength/4)/len(longArgBase))
	overMaxArg = strings.Repeat(longArgBase, (maxArgLength)/len(longArgBase)+1)

	recordAllEvents = map[string]struct{}{
		constants.EnhancedRecordingCommand: {},
		constants.EnhancedRecordingDisk:    {},
		constants.EnhancedRecordingNetwork: {},
	}
)

type expectedEvents struct {
	// info about the command that will be executed
	cmdInfo commandInfo
	// paths that the command will open
	paths []string
	// destination address that the command will connect to
	dstAddr *addrInfo
	// number of times the command will be executed; if left at 0 it
	// will be assumed to be 1
	count int
}

type commandInfo struct {
	// program name of the command, typically the basename of hte full path
	program string
	// arguments to the command, excluding argv[0]
	args []string
	// interpreter for the command, if the command is a script
	interpreter string
	// path to the script that will be executed
	scriptPath string
	// should be set to true if the command is expected to return with
	// a non-zero exit code
	expectedFail bool
}

type addrInfo struct {
	// destination address the command will connect to
	addr string
	// destination port the command will connect to
	port int
}

func TestBPFRecording(t *testing.T) {
	skipIfNoBPF(t)

	srv, bpfSrv := newServices(t)

	testBPFRecording(t, srv, bpfSrv)
}

// TODO(capnspacehook): test with PAM auth enabled, and with a different
// login user once https://github.com/gravitational/teleport/issues/61692
// is fixed.
func TestBPFRecordingWithPAM(t *testing.T) {
	skipIfNoBPF(t)
	skipIfNoPAM(t)

	srv, bpfSrv := newServices(t)
	srv.pamCfg = &servicecfg.PAMConfig{
		Enabled:     true,
		ServiceName: "sshd",
	}

	testBPFRecording(t, srv, bpfSrv)
}

// testBPFRecording runs various commands with Enhanced Session Recording
// enabled and verifies that the recorded events appear in the audit log.
//
// Many sub-tests assert that commands open specific paths, such as
// /etc/passwd. The system paths specified are carefully chosen to
// be paths that should be expected to be present on almost any Linux
// system, and paths that the command should always attempt to open
// given the passed arguments. These paths are either specified in the
// Filesystem Hierarchy Standard (FHS) or are specified in their
// respective man pages.
func testBPFRecording(t *testing.T, srv Server, bpfSrv bpf.BPF) {
	// Create a temp dir and files for commands to use.
	cmdDir := t.TempDir()
	tempFilePath := filepath.Join(cmdDir, "file")
	tempFile, err := os.Create(tempFilePath)
	require.NoError(t, err)
	require.NoError(t, tempFile.Close())
	newFilePath := filepath.Join(cmdDir, "newfile")

	// Lookup paths of programs that are also shell builtins.
	echoPath, err := exec.LookPath("echo")
	require.NoError(t, err, "echo command is required for these tests but was not found")

	// Create a TCP listener for each IP family. Register the listeners
	// to be closed in case the test fails before the connection
	// handling goroutines are started, the listeners will be closed
	// before waitinf for the goroutines to finish otherwise.
	lis4, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		lis4.Close()
	})
	netAddr4 := lis4.Addr().String()
	addr4, port4Str, err := net.SplitHostPort(netAddr4)
	require.NoError(t, err)
	port4, err := strconv.Atoi(port4Str)
	require.NoError(t, err)

	lis6, err := net.Listen("tcp6", "[::1]:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		lis6.Close()
	})
	netAddr6 := lis6.Addr().String()
	addr6, port6Str, err := net.SplitHostPort(netAddr6)
	require.NoError(t, err)
	port6, err := strconv.Atoi(port6Str)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Go(func() { handleConnections(lis4) })
	wg.Go(func() { handleConnections(lis6) })
	t.Cleanup(func() {
		lis4.Close()
		lis6.Close()

		wg.Wait()
	})

	// Create obfuscated script to run.
	obfScript := fmt.Sprintf(`#!/bin/bash
eval $(echo %s | base64 --decode)`,
		base64.StdEncoding.EncodeToString([]byte("ls -h")),
	)
	obfScriptName := "obf.sh"
	obfScriptPath := filepath.Join(cmdDir, obfScriptName)
	err = os.WriteFile(obfScriptPath, []byte(obfScript), 0o700)
	require.NoError(t, err)

	// Create a slice of arguments over the maximum length.
	overMaxArgs := make([]string, maxArgs+3)
	for i := range overMaxArgs {
		overMaxArgs[i] = strconv.Itoa(i) + overMaxArg
	}
	atMaxArgs := slices.Clone(overMaxArgs)
	for i := range atMaxArgs {
		atMaxArgs[i] = atMaxArgs[i][:maxArgLength]
	}
	maxArgPaths := slices.Clone(atMaxArgs)
	for i := range maxArgPaths {
		maxArgPaths[i] = maxArgPaths[i][:maxPathLength]
	}

	// Define the test cases.
	tests := []struct {
		name       string
		command    string
		eventInfos []expectedEvents
	}{
		{
			name:    "no commands",
			command: "true",
		},
		{
			name:    "basic command",
			command: "find " + cmdDir,
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "find",
						args:    []string{cmdDir},
					},
					paths: []string{
						"/proc/filesystems",
						".",
						cmdDir,
					},
				},
			},
		},
		{
			name:    "reading file",
			command: "cat " + tempFilePath,
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "cat",
						args:    []string{tempFilePath},
					},
					paths: []string{tempFilePath},
				},
			},
		},
		{
			name:    "creating file",
			command: "touch " + newFilePath,
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "touch",
						args:    []string{newFilePath},
					},
					paths: []string{newFilePath},
				},
			},
		},
		{
			name:    "writing to a file",
			command: fmt.Sprintf(`%s "Not even a distant land we're on a whole different planet" | tee %s`, echoPath, tempFilePath),
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "echo",
						args:    []string{"Not even a distant land we're on a whole different planet"},
					},
				},
				{
					cmdInfo: commandInfo{
						program: "tee",
						args:    []string{tempFilePath},
					},
					paths: []string{tempFilePath},
				},
			},
		},
		{
			name:    "ls dir",
			command: "ls -la " + cmdDir,
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "ls",
						args:    []string{"-la", cmdDir},
					},
					paths: []string{
						"/proc/filesystems",
						"/etc/nsswitch.conf",
						"/etc/passwd",
						"/etc/group",
						"/etc/localtime",
						cmdDir,
					},
				},
			},
		},
		{
			name:    "shell glob",
			command: fmt.Sprintf(`bash -c "cat %s/*"`, cmdDir),
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "bash",
						args:    []string{"-c", fmt.Sprintf("cat %s/*", cmdDir)},
					},
					paths: []string{
						"/etc/bash.bashrc",
						cmdDir + string(filepath.Separator),
					},
				},
				{
					cmdInfo: commandInfo{
						program: "cat",
						args: []string{
							tempFilePath,
							newFilePath,
							obfScriptPath,
						},
					},
					paths: []string{
						tempFilePath,
						newFilePath,
						obfScriptPath,
					},
				},
			},
		},
		{
			name:    "obfuscated script",
			command: obfScriptPath,
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program:     obfScriptName,
						interpreter: "bash",
						scriptPath:  obfScriptPath,
					},
				},
				{
					cmdInfo: commandInfo{
						program: "base64",
						args:    []string{"--decode"},
					},
				},
				{
					cmdInfo: commandInfo{
						program: "ls",
						args:    []string{"-h"},
					},
					paths: []string{
						"/proc/filesystems",
						".",
					},
				},
			},
		},
		{
			name:    "failed command",
			command: "mkfifo",
			eventInfos: []expectedEvents{
				{
					// this will fail because mkfifo expects an argument
					cmdInfo: commandInfo{
						program:      "mkfifo",
						expectedFail: true,
					},
					paths: []string{"/proc/filesystems"},
				},
			},
		},
		{
			name:    "curl IPv4",
			command: "curl -4 " + netAddr4,
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "curl",
						args:    []string{"-4", netAddr4},
						// curl should exit with 52 since the server will give an empty reply
						expectedFail: true,
					},
					dstAddr: &addrInfo{addr: addr4, port: port4},
				},
			},
		},
		{
			name:    "curl IPv6",
			command: "curl -6 " + netAddr6,
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "curl",
						args:    []string{"-6", netAddr6},
						// curl should exit with 52 since the server will give an empty reply
						expectedFail: true,
					},
					dstAddr: &addrInfo{addr: addr6, port: port6},
				},
			},
		},
		{
			name:    "long arg",
			command: "cat " + longArg,
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "cat",
						args:    []string{longArg},
						// the argument isn't an existing file on disk
						expectedFail: true,
					},
					paths: []string{longArg},
				},
			},
		},
		{
			name:    "arg over max length",
			command: "cat " + overMaxArg,
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "cat",
						args:    []string{overMaxArg[:maxArgLength]},
						// the argument isn't an existing file on disk
						expectedFail: true,
					},
					paths: []string{overMaxArg[:maxPathLength]},
				},
			},
		},
		{
			name:    "max amount of args",
			command: "cat " + strings.Repeat(tempFilePath+" ", maxArgs),
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "cat",
						// argv[0] is counted, so we expect MAXARGS-1 args
						args: slices.Repeat([]string{tempFilePath}, maxArgs-1),
					},
					paths: []string{tempFilePath},
				},
			},
		},
		// TODO(capnspacehook): bpf C code seems to want to add '...' if arguments are truncated but doesn't
		{
			name:    "over max amount of args",
			command: "cat " + strings.Repeat(tempFilePath+" ", maxArgs+3),
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "cat",
						// argv[0] is counted, so we expect MAXARGS-1 args
						args: slices.Repeat([]string{tempFilePath}, maxArgs-1),
					},
					paths: []string{tempFilePath},
				},
			},
		},
		{
			name:    "max amount of args over max length",
			command: "cat " + strings.Join(overMaxArgs[:maxArgs], " "),
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "cat",
						// argv[0] is counted, so we expect MAXARGS-1 args
						args: atMaxArgs[:maxArgs-1],
						// the arguments aren't existing files on disk
						expectedFail: true,
					},
					paths: maxArgPaths[:maxArgs],
				},
			},
		},
		{
			name:    "over max amount of args over max length",
			command: "cat " + strings.Join(overMaxArgs, " "),
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "cat",
						// argv[0] is counted, so we expect MAXARGS-1 args
						args: atMaxArgs[:maxArgs-1],
						// the arguments aren't existing files on disk
						expectedFail: true,
					},
					paths: maxArgPaths,
				},
			},
		},
		{
			name:    "stress test",
			command: fmt.Sprintf("for i in $(seq 1 %d); do { curl %s; ls -lahiZ; } & done; wait", stressTestRunCount, netAddr4),
			eventInfos: []expectedEvents{
				{
					cmdInfo: commandInfo{
						program: "ls",
						args:    []string{"-lahiZ"},
					},
					paths: []string{
						"/proc/filesystems",
						".",
						"/etc/nsswitch.conf",
						"/etc/passwd",
						"/etc/group",
						"/etc/localtime",
					},
					count: stressTestRunCount,
				},
				{
					cmdInfo: commandInfo{
						program: "curl",
						args:    []string{netAddr4},
					},
					dstAddr: &addrInfo{addr: addr4, port: port4},
					count:   stressTestRunCount,
				},
				{
					cmdInfo: commandInfo{
						program: "seq",
						args:    []string{"1", strconv.Itoa(stressTestRunCount)},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedCmdFail := slices.ContainsFunc(tt.eventInfos, func(info expectedEvents) bool {
				return info.cmdInfo.expectedFail
			})

			// Run the command and capture the events.
			recordedEvents := runCommand(t, srv, bpfSrv, tt.command, expectedCmdFail, recordAllEvents)

			commandArgs := make(map[string]int)
			programPaths := make(map[string]string)
			programLibs := make(map[string][]countedValue[string])
			commandOpens := make(map[string][]countedValue[string])
			commandDstAddrs := make(map[string][]countedValue[addrInfo])

			for _, info := range tt.eventInfos {
				cmdInfo := info.cmdInfo
				count := max(info.count, 1)

				// Build a map of expected command arguments.
				cmdArgKey := commandKey(cmdInfo.program, cmdInfo.args)
				commandArgs[cmdArgKey] = count

				// Build a map of program paths on disk. Also build a
				// map of dynamic libraries that should be opened.
				// We conservatively only expect to see libraries loaded
				// in the DT_NEEDED section of the program binary.
				// This gives us more disk events to check against for
				// free, which can be useful to catch rare bugs in the
				// BPF disk tracing program.
				if info.cmdInfo.scriptPath == "" {
					programPath, err := exec.LookPath(cmdInfo.program)
					require.NoError(t, err, "%s command is required for these tests but was not found", cmdInfo.program)
					programPaths[cmdInfo.program] = programPath

					_, ok := programLibs[cmdInfo.program]
					if !ok {
						programLibs[cmdInfo.program] = getProgramLibs(t, programPath, count)
					}
				} else {
					// If a script is being run, the command event will
					// show the script as the program, but it will act
					// like the interpreter.
					programPaths[cmdInfo.program] = info.cmdInfo.scriptPath
					interpPath, err := exec.LookPath(cmdInfo.interpreter)
					require.NoError(t, err)

					_, ok := programLibs[cmdInfo.program]
					if !ok {
						programLibs[cmdInfo.program] = getProgramLibs(t, interpPath, count)
					}
				}

				expectedPaths := commandOpens[cmdInfo.program]
				commandOpens[cmdInfo.program] = append(expectedPaths, makeCounted(info.paths, count)...)

				// Build a map of program destination addresses.
				if info.dstAddr != nil {
					addrs := commandDstAddrs[cmdInfo.program]
					commandDstAddrs[cmdInfo.program] = append(addrs, countedValue[addrInfo]{value: *info.dstAddr, count: count})
				}
			}

			// Check that the emitted events have expected contents.
			for _, event := range recordedEvents {
				switch e := event.(type) {
				case *apievents.SessionCommand:
					t.Logf("  command event: Command=%s Path=%s Args=[%s] NArgs=%d", e.BPFMetadata.Program, e.Path, quoteStrings(e.Argv), len(e.Argv))

					checkCommandEvent(t, e, programPaths, commandArgs)
				case *apievents.SessionDisk:
					t.Logf("  disk event: Command=%s Path=%s", e.BPFMetadata.Program, e.Path)

					if checkDiskEvent(t, e, programLibs, true) {
						continue
					}
					checkDiskEvent(t, e, commandOpens, false)
				case *apievents.SessionNetwork:
					t.Logf("  network event: Command=%s SrcAddr=%s DstAddr=%s DstPort=%d", e.BPFMetadata.Program, e.SrcAddr, e.DstAddr, e.DstPort)

					checkNetworkEvent(t, e, commandDstAddrs)
				}
			}

			// Check that expected events were found the expected number of times.
			for cmd, count := range commandArgs {
				if count > 0 {
					t.Errorf("error: command event for %s was expected %d more times", cmd, count)
				}
			}
			for cmd, paths := range commandOpens {
				for _, path := range paths {
					if path.count > 0 {
						t.Errorf("error: disk event for program %q opening %q was expected %d more times", cmd, path.value, path.count)
					}
				}
			}
			for cmd, addrs := range commandDstAddrs {
				for _, addr := range addrs {
					if addr.count > 0 {
						t.Errorf("error: network event for program %q with destination address %q was expected %d more times", cmd, addr.value, addr.count)
					}
				}
			}
			for cmd, libs := range programLibs {
				for _, lib := range libs {
					if lib.count > 0 {
						t.Errorf("error: disk event for program %q opening library %q was expected %d more times", cmd, lib.value, lib.count)
					}
				}
			}
		})
	}
}

func TestBPFMonitoring(t *testing.T) {
	skipIfNoBPF(t)

	srv, bpfSrv := newServices(t)

	testBPFMonitoring(t, srv, bpfSrv)
}

// TODO(capnspacehook): test with PAM auth enabled, and with a different
// login user once https://github.com/gravitational/teleport/issues/61692
// is fixed.
func TestBPFMonitoringWithPAM(t *testing.T) {
	skipIfNoBPF(t)
	skipIfNoPAM(t)

	srv, bpfSrv := newServices(t)
	srv.pamCfg = &servicecfg.PAMConfig{
		Enabled:     true,
		ServiceName: "sshd",
	}

	testBPFMonitoring(t, srv, bpfSrv)
}

// testBPFMonitoring verifies that events will not be emitted for
// syscalls that happen outside the monitored SSH session.
func testBPFMonitoring(t *testing.T, srv Server, bpfSrv bpf.BPF) {
	lis, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Go(func() { handleConnections(lis) })
	t.Cleanup(func() {
		lis.Close()
		wg.Wait()
	})

	// Run a command that will guarantee to run longer than our curl
	// commands will.
	eventsCh := make(chan []apievents.AuditEvent)
	wg.Go(func() {
		eventsCh <- runCommand(t, srv, bpfSrv, "sleep 3", false, recordAllEvents)
	})

	// Run curl commands that the bpf programs should ignore.
	for range 5 {
		select {
		case <-eventsCh:
			t.Fatal("monitored command finished before curl command(s) did")
		default:
		}

		cmd := exec.CommandContext(t.Context(), "curl", "-4", lis.Addr().String())
		// curl should exit with 56 since the server will reset the connection.
		var exitErr *exec.ExitError
		require.ErrorAs(t, cmd.Run(), &exitErr)
	}

	// Ensure stopping the monitored command early works properly.
	var events []apievents.AuditEvent
	select {
	case events = <-eventsCh:
		require.NotEmpty(t, events)
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for command to finish.")
	}

	// Check that only configured events were recorded.
	var cmdEventCnt int
	var diskEventCnt int
	for _, e := range events {
		switch e := e.(type) {
		case *apievents.SessionCommand:
			cmdEventCnt++
			require.NotEqual(t, "curl", e.BPFMetadata.Program)
		case *apievents.SessionDisk:
			// Many disk events should be emitted but we only care that
			// no curl events were emitted.
			diskEventCnt++
			require.NotEqual(t, "curl", e.BPFMetadata.Program)
		case *apievents.SessionNetwork:
			t.Fatal("Did not expect a network event")
		default:
			t.Fatalf("Unexpected event type: %T", e)
		}
	}

	require.GreaterOrEqual(t, cmdEventCnt, 1)
	require.GreaterOrEqual(t, diskEventCnt, 1)
}

// TestBPFRoleOptions verifies that only event types configured in
// role options will be recorded in the audit log.
func TestBPFRoleOptions(t *testing.T) {
	skipIfNoBPF(t)

	srv, bpfSrv := newServices(t)

	lis, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Go(func() { handleConnections(lis) })
	t.Cleanup(func() {
		lis.Close()
		wg.Wait()
	})

	tests := []struct {
		name   string
		events map[string]struct{}
	}{
		{
			name: "no events",
		},
		{
			name:   "command events",
			events: map[string]struct{}{constants.EnhancedRecordingCommand: {}},
		},
		{
			name:   "disk events",
			events: map[string]struct{}{constants.EnhancedRecordingDisk: {}},
		},
		{
			name:   "network events",
			events: map[string]struct{}{constants.EnhancedRecordingNetwork: {}},
		},
		{
			name: "command and disk events",
			events: map[string]struct{}{
				constants.EnhancedRecordingCommand: {},
				constants.EnhancedRecordingDisk:    {},
			},
		},
		{
			name: "command and network events",
			events: map[string]struct{}{
				constants.EnhancedRecordingCommand: {},
				constants.EnhancedRecordingNetwork: {},
			},
		},
		{
			name: "disk and network events",
			events: map[string]struct{}{
				constants.EnhancedRecordingDisk:    {},
				constants.EnhancedRecordingNetwork: {},
			},
		},
	}

	// curl can generate all 3 types of events.
	command := "curl " + lis.Addr().String()

	// Teleport re-execs (executing /proc/self/exe) and the /bin/sh child
	// may trigger events that we can ignore when checking that all
	// events are from 'curl'.
	skipProgramEvents := []string{"exe", "sh"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run the command and capture the events.
			recordedEvents := runCommand(t, srv, bpfSrv, command, true, tt.events)

			// Check that only configured events were recorded.
			if len(tt.events) == 0 {
				require.Empty(t, recordedEvents)
				return
			}
			require.NotEmpty(t, recordedEvents)

			_, recCmd := tt.events[constants.EnhancedRecordingCommand]
			_, recDisk := tt.events[constants.EnhancedRecordingDisk]
			_, recNet := tt.events[constants.EnhancedRecordingNetwork]

			for _, e := range recordedEvents {
				switch e := e.(type) {
				case *apievents.SessionCommand:
					require.True(t, recCmd, "expected not to see command events")
					if !slices.Contains(skipProgramEvents, e.BPFMetadata.Program) {
						require.Equal(t, "curl", e.BPFMetadata.Program)
					}
				case *apievents.SessionDisk:
					require.True(t, recDisk, "expected not to see disk events")
					if !slices.Contains(skipProgramEvents, e.BPFMetadata.Program) {
						require.Equal(t, "curl", e.BPFMetadata.Program)
					}
				case *apievents.SessionNetwork:
					require.True(t, recNet, "expected not to see network events")
					// Both Teleport re-execs and their child /bin/sh
					// processes shouldn't generate network events, so
					// we can assert that all network events are from
					// curl here.
					require.Equal(t, "curl", e.BPFMetadata.Program)
				default:
					t.Fatalf("Unexpected event type: %T", e)
				}
			}
		})
	}
}

type countedValue[T comparable] struct {
	value T
	count int
}

func makeCounted[T comparable](s []T, count int) []countedValue[T] {
	countedValues := make([]countedValue[T], len(s))
	for i, v := range s {
		countedValues[i] = countedValue[T]{value: v, count: count}
	}
	return countedValues
}

// commandKey returns a string that can be used as a unique map key for
// a command, given the command name and arguments. The returned string
// describes the command clearly when printed.
func commandKey(program string, args []string) string {
	return fmt.Sprintf("%s [%s]", program, quoteStrings(args))
}

func newServices(t *testing.T) (*mockServer, bpf.BPF) {
	t.Helper()

	bpfSrv, err := bpf.New(&servicecfg.BPFConfig{Enabled: true})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, bpfSrv.Close(true))
	})

	srv := newMockServer(t)
	srv.bpf = bpfSrv

	return srv, bpfSrv
}

func handleConnections(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		conn.Close()
	}
}

// runCommand runs the given command with Enhanced Session Recording
// enabled and returns the recorded events.
func runCommand(t *testing.T, srv Server, bpfSrv bpf.BPF, command string, expectedCmdFail bool, recordEvents map[string]struct{}) []apievents.AuditEvent {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	scx := newExecServerContext(t, srv)
	scx.Identity.AccessPermit = &decisionpb.SSHAccessPermit{
		BpfEvents: []string{
			constants.EnhancedRecordingCommand,
			constants.EnhancedRecordingDisk,
			constants.EnhancedRecordingNetwork,
		},
	}
	scx.execRequest.SetCommand(command)

	channel := newMockSSHChannel()

	t.Logf("running %q", command)

	_, err := scx.execRequest.Start(ctx, channel)
	require.NoError(t, err)

	t.Log("reading audit session ID")

	sessionID, err := scx.execRequest.ReadAuditSessionID()
	require.NoError(t, err)

	// Create a fake audit log that can be used to capture the events emitted.
	emitter := &eventstest.MockRecorderEmitter{}

	sessionCtx := &bpf.SessionContext{
		Namespace:      apidefaults.Namespace,
		SessionID:      uuid.New().String(),
		ServerID:       uuid.New().String(),
		ServerHostname: "ip-172-31-11-148",
		Login:          "foo",
		User:           "foo@example.com",
		AuditSessionID: sessionID,
		Emitter:        emitter,
		Events:         recordEvents,
	}
	err = bpfSrv.OpenSession(sessionCtx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bpfSrv.CloseSession(sessionCtx))
	})

	// Signal to child that it may execute the requested program.
	t.Log("sending continue signal")
	scx.execRequest.Continue()

	// Create a channel that will be used to signal that execution is complete.
	var wg sync.WaitGroup
	cmdDone := make(chan error, 1)

	wg.Go(func() {
		execReq, ok := scx.execRequest.(*localExec)
		require.True(t, ok)
		cmdDone <- execReq.Cmd.Wait()
	})

	t.Log("waiting for command to finish")

	// Read from SSH channel to unblock writes; the mock SSH channel
	// uses os.Pipe under the hood.
	wg.Go(func() {
		t.Log("output:")

		stdout := make([]byte, 1024)
		for {
			_, err := channel.Read(stdout)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				t.Logf("failed to read from channel: %v", err)
				continue
			}
			t.Log(string(stdout))
		}
	})
	t.Cleanup(func() {
		_ = channel.CloseWrite()
		wg.Wait()
	})

	// Program should have executed now. If the complete signal has not come
	// over the context, something failed.
	select {
	case <-ctx.Done():
		// We're not interested in the error, we just want to clean up the
		// process.
		_ = scx.killShellw.Close()
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("Timed out waiting for process to finish.")
		}
	case err := <-cmdDone:
		if expectedCmdFail {
			require.Error(t, err)
			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr)
		} else {
			require.NoError(t, err)
		}
	}

	return emitter.Events()
}

func getProgramLibs(t *testing.T, path string, count int) []countedValue[string] {
	elfFile, err := elf.Open(path)
	require.NoError(t, err)
	importedLibs, err := elfFile.ImportedLibraries()
	require.NoError(t, err)

	return makeCounted(importedLibs, count)
}

// checkCommandEvent checks that the given command event matches an
// expected command event. The returned list of command infos will
// have the matching command removed.
func checkCommandEvent(t *testing.T, e *apievents.SessionCommand, cmdPaths map[string]string, commandArgs map[string]int) {
	t.Helper()

	runCmd := e.BPFMetadata.Program
	// Teleport will run the command as 'shell -c <command>' if command
	// is set, and the shell will be 'sh' when IsTestStub is true.
	if runCmd == "sh" {
		return
	}

	path, ok := cmdPaths[runCmd]
	require.True(t, ok, "unexpected command %q", runCmd)
	require.Equal(t, path, e.Path, "unexpected executable path for program %q", runCmd)

	cmdArgKey := commandKey(runCmd, e.Argv)
	count, ok := commandArgs[cmdArgKey]
	require.True(t, ok, "unexpected command %s", cmdArgKey)
	commandArgs[cmdArgKey] = count - 1

	t.Log("command event is expected!")
}

// checkDiskEvent returns true if the given disk event matches an
// expected disk event. If the event is an expected event, the matched
// path will be removed from the expected paths map.
func checkDiskEvent(t *testing.T, e *apievents.SessionDisk, expectedPaths map[string][]countedValue[string], matchBase bool) bool {
	t.Helper()

	path := e.Path
	if matchBase {
		path = filepath.Base(e.Path)
	}

	if checkEvent(e.BPFMetadata.Program, path, expectedPaths) {
		t.Log("disk event is expected!")
		return true
	}

	return false
}

// checkNetworkEvent returns true if the given network event matches an
// expected network event. If the event is an expected event, the matched
// destination address will be removed from the expected destination
// addresses map.
func checkNetworkEvent(t *testing.T, e *apievents.SessionNetwork, expectedDstAddrs map[string][]countedValue[addrInfo]) bool {
	t.Helper()

	netAddr := addrInfo{addr: e.DstAddr, port: int(e.DstPort)}
	if checkEvent(e.BPFMetadata.Program, netAddr, expectedDstAddrs) {
		t.Log("network event is expected!")
		return true
	}

	return false
}

// checkEvent returns true if the given event field matches an expected
// event field. If the event is expected, the matched event field's count
// will be decremented.
func checkEvent[T comparable](program string, eventField T, expectedFields map[string][]countedValue[T]) bool {
	fields, ok := expectedFields[program]
	if !ok {
		return false
	}

	for i := range fields {
		if fields[i].value == eventField {
			fields[i].count--
			expectedFields[program] = fields
			return true
		}
	}

	return false
}

func quoteStrings(s []string) string {
	var quoted []string
	for _, v := range s {
		quoted = append(quoted, strconv.Quote(v))
	}
	return strings.Join(quoted, ", ")
}

// skipIfNoBPF skips the test if BPF tests are not enabled or the test is not run
// as root.
func skipIfNoBPF(t *testing.T) {
	t.Helper()

	if os.Getenv("TELEPORT_BPF_TEST") == "" {
		t.Skip("BPF testing is disabled. Set TELEPORT_BPF_TEST environment variable to enable.")
	}
	testutils.RequireRoot(t)
}

// skipIfNoPAM skips the test if PAM support is not built in or the host
// does not support PAM.
func skipIfNoPAM(t *testing.T) {
	t.Helper()

	if !pam.BuildHasPAM() || !pam.SystemHasPAM() {
		t.Skip("PAM support not enabled, skipping tests")
	}

	if _, err := os.Stat("/etc/pam.d/sshd"); err != nil {
		t.Skip("required PAM policy sshd not found, skipping tests")
	}
}
