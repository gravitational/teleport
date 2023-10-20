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
	require.Equal(t, "__Host-session=7b2275736572223a226c6c616d61222c22736964223a223938373635227d; Path=/; HttpOnly; Secure", setCookies[0])
	// ClearCookie will add an entry with the cookie value cleared out
	require.Equal(t, "__Host-session=; Path=/; HttpOnly; Secure", setCookies[1])
}
