// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package resumption

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	serverProtocolStringV1 = sshutils.SSHVersionPrefix + " resume-v1" // "SSH-2.0-Teleport resume-v1"
	clientProtocolStringV1 = "teleport-resume-v1"

	sshPrefix       = "SSH-2.0-"
	clientSuffixV1  = "\x00" + clientProtocolStringV1 // "\x00teleport-resume-v1"
	clientPreludeV1 = sshPrefix + clientSuffixV1      // "SSH-2.0-\x00teleport-resume-v1"

	detachedTimeout = time.Minute
)

// Component is the logging "component" for connection resumption.
const Component = "resumable"

const ecdhP256UncompressedSize = 65

func serverVersionCRLFV1(pubKey *ecdh.PublicKey, hostID string) string {
	// "SSH-2.0-Teleport resume-v1 base64PubKey hostID\r\n"
	return fmt.Sprintf(serverProtocolStringV1+" %v %v\r\n",
		base64.RawStdEncoding.EncodeToString(pubKey.Bytes()),
		hostID,
	)
}

type SSHServerWrapperConfig struct {
	// SSHServer is a function that takes ownership of a [net.Conn] and uses it
	// as a SSH server. If the Conn is a [sshutils.SSHServerVersionOverrider],
	// the server should use the overridden server version.
	SSHServer func(net.Conn)

	SSHStapledServer func(net.Conn, []byte)

	// HostID is the host ID of the Teleport instance running the server;
	// compliant connection resumption clients will reconnect to the host ID
	// expecting to reach the instance.
	HostID string

	// DataDir is the path to the Teleport data directory, used to store
	// temporary handover sockets.
	DataDir string
}

// NewSSHServerWrapper creates a [SSHServerWrapper].
func NewSSHServerWrapper(cfg SSHServerWrapperConfig) *SSHServerWrapper {
	return &SSHServerWrapper{
		sshServer:        cfg.SSHServer,
		sshStapledServer: cfg.SSHStapledServer,
		hostID:           cfg.HostID,
		dataDir:          cfg.DataDir,

		conns: make(map[resumptionToken]*connEntry),
	}
}

type resumptionToken = [16]byte

// SSHServerWrapper wraps a SSH server, keeping track of which resumption v1
// connections can be resumed by the client. Connections that stay without an
// active underlying connection for a given time ([detachedTimeout]) are
// forcibly closed.
type SSHServerWrapper struct {
	sshServer        func(net.Conn)
	sshStapledServer func(net.Conn, []byte)

	hostID  string
	dataDir string

	mu    sync.Mutex
	conns map[resumptionToken]*connEntry
}

func (r *SSHServerWrapper) runConn(nc net.Conn, maybePermit *[]byte) {
	if r.sshStapledServer != nil && maybePermit != nil {
		r.sshStapledServer(nc, *maybePermit)
		return
	}
	r.sshServer(nc)
}

type connEntry struct {
	conn     *Conn
	remoteIP netip.Addr

	mu      sync.Mutex
	timeout *time.Timer
	running uint
}

func (e *connEntry) increaseRunning() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.timeout.Stop()
	e.running++
}

func (e *connEntry) decreaseRunning() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running--
	if e.running == 0 {
		e.timeout.Reset(detachedTimeout)
	}
}

// PreDetect is intended to be used in a [multiplexer.Mux] as the PreDetect
// hook; it generates the handshake ECDH key and sends it as the SSH server
// version identifier, then returns a post-detect hook to check if the client
// supports resumption and to hijack its connection if that's the case.
func (r *SSHServerWrapper) PreDetect(nc net.Conn) (multiplexer.PostDetectFunc, error) {
	dhKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		slog.ErrorContext(context.TODO(), "failed to generate ECDH key, proceeding without resumption (this is a bug)", "error", err)
		// we are still responsible for sending a RFC 4253-compliant
		// identification string as the PreDetect hook
		return PreDetectFixedSSHVersion(sshutils.SSHVersionPrefix)(nc)
	}

	serverVersionCRLF := serverVersionCRLFV1(dhKey.PublicKey(), r.hostID)
	if _, err := nc.Write([]byte(serverVersionCRLF)); err != nil {
		return nil, trace.Wrap(err)
	}

	return func(conn *multiplexer.Conn) net.Conn {
		isResumeV1, err := peekPrelude(conn, clientPreludeV1)
		if err != nil {
			if !utils.IsOKNetworkError(err) {
				slog.ErrorContext(context.TODO(), "error while peeking resumption prelude", "error", err)
			}
			_ = conn.Close()
			return nil
		}

		if !isResumeV1 {
			slog.DebugContext(context.TODO(), "returning non-resumable connection to multiplexer")
			return &sshVersionSkipConn{
				Conn: conn,

				serverVersion:  serverVersionCRLF[:len(serverVersionCRLF)-2],
				alreadyWritten: serverVersionCRLF,
			}
		}

		// we successfully peeked clientPrelude, so Discard will succeed
		_, _ = conn.Discard(len(clientPreludeV1))

		slog.DebugContext(context.TODO(), "proceeding with connection resumption exchange")
		// this is the post detect hook in the multiplexer, we return nil here
		// to signify that the connection has been hijacked
		var maybePermitNil *[]byte
		r.handleResumptionExchangeV1(conn, maybePermitNil, dhKey)
		return nil
	}, nil
}

var _ multiplexer.PreDetectFunc = (*SSHServerWrapper)(nil).PreDetect

// HandleConnection generates the handshake ECDH key and sends it as the SSH
// server version identifier, then checks if the client supports resumption,
// running the connection as a resumable connection if that's the case, or
// handing the connection to the underlying SSH server otherwise.
func (r *SSHServerWrapper) HandleConnection(nc net.Conn) {
	var maybePermitNil *[]byte
	r.handleConnection(nc, maybePermitNil)
}

func (r *SSHServerWrapper) HandleStapledConnection(nc net.Conn, permit []byte) {
	r.handleConnection(nc, &permit)
}

func (r *SSHServerWrapper) handleConnection(nc net.Conn, maybePermit *[]byte) {
	dhKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		slog.ErrorContext(context.TODO(), "failed to generate ECDH key, proceeding without resumption (this is a bug)", "error", err)
		r.runConn(nc, maybePermit)
		return
	}

	serverVersionCRLF := serverVersionCRLFV1(dhKey.PublicKey(), r.hostID)
	if _, err := nc.Write([]byte(serverVersionCRLF)); err != nil {
		if !utils.IsOKNetworkError(err) {
			slog.ErrorContext(context.TODO(), "error while sending SSH identification string", "error", err)
		}
		_ = nc.Close()
		return
	}

	conn := ensureMultiplexerConn(nc)

	isResumeV1, err := peekPrelude(conn, clientPreludeV1)
	if err != nil {
		if !utils.IsOKNetworkError(err) {
			slog.ErrorContext(context.TODO(), "error while peeking resumption prelude", "error", err)
		}
		_ = conn.Close()
		return
	}

	if !isResumeV1 {
		slog.DebugContext(context.TODO(), "returning non-resumable connection to multiplexer")
		r.runConn(&sshVersionSkipConn{
			Conn: conn,

			serverVersion:  serverVersionCRLF[:len(serverVersionCRLF)-2],
			alreadyWritten: serverVersionCRLF,
		}, maybePermit)
		return
	}

	// we successfully peeked clientPrelude, so Discard will succeed
	_, _ = conn.Discard(len(clientPreludeV1))

	slog.DebugContext(context.TODO(), "proceeding with connection resumption exchange")
	r.handleResumptionExchangeV1(conn, maybePermit, dhKey)
}
