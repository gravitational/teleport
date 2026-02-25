// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package reversetunnelv3

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/yamux"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/types"
	reversetunnelv1 "github.com/gravitational/teleport/gen/proto/go/teleport/reversetunnel/v1"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// yamuxTunnelALPN is the ALPN protocol name used to distinguish
// teleport-reversetunnel connections from other TLS traffic on the same
// listener. Connections with this negotiated protocol are dispatched to the
// reverse tunnel server rather than being treated as gRPC or HTTP/2.
const yamuxTunnelALPN = "teleport-reversetunnel"

// maxMessageSize is the maximum allowed size for a single framed protobuf
// message on the control or dial streams. 128 KiB is intentionally generous —
// dial messages are small, while ProxyHello carrying a large proxy gossip list
// could be somewhat larger. The limit exists to detect framing bugs early.
const maxMessageSize = 128 * 1024

// yamuxConfig returns the yamux session configuration used by both the agent
// and the proxy sides of the tunnel.
func yamuxConfig(log *slog.Logger) *yamux.Config {
	return &yamux.Config{
		AcceptBacklog: 128,

		EnableKeepAlive:        true,
		KeepAliveInterval:      30 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,

		MaxStreamWindowSize: 256 * 1024,

		StreamCloseTimeout: 5 * time.Minute,
		StreamOpenTimeout:  30 * time.Second,

		LogOutput: nil,
		Logger:    (*yamuxLogger)(log),
	}
}

