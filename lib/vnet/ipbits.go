// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	mathrand "math/rand/v2"
	"net"

	"github.com/gravitational/trace"
	"gvisor.dev/gvisor/pkg/tcpip"
)

// newIPv6Prefix returns a Unique Local IPv6 Unicast Address which will be used as a 64-bit prefix for all v6
// IP addresses in the VNet.
func newIPv6Prefix() (tcpip.Address, error) {
	// |   8 bits   |  40 bits   |  16 bits  |          64 bits           |
	// +------------+------------+-----------+----------------------------+
	// | ULA Prefix | Global ID  | Subnet ID |        Interface ID        |
	// +------------+------------+-----------+----------------------------+
	// ULA Prefix is always 0xfd
	// Global ID is random bytes for the specific VNet instance
	// Subnet ID is always 0
	// Interface ID will be the IPv4 address prefixed with zeros.
	var bytes [16]byte
	bytes[0] = 0xfd
	if _, err := rand.Read(bytes[1:6]); err != nil {
		return tcpip.Address{}, trace.Wrap(err, "reading random bytes")
	}
	return tcpip.AddrFrom16(bytes), nil
}

// randomFreeIPv4InNet randomly selects a free address from the IP network range [ipNet], it will call [free]
// to decide if the address is free and it can return, or it needs to keep looking. It will return an error if
// the range is too small or if all of the addresses in the range have been exhausted.
//
// The strategy here is to start with a random address from the range, and if it's free return it, else
// increment it until a free address is found. This should have pretty good performance when the range is
// mostly free, and degrade as it fills.
//
// Most importantly, this strategy allows us to assign IPs in different, possibly overlapping ranges, from
// different clusters without being overly complicated or risking collision.
func randomFreeIPv4InNet(ipNet *net.IPNet, free func(ipv4) bool) (ipv4, error) {
	if len(ipNet.Mask) != 4 || len(ipNet.IP) != 4 {
		return 0, trace.BadParameter("CIDR range must be IPv4, got %q", ipNet.String())
	}
	if leadingOnes, totalBits := ipNet.Mask.Size(); totalBits-leadingOnes < 5 {
		// We have fewer than 5 bits (32 unique addresses)
		return 0, trace.BadParameter("CIDR range must have at least 5 free bits, got %q", ipNet.String())
	}

	netMask := ipv4(binary.BigEndian.Uint32(ipNet.Mask))
	netPrefix := ipv4(binary.BigEndian.Uint32(ipNet.IP))

	// Pick a random starting point and increment until finding a free address.
	randAddrSuffix := ipv4(mathrand.Uint32())

	// Set all leading bits that overlap with the mask to 0.
	randAddrSuffix &= ^netMask
	// Skip 0 and 1, the broadcast address and interface address.
	if randAddrSuffix < 2 {
		randAddrSuffix = 2
	}

	// Record the first attempted suffix to break out of the loop when we get back to it, indicating that the
	// range is full and all IPs are exhausted.
	firstAttempt := randAddrSuffix
	for {
		randAddr := netPrefix | randAddrSuffix
		if free(randAddr) {
			return randAddr, nil
		}

		randAddrSuffix++

		// Set all leading bits that overlap with the mask to 0. This will wrap around the top of the range
		// back to the beginning.
		randAddrSuffix &= ^netMask
		// Skip 0 and 1, the broadcast address and interface address.
		if randAddrSuffix < 2 {
			randAddrSuffix = 2
		}

		if randAddrSuffix == firstAttempt {
			break
		}
	}
	return 0, trace.Wrap(fmt.Errorf("Exhausted all IPs in range %q", ipNet.String()))
}

// ipv4 holds a v4 IP address as a uint32 so we can do math on it.
type ipv4 uint32

func (i ipv4) String() string {
	return net.IP(i.asSlice()).String()
}

func (i ipv4) asArray() [4]byte {
	var bytes [4]byte
	binary.BigEndian.PutUint32(bytes[:], uint32(i))
	return bytes
}

func (i ipv4) asSlice() []byte {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, uint32(i))
	return bytes
}

func ipv4Suffix(addr tcpip.Address) ipv4 {
	bytes := addr.AsSlice()
	bytes = bytes[len(bytes)-4:]
	return ipv4(binary.BigEndian.Uint32(bytes))
}

func ipv6WithSuffix(prefix tcpip.Address, suffix []byte) tcpip.Address {
	addrBytes := prefix.As16()
	offset := len(addrBytes) - len(suffix)
	for i, b := range suffix {
		addrBytes[offset+i] = b
	}
	return tcpip.AddrFrom16(addrBytes)
}
