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

	"github.com/stretchr/testify/require"
)

func TestCookies(t *testing.T) {
	const (
		user      = "llama"
		sessionID = "98765"
	)
	expectedCookie := &Cookie{User: user, SID: sessionID}

	encodedCookie, err := EncodeCookie(user, sessionID)
	require.NoError(t, err)

	cookie, err := DecodeCookie(encodedCookie)
	require.NoError(t, err)
	require.Equal(t, expectedCookie, cookie)

	recorder := httptest.NewRecorder()
	require.Empty(t, recorder.Header().Get("Set-Cookie"))

	require.NoError(t, SetCookie(recorder, user, sessionID))
	ClearCookie(recorder)
	setCookies := recorder.Header().Values("Set-Cookie")
	require.Len(t, setCookies, 2)

	// SetCookie will store the encoded session in the cookie
	require.Equal(t, "__Host-session=7b2275736572223a226c6c616d61222c22736964223a223938373635227d; Path=/; HttpOnly; Secure; SameSite=Lax", setCookies[0])
	// ClearCookie will add an entry with the cookie value cleared out
	require.Equal(t, "__Host-session=; Path=/; HttpOnly; Secure; SameSite=Lax", setCookies[1])
}
