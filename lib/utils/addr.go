/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gravitational/trace"

	apiutils "github.com/gravitational/teleport/api/utils"
)

// NetAddr is network address that includes network, optional path and
// host port
type NetAddr struct {
	// Addr is the host:port address, like "localhost:22"
	Addr string `json:"addr"`
	// AddrNetwork is the type of a network socket, like "tcp" or "unix"
	AddrNetwork string `json:"network,omitempty"`
	// Path is a socket file path, like '/var/path/to/socket' in "unix:///var/path/to/socket"
	Path string `json:"path,omitempty"`
}

// Host returns host part of address without port
func (a *NetAddr) Host() string {
	host, _, err := net.SplitHostPort(a.Addr)
	if err == nil {
		return host
	}
	// this is done to remove optional square brackets
	if ip := net.ParseIP(strings.Trim(a.Addr, "[]")); len(ip) != 0 {
		return ip.String()
	}
	return a.Addr
}

// Port returns defaultPort if no port is set or is invalid,
// the real port otherwise
func (a *NetAddr) Port(defaultPort int) int {
	_, port, err := net.SplitHostPort(a.Addr)
	if err != nil {
		return defaultPort
	}
	porti, err := strconv.Atoi(port)
	if err != nil {
		return defaultPort
	}
	return porti
}

// IsLocal returns true if this is a local address
func (a *NetAddr) IsLocal() bool {
	host, _, err := net.SplitHostPort(a.Addr)
	if err != nil {
		return false
	}
	return IsLocalhost(host)
}

// IsLoopback returns true if this is a loopback address
func (a *NetAddr) IsLoopback() bool {
	return apiutils.IsLoopback(a.Addr)
}

// IsHostUnspecified returns true if this address' host is unspecified.
func (a *NetAddr) IsHostUnspecified() bool {
	return a.Host() == "" || net.ParseIP(a.Host()).IsUnspecified()
}

// IsEmpty returns true if address is empty
func (a *NetAddr) IsEmpty() bool {
	return a == nil || (a.Addr == "" && a.AddrNetwork == "" && a.Path == "")
}

// FullAddress returns full address including network and address (tcp://0.0.0.0:1243)
func (a *NetAddr) FullAddress() string {
	return fmt.Sprintf("%v://%v", a.AddrNetwork, a.Addr)
}

// String returns address without network (0.0.0.0:1234)
func (a *NetAddr) String() string {
	return a.Addr
}

// Network returns the scheme for this network address (tcp or unix)
func (a *NetAddr) Network() string {
	return a.AddrNetwork
}

// MarshalYAML defines how a network address should be marshaled to a string
func (a *NetAddr) MarshalYAML() (any, error) {
	url := url.URL{Scheme: a.AddrNetwork, Host: a.Addr, Path: a.Path}
	return strings.TrimLeft(url.String(), "/"), nil
}

// UnmarshalYAML defines how a string can be unmarshalled into a network address
func (a *NetAddr) UnmarshalYAML(unmarshal func(any) error) error {
	var addr string
	err := unmarshal(&addr)
	if err != nil {
		return err
	}

	parsedAddr, err := ParseAddr(addr)
	if err != nil {
		return err
	}

	*a = *parsedAddr
	return nil
}

func (a *NetAddr) Set(s string) error {
	v, err := ParseAddr(s)
	if err != nil {
		return trace.Wrap(err)
	}
	a.Addr = v.Addr
	a.AddrNetwork = v.AddrNetwork
	return nil
}

// NetAddrsToStrings takes a list of netAddrs and returns a list of address strings.
func NetAddrsToStrings(netAddrs []NetAddr) []string {
	addrs := make([]string, len(netAddrs))
	for i, addr := range netAddrs {
		addrs[i] = addr.String()
	}
	return addrs
}

// ParseAddrs parses the provided slice of strings as a slice of NetAddr's.
func ParseAddrs(addrs []string) (result []NetAddr, err error) {
	for _, addr := range addrs {
		parsed, err := ParseAddr(addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, *parsed)
	}
	return result, nil
}

// ParseAddr takes strings like "tcp://host:port/path" and returns
// *NetAddr or an error
func ParseAddr(a string) (*NetAddr, error) {
	if a == "" {
		return nil, trace.BadParameter("missing parameter address")
	}
	if !strings.Contains(a, "://") {
		a = "tcp://" + a
	}
	u, err := url.Parse(a)
	if err != nil {
		return nil, trace.BadParameter("failed to parse %q: %v", a, err)
	}
	switch u.Scheme {
	case "tcp":
		return &NetAddr{Addr: u.Host, AddrNetwork: u.Scheme, Path: u.Path}, nil
	case "unix":
		return &NetAddr{Addr: u.Path, AddrNetwork: u.Scheme}, nil
	case "http", "https":
		return &NetAddr{Addr: u.Host, AddrNetwork: u.Scheme, Path: u.Path}, nil
	default:
		return nil, trace.BadParameter("%q: unsupported scheme: %q", a, u.Scheme)
	}
}

// MustParseAddr parses the provided string into NetAddr or panics on an error
func MustParseAddr(a string) *NetAddr {
	addr, err := ParseAddr(a)
	if err != nil {
		panic(fmt.Sprintf("failed to parse %v: %v", a, err))
	}
	return addr
}

// MustParseAddrList parses the provided list of strings into a NetAddr list or panics on error
func MustParseAddrList(aList ...string) []NetAddr {
	addrList := make([]NetAddr, len(aList))
	for i, a := range aList {
		addrList[i] = *MustParseAddr(a)
	}
	return addrList
}

