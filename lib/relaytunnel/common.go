package relaytunnel

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

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

type yamuxLogger slog.Logger

var _ yamux.Logger = (*yamuxLogger)(nil)

// Print implements [yamux.Logger].
func (l *yamuxLogger) Print(v ...any) {
	(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintln(v...))
}

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

// Println implements [yamux.Logger].
func (l *yamuxLogger) Println(args ...any) {
	(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintln(args...))
}
