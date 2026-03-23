package utils

import (
	"net"

	"github.com/gravitational/teleport/session/common/netutils"
)

//go:fix inline
type NetAddr = netutils.NetAddr

//go:fix inline
func NetAddrsToStrings(netAddrs []netutils.NetAddr) []string {
	return netutils.NetAddrsToStrings(netAddrs)
}

//go:fix inline
func ParseAddrs(addrs []string) (result []netutils.NetAddr, err error) {
	return netutils.ParseAddrs(addrs)
}

//go:fix inline
func ParseAddr(a string) (*netutils.NetAddr, error) { return netutils.ParseAddr(a) }

//go:fix inline
func MustParseAddr(a string) *netutils.NetAddr { return netutils.MustParseAddr(a) }

//go:fix inline
func MustParseAddrList(aList ...string) []netutils.NetAddr {
	return netutils.MustParseAddrList(aList...)
}

//go:fix inline
func FromAddr(a net.Addr) netutils.NetAddr { return netutils.FromAddr(a) }

//go:fix inline
func JoinAddrSlices(a []netutils.NetAddr, b []netutils.NetAddr) []netutils.NetAddr {
	return netutils.JoinAddrSlices(a, b)
}

//go:fix inline
func ParseHostPortAddr(hostport string, defaultPort int) (*netutils.NetAddr, error) {
	return netutils.ParseHostPortAddr(hostport, defaultPort)
}

//go:fix inline
func DialAddrFromListenAddr(listenAddr netutils.NetAddr) netutils.NetAddr {
	return netutils.DialAddrFromListenAddr(listenAddr)
}

//go:fix inline
func ReplaceLocalhost(addr, replaceWith string) string {
	return netutils.ReplaceLocalhost(addr, replaceWith)
}

//go:fix inline
func IsLocalhost(host string) bool { return netutils.IsLocalhost(host) }

//go:fix inline
func GuessHostIP() (ip net.IP, err error) { return netutils.GuessHostIP() }

//go:fix inline
func ReplaceUnspecifiedHost(addr *netutils.NetAddr, defaultPort int) string {
	return netutils.ReplaceUnspecifiedHost(addr, defaultPort)
}

//go:fix inline
func ToLowerCaseASCII(in string) string { return netutils.ToLowerCaseASCII(in) }
