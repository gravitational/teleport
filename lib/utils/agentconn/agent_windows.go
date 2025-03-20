//go:build windows
// +build windows

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

package agentconn

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"

	"github.com/Microsoft/go-winio"
	"github.com/gravitational/trace"
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
		if msys := os.Getenv("MSYSTEM"); msys != "" {
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

// SIDs that are computer or domain SIDs start with this prefix.
const wellKnownSIDPrefix = "S-1-5-"

var (
	// Format of the contents of a file created by Cygwin 'ssh-agent'.
	// After '!<socket >', the listening port is specified, followed by
	// an optional 's ' that is sometimes set depending on the implementation,
	// ending with a GUID which is used as a shared secret when handshaking
	// with the SSH agent.
	// example:
	// !<socket >51463 s 043B28B0-30D7E90E-027C556A-314067F9
	cygwinSocket = regexp.MustCompile(`!<socket >(\d+) (s )?([A-Fa-f0-9-]+)`)
	// format of an output line from Cygwin 'ps'
	// example:
	//       PID    PPID    PGID     WINPID   TTY         UID    STIME COMMAND
	//      1634    1540    1634       7356     ?      197608 14:31:52 /usr/bin/ps
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
		return nil, trace.Errorf("could not find necessary information in Cygwin socket file")
	}
	port := sockMatches[1]
	if sockMatches[2] != "s " {
		return nil, trace.NotImplemented("dialing mysysgit ssh-agent sockets is not supported")
	}
	key := sockMatches[3]

	u, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var uid uint32
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
			u, err := strconv.ParseUint(sidParts[3], 10, 32)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			uid = uint32(u)
		} else if len(sidParts) == 5 {
			// other well-known SIDs that aren't groups
			x, err := strconv.ParseUint(sidParts[3], 10, 32)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			rid, err := strconv.ParseUint(sidParts[4], 10, 32)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			uid = uint32(0x1000*x + rid)
		} else if len(sidParts) == 8 {
			// SIDs from the local machine's SAM, the machine's primary
			// domain, or a trusted domain of the machine's primary domain
			u, err := strconv.ParseUint(sidParts[7], 10, 32)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			uid = uint32(u)
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
		cygwinRIDNums := []uint32{0x30000, 0x100000, 0x80000000}
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
func getCygwinUIDFromPS() (uint32, error) {
	psOutput, err := exec.Command("ps").Output()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	psMatches := psLine.FindStringSubmatch(string(psOutput))
	if len(psMatches) != 2 {
		return 0, trace.Errorf("UID not found in Cygwin ps output")
	}
	uid, err := strconv.ParseUint(psMatches[1], 10, 32)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return uint32(uid), nil
}

// connect to a listening socket of a Cygwin SSH agent and attempt to
// preform a successful handshake with it. Handshake details here:
// https://stackoverflow.com/questions/23086038/what-mechanism-is-used-by-msys-cygwin-to-emulate-unix-domain-sockets
func attemptCygwinHandshake(port, key string, uid uint32) (net.Conn, error) {
	slog.DebugContext(context.Background(), "[KEY AGENT] attempting a handshake with Cygwin ssh-agent socket", "port", port, "uid", uid)

	conn, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 1. send hex-decoded GUID in little endian
	keyBuf := make([]byte, 0, 16)
	dst := make([]byte, 4)
	// handle each part of the GUID in order
	for i := 8; i <= len(key); i += 9 {
		_, err := hex.Decode(dst, []byte(key)[i-8:i])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		dst[0], dst[1], dst[2], dst[3] = dst[3], dst[2], dst[1], dst[0]
		keyBuf = append(keyBuf, dst...)
	}

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
	binary.LittleEndian.PutUint32(pidsUids[4:], uid)
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
