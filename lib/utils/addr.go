/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
	return IsLoopback(a.Addr)
}

// IsEmpty returns true if address is empty
func (a *NetAddr) IsEmpty() bool {
	return a.Addr == "" && a.AddrNetwork == "" && a.Path == ""
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

// MarshalYAML defines how a network address should be marshalled to a string
func (a *NetAddr) MarshalYAML() (interface{}, error) {
	url := url.URL{Scheme: a.AddrNetwork, Host: a.Addr, Path: a.Path}
	return strings.TrimLeft(url.String(), "/"), nil
}

// UnmarshalYAML defines how a string can be unmarshalled into a network address
func (a *NetAddr) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
		return err
	}
	a.Addr = v.Addr
	a.AddrNetwork = v.AddrNetwork
	return nil
}

// ParseAddr takes strings like "tcp://host:port/path" and returns
// *NetAddr or an error
func ParseAddr(a string) (*NetAddr, error) {
	if !strings.Contains(a, "://") {
		host, port, err := net.SplitHostPort(a)
		if err != nil {
			return nil, trace.BadParameter("invalid network address: '%v', expecting host:port", a)
		}
		return &NetAddr{Addr: fmt.Sprintf("%v:%v", host, port), AddrNetwork: "tcp"}, nil
	}
	u, err := url.Parse(a)
	if err != nil {
		return nil, fmt.Errorf("failed to parse '%v':%v", a, err)
	}
	switch u.Scheme {
	case "tcp":
		return &NetAddr{Addr: u.Host, AddrNetwork: u.Scheme, Path: u.Path}, nil
	case "unix":
		return &NetAddr{Addr: u.Path, AddrNetwork: u.Scheme}, nil
	default:
		return nil, trace.BadParameter("'%v': unsupported scheme: '%v'", a, u.Scheme)
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

// ParseHostPortAddr takes strings like "host:port" and returns
// *NetAddr or an error
//
// If defaultPort == -1 it expects 'hostport' string to have it
func ParseHostPortAddr(hostport string, defaultPort int) (*NetAddr, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		if defaultPort > 0 {
			host, port, err = net.SplitHostPort(
				net.JoinHostPort(hostport, strconv.Itoa(defaultPort)))
		}
		if err != nil {
			return nil, trace.Errorf("failed to parse '%v': %v", hostport, err)
		}
	}
	return ParseAddr(fmt.Sprintf("tcp://%s", net.JoinHostPort(host, port)))
}

func NewNetAddrVal(defaultVal NetAddr, val *NetAddr) *NetAddrVal {
	*val = defaultVal
	return (*NetAddrVal)(val)
}

// NetAddrVal can be used with flag package
type NetAddrVal NetAddr

func (a *NetAddrVal) Set(s string) error {
	v, err := ParseAddr(s)
	if err != nil {
		return err
	}
	a.Addr = v.Addr
	a.AddrNetwork = v.AddrNetwork
	return nil
}

func (a *NetAddrVal) String() string {
	return ((*NetAddr)(a)).FullAddress()
}

func (a *NetAddrVal) Get() interface{} {
	return NetAddr(*a)
}

// NetAddrList is a list of NetAddrs that supports
// helper methods for parsing from CLI tools
type NetAddrList []NetAddr

// Addresses returns a slice of strings converted from the addresses
func (nl *NetAddrList) Addresses() []string {
	var ns []string
	for _, n := range *nl {
		ns = append(ns, n.FullAddress())
	}
	return ns
}

// Set is called by CLI tools
func (nl *NetAddrList) Set(s string) error {
	v, err := ParseAddr(s)
	if err != nil {
		return err
	}

	*nl = append(*nl, *v)
	return nil
}

// String returns debug-friendly representation of the tool
func (nl *NetAddrList) String() string {
	var ns []string
	for _, n := range *nl {
		ns = append(ns, n.FullAddress())
	}
	return strings.Join(ns, " ")
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

// IsLoopback returns 'true' if a given hostname resolves to local
// host's loopback interface
func IsLoopback(host string) bool {
	if strings.Contains(host, ":") {
		var err error
		host, _, err = net.SplitHostPort(host)
		if err != nil {
			return false
		}
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return false
	}
	for _, ip := range ips {
		if ip.IsLoopback() {
			return true
		}
	}
	return false
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
			log.Warn(err)
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
		switch ips[i][12] {
		// our first pick would be "10.x.x.x" IPs:
		case 10:
			return ips[i]
			// our 2nd pick would be "192.x.x.x"
		case 192:
			ip = ips[i]
			// our 3rd pick would be "172.x.x.x"
		case 172:
			if ip == nil {
				ip = ips[i]
			}
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
