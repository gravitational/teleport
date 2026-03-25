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

package mysql

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/mysql/protocol"
)

const (
	// magicProxyReply is sent from proxy to the agent if, and only if, the proxy receives the agent's indication that it would like to handshake.
	// to indicate that the
	// proxy supports a full MySQL handshake and is waiting for the agent to
	// initiate it.
	// TODO(gavin): DELETE IN 21.0.0
	magicProxyReply = "ready-for-handshake"
	// emptyPassword is used as a fake credential because auth is already handled via mTLS.
	emptyPassword = ""
	// clientNameParamName defines the application name parameter name.
	// This is used to set the session context's user agent.
	//
	// https://dev.mysql.com/doc/refman/8.0/en/performance-schema-connection-attribute-tables.html
	clientNameParamName = "_client_name"
	// handshakeModeEnvVar overrides the default MySQL handshake mode for the
	// proxy and/or agent.
	// Only exact string enum values are supported, see [handshakeMode].
	handshakeModeEnvVar = "TELEPORT_MYSQL_DB_HANDSHAKE_MODE"
)

// handshakeMode determines the MySQL handshake behavior of the proxy or agent.
type handshakeMode string

// handshakeModeLogKey is the slog attribute key for handshake mode.
const handshakeModeLogKey = "handshake_mode"

const (
	// handshakeWhenSupported makes the agent or proxy perform a MySQL handshake
	// only when the other party supports a MySQL handshake. Handshake support
	// is determined via a magic "OK" packet value sent by the agent and a
	// corresponding magic proxy reply:
	// +-------------------+                +----------------------+
	// |  Teleport Proxy   |                |  Teleport DB Agent   |
	// +-------------------+                +----------------------+
	//            |                                    |
	//            |--- Connect (reverse tunnel) ------>|
	//            |                                    |
	//            |<----------- OK packet -------------|
	//            |                                    |
	//            |      (If "magic" OK packet)        |
	//         +------------------------------------------+
	//         |  |                                    |  |
	//         |  |--- "magic" reply ----------------->|  |
	//         |  |                                    |  |
	//         |  |<----- Init MySQL handshake --------|  |
	//         |                                          |
	//         +------------------------------------------+
	// In v20 we can be sure that the proxy will support MySQL handshakes, so
	// the agent can stop sending the OK packet to initiate a handshake and
	// instead initiate a partial handshake with the proxy, dial the real
	// database, and then complete the MySQL handshake with by sending OK.
	// Since the agent will no longer have to send the initial OK packet, we
	// will no longer have to worry about an older proxy telling the real client
	// that the connection is ready without the proxy having completed a
	// MySQL handshake with the agent first.
	// This will allow us to forward client capabilities without mangling
	// connection error propagation to the real client.
	handshakeWhenSupported handshakeMode = "when_supported"
	// handshakeAlways makes the agent or proxy unconditionally attempt a MySQL
	// handshake after the proxy dials the agent, without waiting for the magic
	// OK packet / proxy reply.
	handshakeAlways handshakeMode = "always"
	// handshakeDisabled disables MySQL handshake on the proxy or agent.
	handshakeDisabled handshakeMode = "disabled"
)

func getHandshakeMode() handshakeMode {
	switch handshakeMode(os.Getenv(handshakeModeEnvVar)) {
	case handshakeWhenSupported, "":
		// the default setting without env var override
		return handshakeWhenSupported
	case handshakeAlways:
		return handshakeAlways
	default:
		return handshakeDisabled
	}
}

// waitForAgent waits for the agent to indicate to the proxy that the connection
// is ready.
func waitForAgent(ctx context.Context, log *slog.Logger, mode handshakeMode, serviceConn net.Conn, handshakeOpts ...client.Option) error {
	log = log.With(handshakeModeLogKey, mode)
	if err := serviceConn.SetReadDeadline(time.Now().Add(2 * defaults.DatabaseConnectTimeout)); err != nil {
		return trace.Wrap(err)
	}
	defer serviceConn.SetReadDeadline(time.Time{})

	switch mode {
	case handshakeDisabled:
		log.DebugContext(ctx, "Waiting for OK packet from the database agent")
		_, err := waitForOKPacket(serviceConn)
		return trace.Wrap(err)
	case handshakeWhenSupported:
		bufConn := protocol.NewBufferedConn(ctx, serviceConn)
		serviceConn = bufConn
		isHandshakeV10, err := protocol.IsHandshakeV10Packet(bufConn)
		if err != nil {
			return trace.Wrap(err)
		}
		if isHandshakeV10 {
			log.DebugContext(ctx, "Received a handshake v10 packet")
		} else {
			log.DebugContext(ctx, "Waiting for magic OK packet from the database agent")
			pkt, err := waitForOKPacket(serviceConn)
			if err != nil {
				return trace.Wrap(err)
			}
			if !isMagicOKResult(pkt) {
				return nil
			}
			if err := sendMagicProxyReply(serviceConn); err != nil {
				return trace.Wrap(err)
			}
		}
	case handshakeAlways:
	}

	log.DebugContext(ctx, "Performing a MySQL handshake with the database agent")
	return trace.Wrap(handshakeWithAgent(ctx, serviceConn, handshakeOpts...))
}

// isMagicOKResult returns true if the OK packet received from the agent has
// non-zero affected rows.
// This method can be deleted after agents are updated to always initiate a
// MySQL handshake without negotiating one with the magic OK packet.
// TODO(gavin): DELETE IN 21.0.0
func isMagicOKResult(packet *protocol.OK) bool {
	return packet != nil && packet.HasAffectedRows()
}

