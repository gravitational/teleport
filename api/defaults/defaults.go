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

	// ServerAnnounceTTL is the default TTL of server presence resources.
	ServerAnnounceTTL = 10 * time.Minute

	// InstanceHeartbeatTTL is the default TTL of the instance presence resource.
	InstanceHeartbeatTTL = 20 * time.Minute

	// MaxInstanceHeartbeatInterval is the upper bound of the variable instance
	// heartbeat interval.
	MaxInstanceHeartbeatInterval = 18 * time.Minute

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
)

var (
	moduleLock sync.RWMutex

	// serverKeepAliveTTL is a period between server keep-alives,
	// when servers announce only presence without sending full data
	serverKeepAliveTTL = 1 * time.Minute

	// keepAliveInterval is interval at which Teleport will send keep-alive
	// messages to the client. The default interval of 5 minutes (300 seconds) is
	// set to help keep connections alive when using AWS NLBs (which have a default
	// timeout of 350 seconds)
	keepAliveInterval = 5 * time.Minute

	// minInstanceHeartbeatInterval is the lower bound of the variable instance
	// heartbeat interval.
	minInstanceHeartbeatInterval = 3 * time.Minute
)

func SetTestTimeouts(svrKeepAliveTTL, keepAliveTick time.Duration) {
	moduleLock.Lock()
	defer moduleLock.Unlock()

	serverKeepAliveTTL = svrKeepAliveTTL
	keepAliveInterval = keepAliveTick

	// maintain the proportional relationship of instance hb interval to
	// server hb interval.
	minInstanceHeartbeatInterval = svrKeepAliveTTL * 3
}

func ServerKeepAliveTTL() time.Duration {
	moduleLock.RLock()
	defer moduleLock.RUnlock()
	return serverKeepAliveTTL
}

func MinInstanceHeartbeatInterval() time.Duration {
	moduleLock.RLock()
	defer moduleLock.RUnlock()
	return minInstanceHeartbeatInterval
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
	// DefaultMaxErrorMessageSize is the default maximum size of an error message.
	// This can be used to truncate large error messages, which might cause gRPC messages to exceed the maximum allowed size.
	DefaultMaxErrorMessageSize = 1024 * 100 // 100KB
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
	// TODO(greedy52) DELETE in ??. Note that this toggle was planned to be
	// deleted in 15.0 when the feature exits preview. However, many users
	// still rely on this manual toggle as IsALPNConnUpgradeRequired cannot
	// detect many situations where connection upgrade is required. This can be
	// deleted once IsALPNConnUpgradeRequired is improved.
	TLSRoutingConnUpgradeEnvVar = "TELEPORT_TLS_ROUTING_CONN_UPGRADE"
)

const (
	// HealthCheckInterval is the default resource health check interval.
	HealthCheckInterval time.Duration = 30 * time.Second
	// HealthCheckTimeout is the default resource health check timeout.
	HealthCheckTimeout time.Duration = 5 * time.Second
	// HealthCheckHealthyThreshold is the default resource health check healthy
	// threshold.
	HealthCheckHealthyThreshold uint32 = 2
	// HealthCheckUnhealthyThreshold is the default resource health check
	// unhealthy threshold.
	HealthCheckUnhealthyThreshold uint32 = 1
)
