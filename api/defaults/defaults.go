/*
Copyright 2020 Gravitational, Inc.

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

// Package defaults defines Teleport-specific defaults
package defaults

import (
	"sync"
	"time"

	"github.com/gravitational/teleport/api/constants"
)

const (
	// Namespace is default namespace
	Namespace = "default"

	// DefaultIOTimeout is a default network IO timeout.
	DefaultIOTimeout = 30 * time.Second

	// DefaultIdleTimeout is a default idle connection timeout.
	DefaultIdleTimeout = 30 * time.Second

	// KeepAliveCountMax is the number of keep-alive messages that can be sent
	// without receiving a response from the client before the client is
	// disconnected. The max count mirrors ClientAliveCountMax of sshd.
	KeepAliveCountMax = 3

	// MinCertDuration specifies minimum duration of validity of issued certificate
	MinCertDuration = time.Minute

	// MaxCertDuration limits maximum duration of validity of issued certificate
	MaxCertDuration = 30 * time.Hour

	// CertDuration is a default certificate duration.
	CertDuration = 12 * time.Hour

	// ServerAnnounceTTL is a period between heartbeats
	// Median sleep time between node pings is this value / 2 + random
	// deviation added to this time to avoid lots of simultaneous
	// heartbeats coming to auth server
	ServerAnnounceTTL = 600 * time.Second

	// SessionTrackerTTL defines the default base ttl of a session tracker.
	SessionTrackerTTL = 30 * time.Minute

	// BreakerInterval is the period in time the circuit breaker will
	// tally metrics for
	BreakerInterval = time.Minute

	// TrippedPeriod is the default period of time the circuit breaker will
	// remain in breaker.StateTripped before transitioning to breaker.StateRecovering. No
	// outbound requests are allowed for the duration of this period.
	TrippedPeriod = 60 * time.Second

	// RecoveryLimit is the default number of consecutive successful requests needed to transition
	// from breaker.StateRecovering to breaker.StateStandby
	RecoveryLimit = 3

	// BreakerRatio is the default ratio of failed requests to successful requests that will
	// result in the circuit breaker transitioning to breaker.StateTripped
	BreakerRatio = 0.9

	// BreakerRatioMinExecutions is the minimum number of requests before the ratio tripper
	// will consider examining the request pass rate
	BreakerRatioMinExecutions = 10

	// AssistCommandExecutionWorkers is the number of workers that will
	// execute arbitrary remote commands on servers in parallel
	AssistCommandExecutionWorkers = 30
)

var (
	moduleLock sync.RWMutex

	// serverKeepAliveTTL is a period between server keep-alives,
	// when servers announce only presence without sending full data
	serverKeepAliveTTL = 60 * time.Second

	// keepAliveInterval is interval at which Teleport will send keep-alive
	// messages to the client. The default interval of 5 minutes (300 seconds) is
	// set to help keep connections alive when using AWS NLBs (which have a default
	// timeout of 350 seconds)
	keepAliveInterval = 5 * time.Minute
)

func SetTestTimeouts(svrKeepAliveTTL, keepAliveTick time.Duration) {
	moduleLock.Lock()
	defer moduleLock.Unlock()

	serverKeepAliveTTL = svrKeepAliveTTL
	keepAliveInterval = keepAliveTick
}

func ServerKeepAliveTTL() time.Duration {
	moduleLock.RLock()
	defer moduleLock.RUnlock()
	return serverKeepAliveTTL
}

func KeepAliveInterval() time.Duration {
	moduleLock.RLock()
	defer moduleLock.RUnlock()
	return keepAliveInterval
}

// EnhancedEvents returns the default list of enhanced events.
func EnhancedEvents() []string {
	return []string{
		constants.EnhancedRecordingCommand,
		constants.EnhancedRecordingNetwork,
	}
}

const (
	// DefaultChunkSize is the default chunk size for paginated endpoints.
	DefaultChunkSize = 1000
)

const (
	// When running in "SSH Proxy" role this port will be used for incoming
	// connections from SSH nodes who wish to use "reverse tunnell" (when they
	// run behind an environment/firewall which only allows outgoing connections)
	SSHProxyTunnelListenPort = 3024

	// SSHProxyListenPort is the default Teleport SSH proxy listen port.
	SSHProxyListenPort = 3023

	// ProxyWebListenPort is the default Teleport Proxy WebPort address.
	ProxyWebListenPort = 3080

	// StandardHTTPSPort is the default port used for the https URI scheme.
	StandardHTTPSPort = 443
)

const (
	// TunnelPublicAddrEnvar optionally specifies the alternative reverse tunnel address.
	TunnelPublicAddrEnvar = "TELEPORT_TUNNEL_PUBLIC_ADDR"

	// TLSRoutingConnUpgradeEnvVar overwrites the test result for deciding if
	// ALPN connection upgrade is required.
	//
	// Sample values:
	// true
	// <some.cluster.com>=yes,<another.cluster.com>=no
	// 0,<some.cluster.com>=1
	//
	// TODO(greedy52) DELETE IN 15.0
	TLSRoutingConnUpgradeEnvVar = "TELEPORT_TLS_ROUTING_CONN_UPGRADE"
)