// FromAddr returns NetAddr from golang standard net.Addr
func FromAddr(a net.Addr) NetAddr {
	return NetAddr{AddrNetwork: a.Network(), Addr: a.String()}
}

// JoinAddrSlices joins two addr slices and returns a resulting slice
func JoinAddrSlices(a []NetAddr, b []NetAddr) []NetAddr {
	if len(a)+len(b) == 0 {
		return nil
	}
	out := make([]NetAddr, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}

// ParseHostPortAddr takes strings like "host:port" and returns
// *NetAddr or an error
//
// If defaultPort == -1 it expects 'hostport' string to have it
func ParseHostPortAddr(hostport string, defaultPort int) (*NetAddr, error) {
	addr, err := ParseAddr(hostport)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// port is required but not set
	if defaultPort == -1 && addr.Addr == addr.Host() {
		return nil, trace.BadParameter("missing port in address %q", hostport)
	}
	addr.Addr = net.JoinHostPort(addr.Host(), fmt.Sprintf("%v", addr.Port(defaultPort)))
	return addr, nil
}

// DialAddrFromListenAddr returns dial address from listen address
func DialAddrFromListenAddr(listenAddr NetAddr) NetAddr {
	if listenAddr.IsEmpty() {
		return listenAddr
	}
	return NetAddr{Addr: ReplaceLocalhost(listenAddr.Addr, "127.0.0.1")}
}

// ReplaceLocalhost checks if a given address is link-local (like 0.0.0.0 or 127.0.0.1)
// and replaces it with the IP taken from replaceWith, preserving the original port
//
// Both addresses are in "host:port" format
// The function returns the original value if it encounters any problems with parsing
func ReplaceLocalhost(addr, replaceWith string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if IsLocalhost(host) {
		host, _, err = net.SplitHostPort(replaceWith)
		if err != nil {
			return addr
		}
		addr = net.JoinHostPort(host, port)
	}
	return addr
}

// IsLocalhost returns true if this is a local hostname or ip
func IsLocalhost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip.IsLoopback() || ip.IsUnspecified()
}

// GuessIP tries to guess an IP address this machine is reachable at on the
// internal network, always picking IPv4 from the internal address space
//
// If no internal IPs are found, it returns 127.0.0.1 but it never returns
// an address from the public IP space
func GuessHostIP() (ip net.IP, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	adrs := make([]net.Addr, 0)
	for _, iface := range ifaces {
		ifadrs, err := iface.Addrs()
		if err != nil {
			slog.WarnContext(context.Background(), "Unable to get addresses for interface", "interface", iface.Name, "error", err)
		} else {
			adrs = append(adrs, ifadrs...)
		}
	}
	return guessHostIP(adrs), nil
}

func guessHostIP(addrs []net.Addr) (ip net.IP) {
	// collect the list of all IPv4s
	var ips []net.IP
	for _, addr := range addrs {
		var ipAddr net.IP
		a, ok := addr.(*net.IPAddr)
		if ok {
			ipAddr = a.IP
		} else {
			in, ok := addr.(*net.IPNet)
			if ok {
				ipAddr = in.IP
			} else {
				continue
			}
		}
		if ipAddr.To4() == nil || ipAddr.IsLoopback() || ipAddr.IsMulticast() {
			continue
		}
		ips = append(ips, ipAddr)
	}

	for i := range ips {
		first := &net.IPNet{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)}
		second := &net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)}
		third := &net.IPNet{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)}

		// our first pick would be "10.0.0.0/8"
		if first.Contains(ips[i]) {
			ip = ips[i]
			break
			// our 2nd pick would be "192.168.0.0/16"
		} else if second.Contains(ips[i]) {
			ip = ips[i]
			// our 3rd pick would be "172.16.0.0/12"
		} else if third.Contains(ips[i]) && !second.Contains(ip) {
			ip = ips[i]
		}
	}
	if ip == nil {
		if len(ips) > 0 {
			return ips[0]
		}
		// fallback to loopback
		ip = net.IPv4(127, 0, 0, 1)
	}
	return ip
}

// ReplaceUnspecifiedHost replaces unspecified "0.0.0.0" with localhost since "0.0.0.0" is never a valid
// principal (auth server explicitly removes it when issuing host certs) and when a reverse tunnel client used
// establishes SSH reverse tunnel connection the host is validated against
// the valid principal list.
func ReplaceUnspecifiedHost(addr *NetAddr, defaultPort int) string {
	if !addr.IsHostUnspecified() {
		return addr.String()
	}
	port := addr.Port(defaultPort)
	return net.JoinHostPort("localhost", strconv.Itoa(port))
}

// ToLowerCaseASCII returns a lower-case version of in. See RFC 6125 6.4.1. We use
// an explicitly ASCII function to avoid any sharp corners resulting from
// performing Unicode operations on DNS labels.
//
// NOTE: copied verbatim from crypto/x509 source, including the above comments. Teleport
// uses this function to approximate a form of opt-in case-insensitivity for ssh hostnames
func ToLowerCaseASCII(in string) string {
	// If the string is already lower-case then there's nothing to do.
	isAlreadyLowerCase := true
	for _, c := range in {
		if c == utf8.RuneError {
			// If we get a UTF-8 error then there might be
			// upper-case ASCII bytes in the invalid sequence.
			isAlreadyLowerCase = false
			break
		}
		if 'A' <= c && c <= 'Z' {
			isAlreadyLowerCase = false
			break
		}
	}

	if isAlreadyLowerCase {
		return in
	}

	out := []byte(in)
	for i, c := range out {
		if 'A' <= c && c <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}
