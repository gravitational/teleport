//go:build !(desktop_access_rdp && desktop_encoder)

package rdpclient

import (
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/trace"
)

func EncodeQOIZ(frame []byte, x, y, width, height uint16) ([]*tdpb.FastPathPDU, error) {
	return nil, trace.NotImplemented("qoiz encoding not built into this binary")
}

func EncodeQOIZAvailable() bool { return false }
