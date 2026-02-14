/*
Copyright 2025 Gravitational, Inc.

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

package defaults

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnnounceTTLFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
		want  time.Duration
		fn    func() time.Duration
	}{
		{
			name:  "set proxy announce ttl empty",
			env:   "TELEPORT_UNSTABLE_PROXY_ANNOUNCE_TTL",
			value: "",
			want:  ServerAnnounceTTL,
			fn:    ProxyAnnounceTTL,
		},
		{
			name:  "set proxy announce ttl 30s",
			env:   "TELEPORT_UNSTABLE_PROXY_ANNOUNCE_TTL",
			value: "30s",
			want:  30 * time.Second,
			fn:    ProxyAnnounceTTL,
		},
		{
			name:  "set proxy announce ttl 1m",
			env:   "TELEPORT_UNSTABLE_PROXY_ANNOUNCE_TTL",
			value: "1m",
			want:  time.Minute,
			fn:    ProxyAnnounceTTL,
		},
		{
			name:  "set auth announce ttl empty",
			env:   "TELEPORT_UNSTABLE_AUTH_ANNOUNCE_TTL",
			value: "",
			want:  ServerAnnounceTTL,
			fn:    AuthAnnounceTTL,
		},
		{
			name:  "set auth announce ttl 30s",
			env:   "TELEPORT_UNSTABLE_AUTH_ANNOUNCE_TTL",
			value: "30s",
			want:  30 * time.Second,
			fn:    AuthAnnounceTTL,
		},
		{
			name:  "set auth announce ttl 1m",
			env:   "TELEPORT_UNSTABLE_AUTH_ANNOUNCE_TTL",
			value: "1m",
			want:  time.Minute,
			fn:    AuthAnnounceTTL,
		},
		{
			name:  "invalid proxy announce ttl fallback",
			env:   "TELEPORT_UNSTABLE_PROXY_ANNOUNCE_TTL",
			value: "not-a-duration",
			want:  ServerAnnounceTTL,
			fn:    ProxyAnnounceTTL,
		},
		{
			name:  "invalid auth announce ttl fallback",
			env:   "TELEPORT_UNSTABLE_AUTH_ANNOUNCE_TTL",
			value: "not-a-duration",
			want:  ServerAnnounceTTL,
			fn:    AuthAnnounceTTL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.env, tt.value)
			require.Equal(t, tt.want, tt.fn())
		})
	}
}
