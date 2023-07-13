//go:build windows
// +build windows

/*
Copyright 2018 Gravitational, Inc.

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

package agentconn

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/Microsoft/go-winio"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	apiutils "github.com/gravitational/teleport/api/utils"
)

const namedPipe = `\\.\pipe\openssh-ssh-agent`

// Dial creates net.Conn to a SSH agent listening on a Windows named pipe.
// This is behind a build flag because winio.DialPipe is only available on
// Windows. If connecting to a named pipe fails and we're in a Cygwin
// environment, a connection to a Cygwin SSH agent will be attempted.
func Dial(socket string) (net.Conn, error) {
	conn, err := winio.DialPipe(namedPipe, nil)
	if err != nil {
		// MSYSTEM is used to specify what Cygwin environment is used;
		// if it exists, there's a very good chance we're in a Cygwin
		// environment
		if msys, ok := os.LookupEnv("MSYSTEM"); ok && msys != "" {
			conn, err := dialCygwin(socket)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return conn, nil
		}

		return nil, trace.Wrap(err)
	}

	return conn, nil
}

const wellKnownSIDPrefix = "S-1-5-"

var (
	// format of the contents of a file created by Cygwin 'ssh-agent'
	cygwinSocket = regexp.MustCompile(`!<socket >(\d+) (s )?([A-Fa-f0-9-]+)`)
	// format of an output line from Cygwin 'ps'
	psLine = regexp.MustCompile(`(?m)^\s+\d+\s+\d+\s+\d+\s+\d+\s+\?\s+(\d+)`)
)

// attempt to connect a Cygwin SSH agent socket. Some code adapted from
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go
func dialCygwin(socket string) (net.Conn, error) {
	// the "socket" is actually a file Cygwin uses to communicate what the
	// actual socket is and the parameters for the handshake
	contents, err := os.ReadFile(socket)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sockMatches := cygwinSocket.FindStringSubmatch(string(contents))
	if len(sockMatches) != 4 {
		return nil, trace.Errorf("error reading Cygwin socket")
	}
	port := sockMatches[1]
	if sockMatches[2] != "s " {
		return nil, trace.NotImplemented("dialing mysysgit ssh-agent sockets is not supported")
	}
	key := sockMatches[3]

	u, err := apiutils.CurrentUser()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var uid int64
	var unsureOfUID bool
	if !strings.HasPrefix(u.Uid, wellKnownSIDPrefix) {
		// the format of the SID isn't supported, fallback to getting
		// the UID from 'ps'
		uid, err = getCygwinUIDFromPS()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// Attempt to get a Cygwin UID from a Windows SID. Details of
		// UID -> SID mapping here: https://cygwin.com/cygwin-ug-net/ntsec.html
		sidParts := strings.Split(u.Uid, "-")
		if len(sidParts) == 4 {
			// well-known SIDs in the NT_AUTHORITY domain of the S-1-5-RID type
			uid, err = strconv.ParseInt(sidParts[3], 10, 32)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		} else if len(sidParts) == 5 {
			// other well-known SIDs that aren't groups
			x, err := strconv.ParseInt(sidParts[3], 10, 32)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			rid, err := strconv.ParseInt(sidParts[4], 10, 32)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			uid = 0x1000*x + rid
		} else if len(sidParts) == 8 {
			// SIDs from the local machine's SAM, the machine's primary
			// domain, or a trusted domain of the machine's primary domain
			uid, err = strconv.ParseInt(sidParts[7], 10, 32)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			unsureOfUID = true
		} else {
			// the format of the SID isn't supported, fallback to getting
			// the UID from 'ps'
			uid, err = getCygwinUIDFromPS()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	// dial socket and complete handshake
	var conn net.Conn
	if !unsureOfUID {
		// we're confident in what the Cygwin UID is, only make one attempt
		// at establishing a connection
		conn, err = attemptCygwinHandshake(port, key, uid)
		if err == nil {
			return conn, nil
		}
	} else {
		// the Cygwin UID could be built a few different ways; attempt
		// with all UIDs until one succeeds
		cygwinRIDNums := []int64{0x30000, 0x100000, 0x80000000}
		for _, num := range cygwinRIDNums {
			conn, err = attemptCygwinHandshake(port, key, num+uid)
			if err == nil {
				return conn, nil
			}
		}

		// none of those UIDs worked, fallback to getting UID from 'ps'
		uid, err = getCygwinUIDFromPS()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conn, err = attemptCygwinHandshake(port, key, uid)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return conn, nil
}

// use Cygwin 'ps' binary to get the Cygwin UID of the current user
func getCygwinUIDFromPS() (int64, error) {
	// Cygwin 'bash' is used to call 'ps' so a Cygwin environment can be
	// inherited by 'ps'
	psOutput, err := exec.Command("bash.exe", "-c", "ps").Output()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	psMatches := psLine.FindStringSubmatch(string(psOutput))
	if len(psMatches) != 2 {
		return 0, trace.Errorf("error reading Cygwin ps output")
	}
	uid, err := strconv.ParseInt(psMatches[1], 10, 32)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return uid, nil
}

// connect to a listening socket of a Cygwin SSH agent and attempt to
// preform a successful handshake with it. Handshake details here:
// https://stackoverflow.com/questions/23086038/what-mechanism-is-used-by-msys-cygwin-to-emulate-unix-domain-sockets
func attemptCygwinHandshake(port, key string, uid int64) (net.Conn, error) {
	logrus.Debugf("[KEY AGENT] attempting a handshake with Cygwin ssh-agent socket; port=%s uid=%d", port, uid)

	conn, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 1. send hex-encoded GUID
	keyBuf := make([]byte, 16)
	fmt.Sscanf(
		key,
		"%02x%02x%02x%02x-%02x%02x%02x%02x-%02x%02x%02x%02x-%02x%02x%02x%02x",
		&keyBuf[3], &keyBuf[2], &keyBuf[1], &keyBuf[0],
		&keyBuf[7], &keyBuf[6], &keyBuf[5], &keyBuf[4],
		&keyBuf[11], &keyBuf[10], &keyBuf[9], &keyBuf[8],
		&keyBuf[15], &keyBuf[14], &keyBuf[13], &keyBuf[12],
	)

	if _, err = conn.Write(keyBuf); err != nil {
		return nil, trace.Wrap(err)
	}

	// 2. server echoes the same bytes, read them
	if _, err = conn.Read(keyBuf); err != nil {
		return nil, trace.Wrap(err)
	}

	// 3. send PID, Cygwin UID and Cygwin GID of the calling process
	pidsUids := make([]byte, 12)
	pid := os.Getpid()
	gid := pid // for cygwin's AF_UNIX -> AF_INET, pid = gid
	binary.LittleEndian.PutUint32(pidsUids, uint32(pid))
	binary.LittleEndian.PutUint32(pidsUids[4:], uint32(uid))
	binary.LittleEndian.PutUint32(pidsUids[8:], uint32(gid))
	if _, err = conn.Write(pidsUids); err != nil {
		return nil, trace.Wrap(err)
	}

	// 4. server echoes the same bytes, read them
	if _, err = conn.Read(pidsUids); err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}
