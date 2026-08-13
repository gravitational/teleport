// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package sshagent

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// A valid Cygwin socket-file GUID: four 8-hex-digit groups separated by '-',
// which attemptCygwinHandshake decodes into the 16-byte shared secret.
const testCygwinGUID = "043B28B0-30D7E90E-027C556A-314067F9"

// resetCygwinCache clears the process-global UID cache before and after a test
// so cases don't leak state into one another.
func resetCygwinCache(t *testing.T) {
	t.Helper()
	storeCygwinUID(0, false)
	t.Cleanup(func() { storeCygwinUID(0, false) })
}

// newTestKeyring returns an in-memory agent holding a single key, along with
// that key's public half for assertions.
func newTestKeyring(t *testing.T) (agent.Agent, ssh.PublicKey) {
	t.Helper()
	keyring := agent.NewKeyring()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: priv}))
	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return keyring, sshPub
}

// startFakeCygwinAgent serves keyring behind the Cygwin AF_UNIX-over-TCP
// handshake. It returns the path to a Cygwin-style socket file describing the
// listener and the listener's port. accept decides whether a handshake
// presenting a given UID is allowed; a rejected handshake is dropped before the
// agent is served, mimicking a UID mismatch.
func startFakeCygwinAgent(t *testing.T, keyring agent.Agent, accept func(uid uint32) bool) (socketFile, port string) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go serveFakeCygwinConn(conn, keyring, accept)
		}
	}()

	port = fmt.Sprint(l.Addr().(*net.TCPAddr).Port)
	socketFile = filepath.Join(t.TempDir(), "agent.sock")
	contents := fmt.Sprintf("!<socket >%s s %s", port, testCygwinGUID)
	require.NoError(t, os.WriteFile(socketFile, []byte(contents), 0o600))
	return socketFile, port
}

// serveFakeCygwinConn performs the four-step Cygwin handshake, then hands the
// connection to the SSH agent server unless the presented UID is rejected.
func serveFakeCygwinConn(conn net.Conn, keyring agent.Agent, accept func(uid uint32) bool) {
	defer conn.Close()

	// 1 + 2: read the 16-byte GUID and echo it back.
	guid := make([]byte, 16)
	if _, err := io.ReadFull(conn, guid); err != nil {
		return
	}
	if _, err := conn.Write(guid); err != nil {
		return
	}

	// 3 + 4: read the 12-byte pid/uid/gid and echo it back, unless the UID is
	// rejected, in which case the connection is dropped to fail the handshake.
	ids := make([]byte, 12)
	if _, err := io.ReadFull(conn, ids); err != nil {
		return
	}
	uid := binary.LittleEndian.Uint32(ids[4:8])
	if accept != nil && !accept(uid) {
		return
	}
	if _, err := conn.Write(ids); err != nil {
		return
	}

	agent.ServeAgent(keyring, conn)
}

func acceptAnyUID(uint32) bool { return true }

// requireLiveAgentConn asserts that conn speaks the SSH agent protocol and
// holds exactly the expected key.
func requireLiveAgentConn(t *testing.T, conn net.Conn, wantKey ssh.PublicKey) {
	t.Helper()
	keys, err := agent.NewClient(conn).List()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, wantKey.Marshal(), keys[0].Marshal())
}

// TestAttemptCygwinHandshake verifies the handshake wire protocol and that a
// successful handshake caches the UID that worked.
func TestAttemptCygwinHandshake(t *testing.T) {
	resetCygwinCache(t)
	keyring, key := newTestKeyring(t)
	_, port := startFakeCygwinAgent(t, keyring, acceptAnyUID)

	const uid = 1234
	conn, err := attemptCygwinHandshake(port, testCygwinGUID, uid)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	requireLiveAgentConn(t, conn, key)

	got, ok := loadCygwinUID()
	require.True(t, ok, "a successful handshake should cache the UID")
	require.Equal(t, uint32(uid), got)
}

// TestDialCygwinUsesCachedUID verifies that dialCygwin connects using a cached
// UID and skips resolution entirely. On Unix, resolution can never succeed, so a
// successful dial proves the cache was used.
func TestDialCygwinUsesCachedUID(t *testing.T) {
	resetCygwinCache(t)
	keyring, key := newTestKeyring(t)
	socketFile, _ := startFakeCygwinAgent(t, keyring, acceptAnyUID)

	// Without a cached UID, resolution runs and fails on Unix, so the dial fails.
	_, err := dialCygwin(socketFile)
	require.Error(t, err, "expected UID resolution to fail on a non-Cygwin host")

	// With a cached UID, dialCygwin skips resolution and connects.
	storeCygwinUID(4321, true)
	conn, err := dialCygwin(socketFile)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	requireLiveAgentConn(t, conn, key)
}

// TestDialCygwinEvictsStaleUID verifies that when a cached UID stops working,
// dialCygwin evicts it so the next dial does not retry the stale value.
func TestDialCygwinEvictsStaleUID(t *testing.T) {
	resetCygwinCache(t)
	keyring, _ := newTestKeyring(t)

	const staleUID = 9999
	socketFile, _ := startFakeCygwinAgent(t, keyring, func(uid uint32) bool {
		return uid != staleUID
	})

	storeCygwinUID(staleUID, true)

	// The cached UID is rejected, so the cached handshake fails and dialCygwin
	// falls back to resolution, which fails on Unix.
	_, err := dialCygwin(socketFile)
	require.Error(t, err)

	// The stale UID must have been evicted from the cache.
	_, ok := loadCygwinUID()
	require.False(t, ok, "a failing cached UID should be evicted")
}
