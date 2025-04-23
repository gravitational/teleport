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

/*
# QUIC proxy peering

QUIC proxy peering uses QUIC connections to the same address and port as the
regular proxy peering listener (port 3021, by default) and use the same routing
logic as the existing proxy peering.

Until the feature is stabilized, a proxy can advertise support for receiving
QUIC proxy peering connections through a label in its heartbeat. In the current
implementation a proxy will only use outbound QUIC connections if it's also
accepting QUIC connections, and all connections will use the same socket; this
has the effect of taking up half the conntrack entries than TCP proxy peering
(as each outbound TCP connection to a given destination needs use a different
ephemeral port).

## Protocol

The server will accept connections from any Proxy of the cluster; in the
connection, the client can open a bidirectional stream for each dial attempt
through the server. The connections use ALPN with a protocol name that contains
the "version" of the protocol (currently v1alpha, matching the protobuf package
version).

The client opens each stream by sending the protobuf binary encoding of a
DialRequest message (see proto/teleport/quicpeering/v1alpha/dial.proto),
length-prefixed by a little endian 32 bit integer. The server will check that
the request is valid (see the "0-RTT considerations" section), attempt to dial
the agent through a reverse tunnel, then report back an error in a DialResponse
message that contains a google.rpc.Status (by taking the error and passing it
through trace/trail, which is conveniently how we transfer errors on the wire
with gRPC); the response is, like the request, encoded in binary protobuf format
and length-prefixed.

If the status is ok (signifying no error) then the stream will stay open,
carrying the data for the connection between the user and the agent, otherwise
the stream will be closed. For sanity's sake, the size of both messages is
limited and any oversized message is treated as a protocol violation.

## Multiplexing (or lack thereof)

While the current server implementation poses no limits to the amount of streams
in a single connection, real-world tests have shown that the best performance in
terms of throughput and isolation between different user connections is achieved
with individual QUIC connections between proxies. This would be very impractical
with TCP, as that would result in significantly heavier load on the network
infrastructure from having to keep track of all the individual TCP connections,
but QUIC can use its own internal connection IDs as it sends and receives UDP
packets over the same (source address, source port, destination address,
destination port) 4-ple.

As such, the current client implementation opens a new QUIC connection for each
user connection, with a single stream in it. This could be changed without
breaking compatibility in the future.

## 0-RTT considerations

QUIC can make use of TLS session resumption, to make TLS handshakes
computationally cheaper. When resuming a session, the client can send data in
the very first packets, without waiting for any data from the server. This
"0-RTT" data is authentic and protected against eavesdropping, and any response
from the server would likewise be protected against eavesdropping, but an
attacker with the ability to sniff and inject data can blindly replay the
initial resumed handshake including the 0-RTT data.

Since proxy peering is at its most useful when dealing with connections across
regions, it's very advantageous to take advantage of the latency reduction
offered by 0-RTT; to prevent any problems caused by replay attacks, the client
must include a timestamp and a nonce in the request: the server will require a
full handshake if the timestamp is too far off the local time (to prevent
complete cluster outages in case of clock drift), and then will check the nonce
against a list of nonces received recently. If the nonce was already used, the
dial request is rejected (potentially rejecting the legitimate dial attempt if
it happens to be processed after a replay, which will increase the chance that
someone will notice that something is wrong), otherwise the nonce is added to
the list and the dial request is accepted. This makes it so that each dial
request sent as 0-RTT can result in at most one connection opened through an
agent tunnel.

The client should make sure to not send data belonging to the connection as part
of the early data, as an additional layer against replay attacks; this will
cause no further delays if the client intends to wait for the server to reply to
the dial request. A client that wants to make use of multiplexing should take
care to not accidentally send more than one dial request as 0-RTT in a single
connection, to keep the effort needed to handle potential replays at a minimum.

The protocol doesn't currently take advantage of early server-side data for
non-resumed connections, so considerations around the security of "0.5-RTT" data
are not relevant; data sent by the server as a response to the client is either
using 0-RTT keys or is sent after the handshake is completed.
*/
package quic

