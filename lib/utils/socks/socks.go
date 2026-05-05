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

// package socks implements a SOCKS5 handshake.
package socks

import (
	"encoding/binary"
	"io"
	"net"
	"slices"
	"strconv"

	"github.com/gravitational/trace"
)

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
	if !slices.Contains(authMethods, socks5AuthNotRequired) {
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
	destAddr, err := readDestAddr(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Read in the destination port.
	var destPort uint16
	err = binary.Read(conn, binary.BigEndian, &destPort)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return net.JoinHostPort(destAddr, strconv.Itoa(int(destPort))), nil
}

// readDestAddr reads in the destination address.
func readDestAddr(conn net.Conn) (string, error) {
	// Read in the type of the remote host.
	addrType, err := readByte(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Based off the type, determine how many more bytes to read in for the
	// remote address. For IPv4 it's 4 bytes, for IPv6 it's 16, and for domain
	// names read in another byte to determine the length of the field.
	switch addrType {
	case socks5AddressTypeIPv4:
		destAddr := make([]byte, net.IPv4len)
		_, err = io.ReadFull(conn, destAddr)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return net.IP(destAddr).String(), nil
	case socks5AddressTypeIPv6:
		destAddr := make([]byte, net.IPv6len)
		_, err = io.ReadFull(conn, destAddr)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return net.IP(destAddr).String(), nil
	case socks5AddressTypeDomainName:
		len, err := readByte(conn)
		if err != nil {
			return "", trace.Wrap(err)
		}
		destAddr := make([]byte, len)
		_, err = io.ReadFull(conn, destAddr)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return string(destAddr), nil
	default:
		return "", trace.BadParameter("unsupported address type: %v", addrType)
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
