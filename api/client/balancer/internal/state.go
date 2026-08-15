// Copyright 2025 Gravitational, Inc
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

package internal

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
)

// State represents the connectivity state and current configuration of a balancer.
type State struct {
	// Conn is the connectivity state as reported by the balancer.
	Conn connectivity.State
	// Health is the connectivity state based on health checks. This will default
	// to [connectivity.Ready] when health checks are disabled.
	Health connectivity.State
	// Reconnect indicates whether reconnecting is enabled based on the last
	// successful config discovery request.
	Reconnect bool
	// HealthChecking indicates whether reconnecting is enabled based on the
	// last successful discovery request.
	HealthChecking bool
	picker         balancer.Picker
}

// Ready determines if the balancer is ready based on the connection and health state.
func (s State) Ready() bool {
	return s.Conn == connectivity.Ready && s.Health == connectivity.Ready
}

// Connecting indicates we are waiting for a connection to be fully established.
// This includes initialization steps like config discovery and health checking.
func (s State) Connecting() bool {
	return s.Conn != connectivity.Ready || s.Health == connectivity.Connecting
}

// Unhealthy indicates the connected server is unhealthy based on health checks.
func (s State) Unhealthy() bool {
	return s.Conn == connectivity.Ready && s.Health == connectivity.TransientFailure
}

func (s State) BalancerState() balancer.State {
	state := s.Conn
	if state == connectivity.Ready {
		state = s.Health
	}
	return balancer.State{
		ConnectivityState: state,
		Picker:            s.picker,
	}
}