import (
	"encoding/binary"
	"errors"
	"math"
	"time"

	"github.com/gravitational/trace"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/quicvarint"
	"google.golang.org/protobuf/proto"
)

const (
	// maxIdleTimeout is the arbitrary timeout after which a QUIC connection
	// that hasn't received data is presumed to be lost to the aether.
	maxIdleTimeout = 30 * time.Second
	// keepAlivePeriod is the interval of QUIC keepalive packets sent if the
	// connection is otherwise idle.
	keepAlivePeriod = 5 * time.Second

	maxReceiveWindow   = quicvarint.Max
	maxIncomingStreams = 1 << 60 // maximum allowed value as per the quic-go docs

	// nextProto is the ALPN indicator for the current version of the QUIC proxy
	// peering protocol.
	nextProto = "teleport-peer-v1alpha"

	// maxMessageSize is the maximum accepted size (in protobuf binary format)
	// for the request and response messages exchanged as part of the dialing.
	maxMessageSize = 128 * 1024

	// timestampGraceWindow is the maximum time difference between local time
	// and reported time in a 0-RTT request. Clients should not keep trying to
	// use a request after this much time has passed.
	timestampGraceWindow = time.Minute
	// noncePersistence is the shortest time for which a nonce will be kept in
	// memory to prevent 0-RTT replay attacks. Should be significantly longer
	// than [quicTimestampGraceWindow]. In the current implementation, nonces
	// can be kept for at most twice this value.
	noncePersistence = 5 * time.Minute

	dialTimeout          = 30 * time.Second
	requestTimeout       = 10 * time.Second
	errorResponseTimeout = 10 * time.Second

	// noStreamErrorCode indicates that the stream was closed with no errors.
	noStreamErrorCode quic.StreamErrorCode = 0
	// genericStreamErrorCode indicates that the stream was closed with a
	// non-specific error.
	genericStreamErrorCode quic.StreamErrorCode = 1

	// noApplicationErrorCode indicates that the connection was closed with no
	// errors.
	noApplicationErrorCode quic.ApplicationErrorCode = 0
	// genericApplicationErrorCode indicates that the connection was closed with
	// a non-specific error.
	genericApplicationErrorCode quic.ApplicationErrorCode = 1
)

// marshalSized returns the wire encoding of the given [proto.Message] prefixed
// by its length encoded as a little endian 32-bit integer.
func marshalSized(m proto.Message) ([]byte, error) {
	var size int = proto.MarshalOptions{}.Size(m)
	// we're going to allocate 4+size, so size can't exceed MaxInt-4
	// (overflowing would be confusing at best, a runtime panic at worst)
	if size > math.MaxInt-4 {
		return nil, trace.LimitExceeded("oversized message")
	}
	// we're going to store the size in the first four bytes of the buffer, so
	// we leave them as zeroes at first, then fix it later
	buf := make([]byte, 4, 4+size)
	buf, err := proto.MarshalOptions{UseCachedSize: true}.MarshalAppend(buf, m)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// the int64 cast is a noop on 64-bit archs but it's necessary to build on
	// 32-bit (even though the condition is always false there, since slices
	// can't be bigger than MaxInt32 which is smaller than MaxUint32)
	if int64(len(buf))-4 > math.MaxUint32 {
		return nil, trace.LimitExceeded("oversized message")
	}
	binary.LittleEndian.PutUint32(buf, uint32(len(buf)-4))
	return buf, nil
}

// ignoreCodeZero returns nil if the given error is a [quic.StreamError] or
// [quic.ApplicationError] with an error code of 0 - otherwise, the error is
// returned as is.
func ignoreCodeZero(err error) error {
	if streamErr := (*quic.StreamError)(nil); errors.As(err, &streamErr) && streamErr.ErrorCode == noStreamErrorCode {
		return nil
	}
	if appErr := (*quic.ApplicationError)(nil); errors.As(err, &appErr) && appErr.ErrorCode == noApplicationErrorCode {
		return nil
	}
	return err
}

// ignoreCodeZero2 is a helper for [ignoreCodeZero] that also passes along a
// value unaltered.
func ignoreCodeZero2[T any](val T, err error) (T, error) {
	err = ignoreCodeZero(err)
	return val, err
}
