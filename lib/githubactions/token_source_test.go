/*
Copyright 2022 Gravitational, Inc.

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
