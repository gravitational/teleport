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

func (a *NetAddr) Network() string {
	return a.AddrNetwork
}

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
		return nil, trace.Errorf("unsupported scheme '%v': '%v'", a, u.Scheme)
	}
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

func NewNetAddrList(addrs *[]NetAddr) *NetAddrList {
	return &NetAddrList{addrs: addrs}
}

type NetAddrList struct {
	addrs *[]NetAddr
}

func (nl *NetAddrList) Set(s string) error {
	v, err := ParseAddr(s)
	if err != nil {
		return err
	}
	*nl.addrs = append(*nl.addrs, *v)
	return nil
}

func (nl *NetAddrList) String() string {
	var ns []string
	for _, n := range *nl.addrs {
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
	ip := net.ParseIP(host)
	if ip.IsLoopback() || ip.IsUnspecified() {
		host, _, err = net.SplitHostPort(replaceWith)
		if err != nil {
			return addr
		}
		addr = net.JoinHostPort(host, port)
	}
	return addr
}
