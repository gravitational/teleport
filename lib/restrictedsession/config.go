/*
Copyright 2021 Gravitational, Inc.

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
