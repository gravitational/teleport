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

package githubactions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIDTokenSource_GetIDToken(t *testing.T) {
	t.Parallel()
	requestToken := "foo-bar-biz"
	idToken := "iam.a.jwt"
	reqChan := make(chan *http.Request, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqChan <- r

		response := tokenResponse{
			Value: idToken,
		}
		responseBytes, err := json.Marshal(response)
		require.NoError(t, err)
		_, err = w.Write(responseBytes)
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)
	source := IDTokenSource{
		getIDTokenURL: func() string {
			return srv.URL + "/example/github/api?alpha=bravo"
		},
		getRequestToken: func() string {
			return requestToken
		},
	}

	ctx := context.Background()
	got, err := source.GetIDToken(ctx)
	require.NoError(t, err)
	require.Equal(t, idToken, got)

	select {
	// ensure the request includes expected headers/path
	case r := <-reqChan:
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/example/github/api", r.URL.Path)
		require.Equal(t, "bravo", r.URL.Query().Get("alpha"))
		require.Equal(t, "teleport.cluster.local", r.URL.Query().Get("audience"))
		require.Equal(t, "Bearer "+requestToken, r.Header.Get("Authorization"))
	default:
		require.FailNow(t, "missing request value from channel")
	}
}
