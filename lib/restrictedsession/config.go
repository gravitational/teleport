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

package restrictedsession

import (
	"net"

	"github.com/gravitational/trace"
)

func compactIP(ip net.IP) net.IP {
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4
	}
	return ip
}

// ParseIPSpec takes in either a CIDR format (e.g. 192.168.1.2/16 or fe::/8)
// or a single IP address (e.g. 10.1.2.3 or fe::1) and returns *net.IPNet.
// In case of a single IP address, the associated network length is either
// /32 for IPv4 or /128 for IPv6.
func ParseIPSpec(cidr string) (*net.IPNet, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err == nil {
		return ipnet, nil
	}

	// not in CIDR format, try as a plain IP
	ip := net.ParseIP(cidr)
	if ip == nil {
		return nil, trace.BadParameter("%q is not an IP nor CIDR", cidr)
	}

	ip = compactIP(ip)
	bits := len(ip) * 8
	return &net.IPNet{
		IP:   ip,
		Mask: net.CIDRMask(bits, bits),
	}, nil
}

// NetworkRestrictions specifies which addresses should be blocked.
type NetworkRestrictions struct {
	// Enabled controls if restrictions are enforced.
	Enabled bool

	// Allow holds a list of IPs (with masks) to allow, overriding deny list
	Allow []net.IPNet

	// Deny holds a list of IPs (with masks) to deny (block)
	Deny []net.IPNet
}
