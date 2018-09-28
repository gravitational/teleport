/*
Copyright 2018 Gravitational, Inc.

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

// package socks implements a SOCKS5 handshake.
package socks

import (
	"encoding/binary"
	"io"
	"net"
	"strconv"

	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentSOCKS,
})

const (
	socks5Version               byte = 0x05
	socks5Reserved              byte = 0x00
	socks5AuthNotRequired       byte = 0x00
	socks5CommandConnect        byte = 0x01
	socks5AddressTypeIPv4       byte = 0x01
	socks5AddressTypeDomainName byte = 0x03
	socks5AddressTypeIPv6       byte = 0x04
	socks5Succeeded             byte = 0x00
)

// Handshake performs a SOCKS5 handshake with the client and returns
// the remote address to proxy the connection to.
func Handshake(conn net.Conn) (string, error) {
	// Read in the version and reject anything other than SOCKS5.
	version, err := readByte(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if version != socks5Version {
		return "", trace.BadParameter("only SOCKS5 is supported")
	}

	// Read in the authentication method requested by the client and write back
	// the method that was selected. At the moment only "no authentication
	// required" is supported.
	authMethods, err := readAuthenticationMethod(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if !byteSliceContains(authMethods, socks5AuthNotRequired) {
		return "", trace.BadParameter("only 'no authentication required' is supported")
	}
	err = writeMethodSelection(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Read in the request from the client and make sure the requested command
	// is supported and extract the remote address. If everything is good, write
	// out a success response.
	remoteAddr, err := readRequest(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}
	err = writeReply(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return remoteAddr, nil
}

// readAuthenticationMethod reads in the authentication methods the client
// supports.
func readAuthenticationMethod(conn net.Conn) ([]byte, error) {
	// Read in the number of authentication methods supported.
	nmethods, err := readByte(conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Read nmethods number of bytes from the connection return the list of
	// supported authentication methods to the caller.
	authMethods := make([]byte, nmethods)
	for i := byte(0); i < nmethods; i++ {
		method, err := readByte(conn)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		authMethods = append(authMethods, method)
	}

	return authMethods, nil
}

// writeMethodSelection writes out the response to the authentication methods.
// Right now, only SOCKS5 and "no authentication methods" is supported.
func writeMethodSelection(conn net.Conn) error {
	message := []byte{socks5Version, socks5AuthNotRequired}

	n, err := conn.Write(message)
	if err != nil {
		return trace.Wrap(err)
	}
	if n != len(message) {
		return trace.BadParameter("wrote: %v wanted to write: %v", n, len(message))
	}

	return nil
}

// readRequest reads in the SOCKS5 request from the client and returns the
// host:port the client wants to connect to.
func readRequest(conn net.Conn) (string, error) {
	// Read in the version and reject anything other than SOCKS5.
	version, err := readByte(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if version != socks5Version {
		return "", trace.BadParameter("only SOCKS5 is supported")
	}

	// Read in the command the client is requesting. Only CONNECT is supported.
	command, err := readByte(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if command != socks5CommandConnect {
		return "", trace.BadParameter("only CONNECT command is supported")
	}

	// Read in and throw away the reserved byte.
	_, err = readByte(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Read in the address type and determine how many more bytes need to be
	// read in to read in the remote host address.
	addrLen, err := readAddrType(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Read in the destination address.
	destAddr := make([]byte, addrLen)
	_, err = io.ReadFull(conn, destAddr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Read in the destination port.
	var destPort uint16
	err = binary.Read(conn, binary.BigEndian, &destPort)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return net.JoinHostPort(string(destAddr), strconv.Itoa(int(destPort))), nil
}

// readAddrType reads in the address type and returns the length of the dest
// addr field.
func readAddrType(conn net.Conn) (int, error) {
	// Read in the type of the remote host.
	addrType, err := readByte(conn)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Based off the type, determine how many more bytes to read in for the
	// remote address. For IPv4 it's 4 bytes, for IPv6 it's 16, and for domain
	// names read in another byte to determine the length of the field.
	switch addrType {
	case socks5AddressTypeIPv4:
		return net.IPv4len, nil
	case socks5AddressTypeIPv6:
		return net.IPv6len, nil
	case socks5AddressTypeDomainName:
		len, err := readByte(conn)
		if err != nil {
			return 0, trace.Wrap(err)
		}
		return int(len), nil
	default:
		return 0, trace.BadParameter("unsupported address type: %v", addrType)
	}
}

// Write the response to the client.
func writeReply(conn net.Conn) error {
	// Write success reply, similar to OpenSSH only success is written.
	// https://github.com/openssh/openssh-portable/blob/5d14019/channels.c#L1442-L1452
	message := []byte{
		socks5Version,
		socks5Succeeded,
		socks5Reserved,
		socks5AddressTypeIPv4,
	}
	n, err := conn.Write(message)
	if err != nil {
		return trace.Wrap(err)
	}
	if n != len(message) {
		return trace.BadParameter("wrote: %v wanted to write: %v", n, len(message))
	}

	// Reply also requires BND.ADDR and BDN.PORT even though they are ignored
	// because Teleport only supports CONNECT.
	message = []byte{0, 0, 0, 0, 0, 0}
	n, err = conn.Write(message)
	if err != nil {
		return trace.Wrap(err)
	}
	if n != len(message) {
		return trace.BadParameter("wrote: %v wanted to write: %v", n, len(message))
	}

	return nil
}

// readByte a single byte from the passed in net.Conn.
func readByte(conn net.Conn) (byte, error) {
	b := make([]byte, 1)
	_, err := io.ReadFull(conn, b)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return b[0], nil
}

// byteSliceContains checks if the slice a contains the byte b.
func byteSliceContains(a []byte, b byte) bool {
	for _, v := range a {
		if v == b {
			return true
		}
	}

	return false
}
