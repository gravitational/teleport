// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestWebSessionV2_GetEarliestExpiry(t *testing.T) {
	t.Parallel()

	bearer := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	session := bearer.Add(1 * time.Hour)

	tests := []struct {
		name               string
		usage              types.WebSessionUsage
		bearerTokenExpires time.Time
		sessionExpires     time.Time
		want               time.Time
	}{
		{
			name:               "standard usage with bearer earlier than session returns bearer",
			usage:              types.WebSessionUsage_WEB_SESSION_USAGE_UNSPECIFIED,
			bearerTokenExpires: bearer,
			sessionExpires:     session,
			want:               bearer,
		},
		{
			name:               "standard usage with bearer later than session is capped at session",
			usage:              types.WebSessionUsage_WEB_SESSION_USAGE_UNSPECIFIED,
			bearerTokenExpires: session.Add(1 * time.Hour),
			sessionExpires:     session,
			want:               session,
		},
		{
			name:               "standard usage with zero bearer falls back to session",
			usage:              types.WebSessionUsage_WEB_SESSION_USAGE_UNSPECIFIED,
			bearerTokenExpires: time.Time{},
			sessionExpires:     session,
			want:               session,
		},
		{
			name:               "standard usage with zero session falls back to bearer",
			usage:              types.WebSessionUsage_WEB_SESSION_USAGE_UNSPECIFIED,
			bearerTokenExpires: bearer,
			sessionExpires:     time.Time{},
			want:               bearer,
		},
		{
			name:               "access graph usage ignores bearer and returns session expiry",
			usage:              types.WebSessionUsage_WEB_SESSION_USAGE_ACCESS_GRAPH_API,
			bearerTokenExpires: time.Time{},
			sessionExpires:     session,
			want:               session,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws, err := types.NewWebSession(uuid.NewString(), types.KindWebSession, types.WebSessionSpecV2{
				User:               "alice",
				BearerTokenExpires: tc.bearerTokenExpires,
				Expires:            tc.sessionExpires,
				Usage:              tc.usage,
			})
			require.NoError(t, err)

			got := ws.GetEarliestExpiry()
			require.True(t, tc.want.Equal(got), "expected %s, got %s", tc.want, got)
		})
	}
}
