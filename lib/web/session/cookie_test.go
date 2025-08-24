/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package session

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCookies(t *testing.T) {
	const (
		user      = "llama"
		sessionID = "98765"
	)
	expectedCookie := &Cookie{User: user, SID: sessionID}

	t.Run("encode and decode", func(t *testing.T) {
		encodedCookie, err := EncodeCookie(user, sessionID)
		require.NoError(t, err)

		cookie, err := DecodeCookie(encodedCookie)
		require.NoError(t, err)
		require.Equal(t, expectedCookie, cookie)
	})

	tests := []struct {
		name           string
		expiry         time.Time
		expectClear    bool
		expectedCookie string
	}{
		{
			name:           "valid expiry",
			expiry:         time.Now().Add(10 * time.Second),
			expectClear:    true,
			expectedCookie: "__Host-session=7b2275736572223a226c6c616d61222c22736964223a223938373635227d; Path=/; Max-Age=9; HttpOnly; Secure; SameSite=Lax",
		},
		{
			name:           "expired cert (returns session cookie)",
			expiry:         time.Now().Add(-10 * time.Second),
			expectClear:    false,
			expectedCookie: "__Host-session=7b2275736572223a226c6c616d61222c22736964223a223938373635227d; Path=/; HttpOnly; Secure; SameSite=Lax",
		},
		{
			name:           "zero time (returns session cookie)",
			expiry:         time.Time{},
			expectClear:    false,
			expectedCookie: "__Host-session=7b2275736572223a226c6c616d61222c22736964223a223938373635227d; Path=/; HttpOnly; Secure; SameSite=Lax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			require.Empty(t, recorder.Header().Get("Set-Cookie"))

			require.NoError(t, SetCookie(recorder, user, sessionID, tt.expiry))

			if tt.expectClear {
				ClearCookie(recorder)
				setCookies := recorder.Header().Values("Set-Cookie")
				require.Len(t, setCookies, 2)
				require.Equal(t, tt.expectedCookie, setCookies[0])
				require.Equal(t, "__Host-session=; Path=/; HttpOnly; Secure; SameSite=Lax", setCookies[1])
			} else {
				setCookies := recorder.Header().Values("Set-Cookie")
				require.Len(t, setCookies, 1)
				require.Equal(t, tt.expectedCookie, setCookies[0])
			}
		})
	}
}
