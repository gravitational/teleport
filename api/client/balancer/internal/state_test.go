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
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/connectivity"
)

func TestStateReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state State
		want  bool
	}{
		{
			name: "ready when conn and health ready",
			state: State{
				Conn:   connectivity.Ready,
				Health: connectivity.Ready,
			},
			want: true,
		},
		{
			name: "not ready when conn not ready",
			state: State{
				Conn:   connectivity.Connecting,
				Health: connectivity.Ready,
			},
			want: false,
		},
		{
			name: "not ready when health not ready",
			state: State{
				Conn:   connectivity.Ready,
				Health: connectivity.TransientFailure,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.state.Ready())
		})
	}
}

func TestStateConnecting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state State
		want  bool
	}{
		{
			name: "connecting when conn not ready",
			state: State{
				Conn:   connectivity.Connecting,
				Health: connectivity.Ready,
			},
			want: true,
		},
		{
			name: "connecting when health connecting",
			state: State{
				Conn:   connectivity.Ready,
				Health: connectivity.Connecting,
			},
			want: true,
		},
		{
			name: "not connecting when ready and healthy",
			state: State{
				Conn:   connectivity.Ready,
				Health: connectivity.Ready,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.state.Connecting())
		})
	}
}

func TestStateUnhealthy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state State
		want  bool
	}{
		{
			name: "unhealthy when conn ready and health transient failure",
			state: State{
				Conn:   connectivity.Ready,
				Health: connectivity.TransientFailure,
			},
			want: true,
		},
		{
			name: "not unhealthy when conn not ready",
			state: State{
				Conn:   connectivity.Connecting,
				Health: connectivity.TransientFailure,
			},
			want: false,
		},
		{
			name: "not unhealthy when health ready",
			state: State{
				Conn:   connectivity.Ready,
				Health: connectivity.Ready,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.state.Unhealthy())
		})
	}
}
