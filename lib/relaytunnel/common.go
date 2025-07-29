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

package relaytunnel

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/hashicorp/yamux"
	"google.golang.org/protobuf/proto"

	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

const yamuxTunnelALPN = "teleport-relaytunnel"

func readProto(r io.Reader, m proto.Message) error {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(r, sizeBuf[:]); err != nil {
		return trace.Wrap(err)
	}
	size := binary.LittleEndian.Uint32(sizeBuf[:])
	if size > maxMessageSize {
		return trace.LimitExceeded("bad size")
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

func writeProto(w io.Writer, m proto.Message) error {
	msgBuf, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	if len(msgBuf) > maxMessageSize {
		return trace.LimitExceeded("bad size")
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

func addrToProto(a net.Addr) *relaytunnelv1alpha.Addr {
	if a == nil {
		return nil
	}

	return &relaytunnelv1alpha.Addr{
		Network: a.Network(),
		Addr:    a.String(),
	}
}

func addrFromProto(a *relaytunnelv1alpha.Addr) net.Addr {
	if a == nil {
		return nil
	}

	return &utils.NetAddr{
		AddrNetwork: a.GetNetwork(),
		Addr:        a.GetAddr(),
	}
}

type RelayInfoHolder struct {
	mu sync.Mutex

	relayGroup string
	relayIDs   []string
}

func (r *RelayInfoHolder) GetRelayInfo() (relayGroup string, relayIDs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.relayGroup, slices.Clone(r.relayIDs)
}

func (r *RelayInfoHolder) SetRelayInfo(relayGroup string, relayIDs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.relayGroup = relayGroup
	r.relayIDs = relayIDs
}

type GetRelayInfoFunc func() (relayGroup string, relayIDs []string)
type SetRelayInfoFunc func(relayGroup string, relayIDs []string)

var _ GetRelayInfoFunc = (*RelayInfoHolder)(nil).GetRelayInfo
var _ SetRelayInfoFunc = (*RelayInfoHolder)(nil).SetRelayInfo

type yamuxLogger slog.Logger

var _ yamux.Logger = (*yamuxLogger)(nil)

// Printf implements [yamux.Logger].
func (l *yamuxLogger) Printf(format string, args ...any) {
	if f, ok := strings.CutPrefix(format, "[ERR] "); ok {
		(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintf(f, args...))
	} else if f, ok := strings.CutPrefix(format, "[WARN] "); ok {
		(*slog.Logger)(l).WarnContext(context.Background(), fmt.Sprintf(f, args...))
	} else {
		(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintf(format, args...))
	}
}

// Print implements [yamux.Logger].
func (l *yamuxLogger) Print(args ...any) {
	(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprint(args...))
}

// Println implements [yamux.Logger].
func (l *yamuxLogger) Println(args ...any) {
	(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintln(args...))
}
