/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package azuredevops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIDTokenSource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mux := http.NewServeMux()
	mux.HandleFunc("/oidctoken", func(w http.ResponseWriter, req *http.Request) {
		// Check the request
		require.Equal(t, http.MethodPost, req.Method)
		authHeader := req.Header.Get("Authorization")
		require.NotEmpty(t, authHeader)
		require.Equal(t, "Bearer FAKE_ACCESS_TOKEN", authHeader)
		require.Equal(t, "7.1", req.URL.Query().Get("api-version"))
		// Send response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := createOidctokenResp{
			OIDCToken: "FAKE_ID_TOKEN",
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	fakeEnv := map[string]string{
		"SYSTEM_ACCESSTOKEN":    "FAKE_ACCESS_TOKEN",
		"SYSTEM_OIDCREQUESTURI": srv.URL + "/oidctoken",
	}
	getFakeEnv := func(key string) string {
		return fakeEnv[key]
	}

	idTokenSource := NewIDTokenSource(getFakeEnv)

	got, err := idTokenSource.GetIDToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "FAKE_ID_TOKEN", got)
}
