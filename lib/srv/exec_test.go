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

package srv

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"reflect"
	"strconv"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	modules.SetInsecureTestMode(true)
	// If the test is re-executing itself, execute the command that comes over
	// the pipe.
	if IsReexec() {
		RunAndExit(os.Args[1])
		return
	}

	// Otherwise run tests as normal.
	code := m.Run()
	os.Exit(code)
}

// TestEmitExecAuditEvent make sure the full command and exit code for a
// command is always recorded.
func TestEmitExecAuditEvent(t *testing.T) {
	t.Parallel()

	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)

	rec, ok := scx.session.recorder.(*mockRecorder)
	require.True(t, ok)

	expectedUsr, err := user.Current()
	require.NoError(t, err)
	expectedHostname := "testHost"

	expectedMeta := apievents.UserMetadata{
		User:                 "teleportUser",
		Login:                expectedUsr.Username,
		Impersonator:         "",
		AWSRoleARN:           "",
		AccessRequests:       []string(nil),
		UserKind:             apievents.UserKind_USER_KIND_HUMAN,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     []uint8(nil),
		XXX_sizecache:        0,
	}

	tests := []struct {
		name     string
		inResult ExecResult
	}{
		{
			name: "success",
			inResult: ExecResult{
				Command: "exit 0",
				Error:   nil,
				Code:    teleport.RemoteCommandSuccess,
			},
		},
		{
			name: "exit with error",
			inResult: ExecResult{
				Command: "exit 255",
				Error:   fmt.Errorf("unknown error"),
				Code:    teleport.RemoteCommandFailure,
			},
		},
		{
			name: "command injection",
			inResult: ExecResult{
				Command: "/bin/teleport scp --remote-addr=127.0.0.1:50862 --local-addr=127.0.0.1:54895 -f ~/file.txt && touch /tmp/new.txt",
				Error:   fmt.Errorf("unknown error"),
				Code:    teleport.RemoteCommandFailure,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitExecAuditEvent(scx, tt.inResult)
			execEvent := rec.emitter.LastEvent().(*apievents.Exec)
			require.Equal(t, tt.inResult.Command, execEvent.Command)
			if tt.inResult.Error != nil {
				require.Equal(t, tt.inResult.Error.Error(), execEvent.Error)
			} else {
				require.Empty(t, execEvent.Error)
			}
			require.Equal(t, strconv.Itoa(tt.inResult.Code), execEvent.ExitCode)
			require.Equal(t, expectedMeta, execEvent.UserMetadata)
			require.Equal(t, "123", execEvent.ServerID)
			require.Equal(t, "abc", execEvent.ForwardedBy)
			require.Equal(t, expectedHostname, execEvent.ServerHostname)
			require.Equal(t, "testNamespace", execEvent.ServerNamespace)
			require.Equal(t, "xxx", execEvent.SessionID)
			require.Equal(t, "10.0.0.5:4817", execEvent.RemoteAddr)
			require.Equal(t, "127.0.0.1:3022", execEvent.LocalAddr)
			require.NotEmpty(t, events.EventID)
		})
	}
}

func TestRemoteExecResultFromWaitErr(t *testing.T) {
	t.Parallel()

	code, err := remoteExecResultFromWaitErr(nil)
	require.NoError(t, err)
	require.Equal(t, teleport.RemoteCommandSuccess, code)

	exitErr := newSSHExitErrorWithStatus(t, 42)
	code, err = remoteExecResultFromWaitErr(exitErr)
	require.Same(t, exitErr, err)
	require.Equal(t, 42, code)

	otherErr := errors.New("boom")
	code, err = remoteExecResultFromWaitErr(otherErr)
	require.Same(t, otherErr, err)
	require.Equal(t, teleport.RemoteCommandFailure, code)
}

func newSSHExitErrorWithStatus(t *testing.T, status int64) error {
	t.Helper()

	exitErr := &ssh.ExitError{}

	v := reflect.ValueOf(exitErr).Elem()
	waitMsgField := v.FieldByName("Waitmsg")
	require.True(t, waitMsgField.IsValid())
	statusField := waitMsgField.FieldByName("status")
	require.True(t, statusField.IsValid())

	reflect.NewAt(statusField.Type(), unsafe.Pointer(statusField.UnsafeAddr())).Elem().SetInt(status)

	return exitErr
}

func TestLoginDefsParser(t *testing.T) {
	t.Parallel()

	expectedEnvSuPath := "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/bar"
	expectedSuPath := "PATH=/usr/local/bin:/usr/bin:/bin:/foo"

	require.Equal(t, expectedEnvSuPath, getDefaultEnvPath("0", "../../fixtures/login.defs"))
	require.Equal(t, expectedSuPath, getDefaultEnvPath("1000", "../../fixtures/login.defs"))
	require.Equal(t, defaultEnvPath, getDefaultEnvPath("1000", "bad/file"))
}

func newExecServerContext(t *testing.T, srv Server) *ServerContext {
	scx := newTestServerContext(t, srv, nil, nil)

	term, err := newLocalTerminal(scx)
	require.NoError(t, err)
	term.SetTermType("xterm")

	rec := &mockRecorder{done: false}
	scx.session = &session{
		id:       "xxx",
		term:     term,
		emitter:  rec,
		recorder: rec,
		scx:      scx,
	}
	err = scx.SetSSHRequest(&ssh.Request{Type: sshutils.ExecRequest})
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, scx.session.term.Close()) })

	return scx
}