// readProto reads a length-prefixed protobuf message from r. The message must
// fit within maxMessageSize or an error is returned.
func readProto(r io.Reader, m proto.Message) error {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(r, sizeBuf[:]); err != nil {
		return trace.Wrap(err)
	}
	size := binary.LittleEndian.Uint32(sizeBuf[:])
	if size > maxMessageSize {
		return trace.LimitExceeded("message size %d exceeds limit %d", size, maxMessageSize)
	}

	msgBuf := make([]byte, size)
	if _, err := io.ReadFull(r, msgBuf); err != nil {
		if errors.Is(err, io.EOF) {
			return trace.Wrap(io.ErrUnexpectedEOF)
		}
		return trace.Wrap(err)
	}

	if err := proto.Unmarshal(msgBuf, m); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// writeProto writes a length-prefixed protobuf message to w.
func writeProto(w io.Writer, m proto.Message) error {
	msgBuf, err := proto.Marshal(m)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(msgBuf) > maxMessageSize {
		return trace.LimitExceeded("message size %d exceeds limit %d", len(msgBuf), maxMessageSize)
	}
	var sizeBuf [4]byte
	binary.LittleEndian.PutUint32(sizeBuf[:], uint32(len(msgBuf)))
	if _, err := w.Write(sizeBuf[:]); err != nil {
		return trace.Wrap(err)
	}
	if _, err := w.Write(msgBuf); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// addrToProto converts a net.Addr into a protobuf Addr. Returns nil if a is nil.
func addrToProto(a net.Addr) *reversetunnelv1.Addr {
	if a == nil {
		return nil
	}
	return &reversetunnelv1.Addr{
		Network: a.Network(),
		Addr:    a.String(),
	}
}

// addrFromProto converts a protobuf Addr into a net.Addr. Returns nil if a is nil.
func addrFromProto(a *reversetunnelv1.Addr) net.Addr {
	if a == nil {
		return nil
	}
	return &utils.NetAddr{
		AddrNetwork: a.GetNetwork(),
		Addr:        a.GetAddr(),
	}
}

// tcpAddrFromProto is like addrFromProto but additionally converts TCP addresses
// to *net.TCPAddr, which is required by some internal components (e.g.
// x/crypto/ssh and connection resumption) that type-assert to *net.TCPAddr
// rather than treating addresses as generic net.Addrs.
func tcpAddrFromProto(a *reversetunnelv1.Addr) net.Addr {
	addr := addrFromProto(a)
	if addr == nil {
		return nil
	}
	if addr.Network() == "tcp" {
		ap, err := netip.ParseAddrPort(addr.String())
		if err == nil {
			ap = netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
			return net.TCPAddrFromAddrPort(ap)
		}
	}
	return addr
}

// systemRoleForTunnelType returns the SystemRole that must appear in an agent's
// Instance certificate for it to be authorised to register the given
// TunnelType. Returns an empty string and false if the TunnelType is unknown.
func systemRoleForTunnelType(t types.TunnelType) (types.SystemRole, bool) {
	switch t {
	case types.NodeTunnel:
		return types.RoleNode, true
	case types.AppTunnel:
		return types.RoleApp, true
	case types.KubeTunnel:
		return types.RoleKube, true
	case types.DatabaseTunnel:
		return types.RoleDatabase, true
	case types.WindowsDesktopTunnel:
		return types.RoleWindowsDesktop, true
	case types.OktaTunnel:
		return types.RoleOkta, true
	default:
		return "", false
	}
}

// validateServices checks that every service in services has a corresponding
// SystemRole present in id, and that the TunnelType string is a recognised
// value. Returns the first validation error encountered.
//
// Instance certs carry all authorised roles in SystemRoles; older single-role
// certs encode the role in Groups. Both are checked for backwards
// compatibility.
func validateServices(id *tlsca.Identity, services []types.TunnelType) error {
	if len(services) == 0 {
		return trace.BadParameter("agent must advertise at least one service")
	}
	for _, svc := range services {
		role, ok := systemRoleForTunnelType(svc)
		if !ok {
			return trace.BadParameter("unsupported service type %q", svc)
		}
		if !slices.Contains(id.SystemRoles, string(role)) && !slices.Contains(id.Groups, string(role)) {
			return trace.AccessDenied("service type %q requires role %q, which is not present in the client certificate", svc, role)
		}
	}
	return nil
}

// yamuxLogger adapts *slog.Logger to the yamux.Logger interface by parsing the
// log-level prefix that yamux prepends to its messages.
type yamuxLogger slog.Logger

var _ yamux.Logger = (*yamuxLogger)(nil)

// Printf implements [yamux.Logger].
func (l *yamuxLogger) Printf(format string, args ...any) {
	if f, ok := strings.CutPrefix(format, "[ERR] "); ok {
		//nolint:sloglint // adapting a fmt.Printf-like interface
		(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintf(f, args...))
	} else if f, ok := strings.CutPrefix(format, "[WARN] "); ok {
		//nolint:sloglint // adapting a fmt.Printf-like interface
		(*slog.Logger)(l).WarnContext(context.Background(), fmt.Sprintf(f, args...))
	} else {
		//nolint:sloglint // adapting a fmt.Printf-like interface
		(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintf(format, args...))
	}
}

// Print implements [yamux.Logger].
func (l *yamuxLogger) Print(args ...any) {
	//nolint:sloglint // adapting a fmt.Print-like interface
	(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprint(args...))
}

// Println implements [yamux.Logger].
func (l *yamuxLogger) Println(args ...any) {
	l.Print(args...)
}

// yamuxStreamConn wraps a *yamux.Stream as a net.Conn with explicit local and
// remote addresses, and converts yamux.ErrTimeout to os.ErrDeadlineExceeded so
// that TLS and HTTP/2 layers handle deadline expiry correctly (yamux.ErrTimeout
// has Temporary() == false, which causes *tls.Conn to kill the connection
// rather than returning a timeout error to the caller).
//
// See https://github.com/hashicorp/yamux/issues/156.
type yamuxStreamConn struct {
	*yamux.Stream
	localAddr  net.Addr
	remoteAddr net.Addr
}

// Read implements [net.Conn].
func (c *yamuxStreamConn) Read(b []byte) (int, error) {
	n, err := c.Stream.Read(b)
	//nolint:errorlint // workaround for yamux exact-error-value behaviour
	if err == yamux.ErrTimeout {
		err = os.ErrDeadlineExceeded
	}
	return n, err
}

// Write implements [net.Conn].
func (c *yamuxStreamConn) Write(b []byte) (int, error) {
	n, err := c.Stream.Write(b)
	//nolint:errorlint // workaround for yamux exact-error-value behaviour
	if err == yamux.ErrTimeout {
		err = os.ErrDeadlineExceeded
	}
	return n, err
}

// LocalAddr implements [net.Conn].
func (c *yamuxStreamConn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr implements [net.Conn].
func (c *yamuxStreamConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}
