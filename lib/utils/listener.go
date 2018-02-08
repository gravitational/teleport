package utils

import (
	"net"
	"os"

	"github.com/gravitational/trace"
)

// GetListenerFile returns file associated with listener
func GetListenerFile(listener net.Listener) (*os.File, error) {
	switch t := listener.(type) {
	case *net.TCPListener:
		return t.File()
	case *net.UnixListener:
		return t.File()
	}
	return nil, trace.BadParameter("unsupported listener: %T", listener)
}
