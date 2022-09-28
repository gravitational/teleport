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
	// TODO: Make this table driven & add more cases
	requestToken := "foo-bar-biz"
	idToken := "iam.a.jwt"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// mimics the internal API accessible by Github Actions
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/example/github/api", r.URL.Path)
		require.Equal(t, "bravo", r.URL.Query().Get("alpha"))
		require.Equal(t, "teleport.cluster.local", r.URL.Query().Get("audience"))

		require.Equal(t, "Bearer "+requestToken, r.Header.Get("Authorization"))

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
}
