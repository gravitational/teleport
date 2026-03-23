package utils

import (
	"net"

	"github.com/gravitational/teleport/session/common/netutils"
)

//go:fix inline
type NetAddr = netutils.NetAddr

//go:fix inline
func ParseAddr(a string) (*netutils.NetAddr, error) { return netutils.ParseAddr(a) }

//go:fix inline
func MustParseAddr(a string) *netutils.NetAddr { return netutils.MustParseAddr(a) }

//go:fix inline
func FromAddr(a net.Addr) netutils.NetAddr { return netutils.FromAddr(a) }

//go:fix inline
func ParseHostPortAddr(hostport string, defaultPort int) (*netutils.NetAddr, error) {
	return netutils.ParseHostPortAddr(hostport, defaultPort)
}