func sendMagicProxyReply(conn net.Conn) error {
	if _, err := conn.Write([]byte{byte(len(magicProxyReply))}); err != nil {
		return trace.Wrap(err)
	}
	_, err := io.Copy(conn, strings.NewReader(magicProxyReply))
	return trace.Wrap(err)
}

func handshakeWithAgent(ctx context.Context, serviceConn net.Conn, options ...client.Option) error {
	_, err := client.ConnectWithDialer(ctx, "tcp", "stubAddr",
		"stubUser",
		emptyPassword,
		"stubDB",
		func(_ context.Context, _, _ string) (net.Conn, error) {
			return serviceConn, nil
		},
		options...,
	)
	return trace.Wrap(err)
}

// notifyProxy notifies the proxy that the connection is ready on the agent side.
func notifyProxy(ctx context.Context, log *slog.Logger, mode handshakeMode, proxyConn *server.Conn) error {
	log = log.With(handshakeModeLogKey, mode)
	// no matter the path we take, the client will expect the sequence number to
	// start with zero as we enter the command phase, and a mismatch could cause
	// sequence number errors
	defer proxyConn.ResetSequence()
	switch mode {
	case handshakeDisabled:
		// Send back OK packet to indicate auth/connect success. At this point
		// the original client should consider the connection phase completed.
		log.DebugContext(ctx, "Notifying the proxy of connection success with an empty OK packet")
		return trace.Wrap(proxyConn.WriteOK(nil))
	case handshakeWhenSupported:
		log.DebugContext(ctx, "Sending a magic OK packet to the proxy to request a MySQL handshake")
		if err := proxyConn.WriteOK(newMagicOKResult()); err != nil {
			return trace.Wrap(err)
		}
		proxyConn.ResetSequence()
		bufConn := protocol.NewBufferedConn(ctx, proxyConn.Conn.Conn)
		proxyConn.Conn.Conn = bufConn
		const magicReplyTimeout = 5 * time.Second
		log.DebugContext(ctx, "Waiting for magic reply from the proxy",
			"timeout", magicReplyTimeout,
		)
		found, err := waitForMagicProxyReply(bufConn, magicReplyTimeout)
		if err != nil {
			log.DebugContext(ctx, "The proxy did not reply to the request for a MySQL handshake, assuming that this proxy does not support MySQL handshakes",
				"error", err,
			)
			return nil
		}
		if !found {
			return nil
		}
	case handshakeAlways:
	}

	log.DebugContext(ctx, "Performing a MySQL handshake with the proxy")
	return trace.Wrap(handshakeWithProxy(proxyConn))
}

// newMagicOKResult is sent by the DB agent to tell the proxy to tell the proxy
// that the agent would like to perform a MySQL handshake.
// The proxy checks for the magic OK packet to determine if it should perform a
// full MySQL handshake to forward the real client's attributes and,
// eventually, capabilities.
// This ceremony is required for backwards compatibility, since we
// historically only had the proxy wait for an "ok" packet without doing a
// MySQL protocol handshake with the agent.
// Any non-zero value for the OK packet's affected rows will work.
// TODO(gavin): DELETE IN 20.0.0
func newMagicOKResult() *mysql.Result {
	return &mysql.Result{AffectedRows: 42}
}

// waitForMagicProxyReply peeks into the connection looking for the proxy reply,
// returning true and advancing its connection reader if, and only if, the proxy
// reply is found.
// If an older proxy doesn't send the reply, then the bytes we peek into
// will be from the real client. We could block indefinitely until the real
// client sends some commands, but that would delay the session start event.
// Normally, clients like libmysql will send some startup commands anyway, but
// by enforcing a timeout we can be sure the session start event will be emitted
// even if the client doesn't send any commands.
// This is out of an abundance of caution - I can't actually foresee a problem
// with delaying session start until the client sends some bytes, since the
// client will be unable to do anything with the session otherwise.
// TODO(gavin): DELETE IN 20.0.0
func waitForMagicProxyReply(conn protocol.BufferedConn, timeout time.Duration) (found bool, err error) {
	if timeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return false, trace.Wrap(err)
		}
		defer conn.SetReadDeadline(time.Time{})
	}
	defer func() {
		if err != nil && (errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, io.EOF)) {
			err = nil
		}
	}()
	payloadLen, err := conn.Peek(1)
	if err != nil {
		return false, trace.Wrap(err)
	}
	wantBytes := []byte(magicProxyReply)
	if int(payloadLen[0]) != len(wantBytes) {
		return false, nil
	}
	buf, err := conn.Peek(len(wantBytes) + 1)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if bytes.Equal(buf[1:], wantBytes) {
		_, _ = conn.Discard(len(buf))
		return true, nil
	}
	return false, nil
}

func handshakeWithProxy(proxyConn *server.Conn) error {
	if err := proxyConn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return trace.Wrap(err)
	}
	defer proxyConn.SetReadDeadline(time.Time{})
	if err := proxyConn.WriteInitialHandshake(); err != nil {
		return trace.Wrap(err)
	}
	if err := proxyConn.ReadHandshakeResponse(); err != nil {
		return trace.Wrap(err)
	}
	if err := proxyConn.WriteOK(nil); err != nil {
		return trace.Wrap(err)
	}
	proxyConn.ResetSequence()
	return nil
}
