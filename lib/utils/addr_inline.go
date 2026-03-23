package utils

import (
	"net"

	"github.com/gravitational/teleport/session/common/netutils"
)

//go:fix inline
type NetAddr = netutils.Addr

//go:fix inline
func ParseAddr(a string) (*netutils.Addr, error) { return netutils.ParseAddr(a) }

//go:fix inline
func MustParseAddr(a string) *netutils.Addr { return netutils.MustParseAddr(a) }

//go:fix inline
func FromAddr(a net.Addr) netutils.Addr { return netutils.FromAddr(a) }

//go:fix inline
func ParseHostPortAddr(hostport string, defaultPort int) (*netutils.Addr, error) {
	return netutils.ParseHostPortAddr(hostport, defaultPort)
}
