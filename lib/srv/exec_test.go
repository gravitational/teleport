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

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// TestEmitExecAuditEvent make sure the full command and exit code for a
// command is always recorded.
func TestEmitExecAuditEvent(t *testing.T) {
	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)

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

	var tests = []struct {
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
		execEvent := srv.MockEmitter.LastEvent().(*apievents.Exec)
		require.Equal(t, tt.outCommand, execEvent.Command)
		require.Equal(t, tt.outCode, execEvent.ExitCode)
		require.Equal(t, expectedMeta, execEvent.UserMetadata)
		require.Equal(t, "testHostUUID", execEvent.ServerID)
		require.Equal(t, expectedHostname, execEvent.ServerHostname)
		require.Equal(t, "testNamespace", execEvent.ServerNamespace)
		require.Equal(t, "xxx", execEvent.SessionID)
		require.Equal(t, "10.0.0.5:4817", execEvent.RemoteAddr)
		require.Equal(t, "127.0.0.1:3022", execEvent.LocalAddr)
	}
}

func TestLoginDefsParser(t *testing.T) {
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

	scx.session = &session{id: "xxx"}
	scx.session.term = term
	scx.request = &ssh.Request{Type: sshutils.ExecRequest}

	t.Cleanup(func() { require.NoError(t, scx.session.term.Close()) })

	return scx
}
