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

package srv

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	utils.InitLoggerForTests()

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
	expectedHostname, err := os.Hostname()
	if err != nil {
		expectedHostname = "localhost"
	}
	expectedMeta := apievents.UserMetadata{
		User:                 "teleportUser",
		Login:                expectedUsr.Username,
		Impersonator:         "",
		AWSRoleARN:           "",
		AccessRequests:       []string(nil),
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     []uint8(nil),
		XXX_sizecache:        0,
	}

	tests := []struct {
		inCommand  string
		inError    error
		outCommand string
		outCode    string
	}{
		// Successful execution.
		{
			inCommand:  "exit 0",
			inError:    nil,
			outCommand: "exit 0",
			outCode:    strconv.Itoa(teleport.RemoteCommandSuccess),
		},
		// Exited with error.
		{
			inCommand:  "exit 255",
			inError:    fmt.Errorf("unknown error"),
			outCommand: "exit 255",
			outCode:    strconv.Itoa(teleport.RemoteCommandFailure),
		},
		// Command injection.
		{
			inCommand:  "/bin/teleport scp --remote-addr=127.0.0.1:50862 --local-addr=127.0.0.1:54895 -f ~/file.txt && touch /tmp/new.txt",
			inError:    fmt.Errorf("unknown error"),
			outCommand: "/bin/teleport scp --remote-addr=127.0.0.1:50862 --local-addr=127.0.0.1:54895 -f ~/file.txt && touch /tmp/new.txt",
			outCode:    strconv.Itoa(teleport.RemoteCommandFailure),
		},
	}
	for _, tt := range tests {
		emitExecAuditEvent(scx, tt.inCommand, tt.inError)
		execEvent := rec.emitter.LastEvent().(*apievents.Exec)
		require.Equal(t, tt.outCommand, execEvent.Command)
		require.Equal(t, tt.outCode, execEvent.ExitCode)
		require.Equal(t, expectedMeta, execEvent.UserMetadata)
		require.Equal(t, "testHostUUID", execEvent.ServerID)
		require.Equal(t, expectedHostname, execEvent.ServerHostname)
		require.Equal(t, "testNamespace", execEvent.ServerNamespace)
		require.Equal(t, "xxx", execEvent.SessionID)
		require.Equal(t, "10.0.0.5:4817", execEvent.RemoteAddr)
		require.Equal(t, "127.0.0.1:3022", execEvent.LocalAddr)
		require.NotZero(t, events.EventID)
	}
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
	scx := newTestServerContext(t, srv, nil)

	term, err := newLocalTerminal(scx)
	require.NoError(t, err)
	term.SetTermType("xterm")

	rec := &mockRecorder{done: false}
	scx.session = &session{
		id:       "xxx",
		term:     term,
		emitter:  rec,
		recorder: rec,
	}
	err = scx.SetSSHRequest(&ssh.Request{Type: sshutils.ExecRequest})
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, scx.session.term.Close()) })

	return scx
}
