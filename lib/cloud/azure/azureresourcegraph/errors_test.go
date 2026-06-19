/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package azureresourcegraph

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExtractRetryAfterDuration(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers map[string]string
		want    time.Duration
	}{
		{name: "retry-after seconds", headers: map[string]string{"Retry-After": "5"}, want: 5 * time.Second},
		{name: "retry-after zero", headers: map[string]string{"Retry-After": "0"}, want: 0},
		{name: "quota resets after hh:mm:ss", headers: map[string]string{"X-Ms-User-Quota-Resets-After": "00:00:05"}, want: 5 * time.Second},
		{name: "quota resets with hours", headers: map[string]string{"X-Ms-User-Quota-Resets-After": "01:02:03"}, want: time.Hour + 2*time.Minute + 3*time.Second},
		{
			name:    "retry-after takes precedence",
			headers: map[string]string{"Retry-After": "10", "X-Ms-User-Quota-Resets-After": "00:00:05"},
			want:    10 * time.Second,
		},
		{
			name:    "falls back to quota header when retry-after is unparseable",
			headers: map[string]string{"Retry-After": "soon", "X-Ms-User-Quota-Resets-After": "00:00:07"},
			want:    7 * time.Second,
		},
		{name: "no headers", headers: nil, want: 0},
		{name: "quota wrong number of parts", headers: map[string]string{"X-Ms-User-Quota-Resets-After": "00:05"}, want: 0},
		{name: "quota non-numeric", headers: map[string]string{"X-Ms-User-Quota-Resets-After": "aa:bb:cc"}, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			header := make(http.Header)
			for k, v := range tc.headers {
				header.Set(k, v)
			}
			require.Equal(t, tc.want, extractRetryAfterDuration(header))
		})
	}
}
