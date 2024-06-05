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
	"context"
	"errors"
	"net"
	"slices"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/aws"
)

// SSHRouteMatcher is a helper used to decide if an ssh dial request should match
// a given server. This is broken out of proxy.Router as a standalone helper in order
// to let other parts of teleport easily find matching servers when generating
// error messages or building access requests.
type SSHRouteMatcher struct {
	cfg            SSHRouteMatcherConfig
	ips            []string
	matchServerIDs bool
}

// SSHRouteMatcherConfig configures an SSHRouteMatcher.
type SSHRouteMatcherConfig struct {
	// Host is the target host that we want to route to.
	Host string
	// Port is an optional target port. If empty or zero
	// it will match servers listening on any port.
	Port string
	// Resolver can be set to override default hostname lookup
	// behavior (used in tests).
	Resolver HostResolver
	// CaseInsensitive enabled case insensitive routing when true.
	CaseInsensitive bool
}

// HostResolver provides an interface matching the net.Resolver.LookupHost method. Typically
// only used as a means of overriding dns resolution behavior in tests.
type HostResolver interface {
	// LookupHost performs a hostname lookup.  See net.Resolver.LookupHost for details.
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
}

var errEmptyHost = errors.New("cannot route to empty target host")

// NewSSHRouteMatcherFromConfig sets up an ssh route matcher from the supplied configuration.
func NewSSHRouteMatcherFromConfig(cfg SSHRouteMatcherConfig) (*SSHRouteMatcher, error) {
	if cfg.Host == "" {
		return nil, trace.Wrap(errEmptyHost)
	}

	if cfg.Resolver == nil {
		cfg.Resolver = net.DefaultResolver
	}

	m := newSSHRouteMatcher(cfg)
	return &m, nil
}

// NewSSHRouteMatcher builds a new matcher for ssh routing decisions.
func NewSSHRouteMatcher(host, port string, caseInsensitive bool) SSHRouteMatcher {
	return newSSHRouteMatcher(SSHRouteMatcherConfig{
		Host:            host,
		Port:            port,
		CaseInsensitive: caseInsensitive,
		Resolver:        net.DefaultResolver,
	})
}

func newSSHRouteMatcher(cfg SSHRouteMatcherConfig) SSHRouteMatcher {
	_, err := uuid.Parse(cfg.Host)
	dialByID := err == nil || aws.IsEC2NodeID(cfg.Host)

	ips, _ := cfg.Resolver.LookupHost(context.Background(), cfg.Host)

	return SSHRouteMatcher{
		cfg:            cfg,
		ips:            ips,
		matchServerIDs: dialByID,
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
	return m.RouteToServerScore(server) > 0
}

const (
	notMatch      = 0
	indirectMatch = 1
	directMatch   = 2
)

// RouteToServerScore checks wether this route matcher wants to route to the supplied server
// and represents the result of that check as an integer score indicating the strength of the
// match. Positive scores indicate a match, higher being stronger.
func (m *SSHRouteMatcher) RouteToServerScore(server RouteableServer) (score int) {
	// if host is a UUID or EC2 ID match only
	// by server name and treat matches as unambiguous
	if m.matchServerIDs && server.GetName() == m.cfg.Host {
		return directMatch
	}

	hostnameMatch := m.routeToHostname(server.GetHostname())

	// if the server has connected over a reverse tunnel
	// then match only by hostname.
	if server.GetUseTunnel() {
		if hostnameMatch {
			return directMatch
		}
		return notMatch
	}

	matchAddr := func(addr string) int {
		ip, nodePort, err := net.SplitHostPort(addr)
		if err != nil {
			return notMatch
		}

		if m.cfg.Port != "" && m.cfg.Port != "0" && m.cfg.Port != nodePort {
			// if port is well-specified and does not match, don't bother
			// continuing the check.
			return notMatch
		}

		if hostnameMatch || m.cfg.Host == ip {
			// server presents a hostname or addr that exactly matches
			// our target.
			return directMatch
		}

		if slices.Contains(m.ips, ip) {
			// server presents an addr that indirectly matches our target
			// due to dns resolution.
			return indirectMatch
		}

		return notMatch
	}

	score = matchAddr(server.GetAddr())

	for _, addr := range server.GetPublicAddrs() {
		score = max(score, matchAddr(addr))
	}

	return score
}

// routeToHostname helps us perform a special kind of case-insensitive comparison. SSH certs do not generally
// treat principals/hostnames in a case-insensitive manner. This is often worked-around by forcing all principals and
// hostnames to be lowercase. For backwards-compatibility reasons, teleport must support case-sensitive routing by default
// and can't do this. Instead, teleport nodes whose hostnames contain uppercase letters will present certs that include both
// the literal hostname and a lowered version of the hostname, meaning that it is sane to route a request for host 'foo' to
// host 'Foo', but it is not sane to route a request for host 'Bar' to host 'bar'.
func (m *SSHRouteMatcher) routeToHostname(principal string) bool {
	if !m.cfg.CaseInsensitive {
		return m.cfg.Host == principal
	}

	if len(m.cfg.Host) != len(principal) {
		return false
	}

	// the below is modeled off of the fast ASCII path of strings.EqualFold
	for i := 0; i < len(principal) && i < len(m.cfg.Host); i++ {
		pr := principal[i]
		hr := m.cfg.Host[i]
		if pr|hr >= utf8.RuneSelf {
			// not pure-ascii, fallback to literal comparison
			return m.cfg.Host == principal
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
	return m.cfg.Host == ""
}

// MatchesServerIDs checks if this matcher wants to perform server ID matching.
func (m *SSHRouteMatcher) MatchesServerIDs() bool {
	return m.matchServerIDs
}
