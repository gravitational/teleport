// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"net"
	"slices"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/gravitational/teleport/api/utils/aws"
)

// SSHRouteMatcher is a helper used to decide if an ssh dial request should match
// a given server. This is broken out of proxy.Router as a standalone helper in order
// to let other parts of teleport easily find matching servers when generating
// error messages or building access requests.
type SSHRouteMatcher struct {
	targetHost      string
	targetPort      string
	caseInsensitive bool
	ips             []string
	matchServerIDs  bool
}

// NewSSHRouteMatcher builds a new matcher for ssh routing decisions.
func NewSSHRouteMatcher(host, port string, caseInsensitive bool) SSHRouteMatcher {
	_, err := uuid.Parse(host)
	dialByID := err == nil || aws.IsEC2NodeID(host)

	ips, _ := net.LookupHost(host)

	return SSHRouteMatcher{
		targetHost:      host,
		targetPort:      port,
		caseInsensitive: caseInsensitive,
		ips:             ips,
		matchServerIDs:  dialByID,
	}
}

// RouteableServer is an interface describing the subset of the types.Server interface
// required to make a routing decision.
type RouteableServer interface {
	GetName() string
	GetHostname() string
	GetAddr() string
	GetUseTunnel() bool
	GetPublicAddrs() []string
}

// RouteToServer checks if this route matcher wants to route to the supplied server.
func (m *SSHRouteMatcher) RouteToServer(server RouteableServer) bool {
	// if host is a UUID or EC2 ID match only
	// by server name and treat matches as unambiguous
	if m.matchServerIDs && server.GetName() == m.targetHost {
		return true
	}

	hostnameMatch := m.routeToHostname(server.GetHostname())

	// if the server has connected over a reverse tunnel
	// then match only by hostname.
	if server.GetUseTunnel() {
		return hostnameMatch
	}

	matchAddr := func(addr string) bool {
		ip, nodePort, err := net.SplitHostPort(addr)
		if err != nil {
			return false
		}

		if (m.targetHost == ip || hostnameMatch || slices.Contains(m.ips, ip)) &&
			(m.targetPort == "" || m.targetPort == "0" || m.targetPort == nodePort) {
			return true
		}

		return false
	}

	if matchAddr(server.GetAddr()) {
		return true
	}

	for _, addr := range server.GetPublicAddrs() {
		if matchAddr(addr) {
			return true
		}
	}

	return false
}

// routeToHostname helps us perform a special kind of case-insensitive comparison. SSH certs do not generally
// treat principals/hostnames in a case-insensitive manner. This is often worked-around by forcing all principals and
// hostnames to be lowercase. For backwards-compatibility reasons, teleport must support case-sensitive routing by default
// and can't do this. Instead, teleport nodes whose hostnames contain uppercase letters will present certs that include both
// the literal hostname and a lowered version of the hostname, meaning that it is sane to route a request for host 'foo' to
// host 'Foo', but it is not sane to route a request for host 'Bar' to host 'bar'.
func (m *SSHRouteMatcher) routeToHostname(principal string) bool {
	if !m.caseInsensitive {
		return m.targetHost == principal
	}

	if len(m.targetHost) != len(principal) {
		return false
	}

	// the below is modeled off of the fast ASCII path of strings.EqualFold
	for i := 0; i < len(principal) && i < len(m.targetHost); i++ {
		pr := principal[i]
		hr := m.targetHost[i]
		if pr|hr >= utf8.RuneSelf {
			// not pure-ascii, fallback to literal comparison
			return m.targetHost == principal
		}

		// Easy case.
		if pr == hr {
			continue
		}

		// Check if principal is an upper-case equivalent to host.
		if 'A' <= pr && pr <= 'Z' && hr == pr+'a'-'A' {
			continue
		}
		return false
	}

	return true
}

// IsEmpty checks if this route matcher has had a hostname set.
func (m *SSHRouteMatcher) IsEmpty() bool {
	return m.targetHost == ""
}

// MatchesServerIDs checks if this matcher wants to perform server ID matching.
func (m *SSHRouteMatcher) MatchesServerIDs() bool {
	return m.matchServerIDs
}
